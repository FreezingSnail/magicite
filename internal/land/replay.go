package land

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/FreezingSnail/magicite/internal/stamp"
)

type replayCommit struct {
	rev     string
	message string
	author  string
	date    string
}

// stampRange rewrites commits unique to the seat branch with one canonical stamp block.
func (p *Pipeline) stampRange(ctx context.Context, c *Context, s stamp.Stamp) error {
	exit, output, err := p.git(ctx, c, c.Worktree, "rev-list", "--reverse", c.Integration+".."+c.Branch)
	if err != nil || exit != 0 {
		return p.stampError(replayFailure("rev-list", exit, output, err))
	}
	commits := strings.Fields(output)
	if len(commits) == 0 {
		return nil
	}

	exit, output, err = p.git(ctx, c, c.Worktree, "rev-parse", c.Branch)
	if err != nil || exit != 0 {
		return p.stampError(replayFailure("rev-parse", exit, output, err))
	}
	tip := strings.TrimSpace(output)
	if tip == "" {
		return p.stampError(fmt.Errorf("rev-parse returned an empty branch tip"))
	}

	if len(commits) == 1 {
		if err := p.stampHead(ctx, c, s); err != nil {
			p.restoreStamp(ctx, c, tip, false, false)
			return p.stampError(err)
		}
		return nil
	}

	replay, changed, err := p.replayCommits(ctx, c, commits, s)
	if err != nil {
		return p.stampError(err)
	}
	if !changed {
		return nil
	}

	exit, output, err = p.git(ctx, c, c.Worktree, "checkout", "--detach", c.Integration)
	if err != nil || exit != 0 {
		p.restoreStamp(ctx, c, tip, false, false)
		return p.stampError(replayFailure("checkout detach", exit, output, err))
	}

	pickInProgress := false
	for _, commit := range replay {
		pickInProgress = true
		exit, output, err = p.git(ctx, c, c.Worktree, "cherry-pick", "--no-commit", commit.rev)
		if err != nil || exit != 0 {
			failure := replayFailure("cherry-pick", exit, output, err)
			if strings.Contains(strings.ToLower(output), "conflict") {
				failure = fmt.Errorf("%w: %w", ErrConflict, failure)
			}
			p.restoreStamp(ctx, c, tip, pickInProgress, false)
			return p.stampError(failure)
		}
		pickInProgress = false

		path, cleanup, err := messageFile(stamp.Apply(commit.message, s.Trailers()))
		if err != nil {
			p.restoreStamp(ctx, c, tip, false, false)
			return p.stampError(err)
		}
		exit, output, err = p.git(ctx, c, c.Worktree, "commit", "-F", path, "--author="+commit.author, "--date="+commit.date)
		cleanup()
		if err != nil || exit != 0 {
			p.restoreStamp(ctx, c, tip, false, false)
			return p.stampError(replayFailure("commit", exit, output, err))
		}
	}

	exit, output, err = p.git(ctx, c, c.Worktree, "branch", "-f", c.Branch, "HEAD")
	if err != nil || exit != 0 {
		p.restoreStamp(ctx, c, tip, false, false)
		return p.stampError(replayFailure("branch move", exit, output, err))
	}
	exit, output, err = p.git(ctx, c, c.Worktree, "checkout", c.Branch)
	if err != nil || exit != 0 {
		p.restoreStamp(ctx, c, tip, false, true)
		return p.stampError(replayFailure("checkout branch", exit, output, err))
	}
	return nil
}

// commitMessage returns rev's complete commit message.
func (p *Pipeline) commitMessage(ctx context.Context, c *Context, rev string) (string, error) {
	exit, output, err := p.git(ctx, c, c.Worktree, "log", "-1", "--format=%B", rev)
	if err != nil || exit != 0 {
		return "", replayFailure("commit message", exit, output, err)
	}
	return output, nil
}

// stampHead amends the current branch tip when its canonical stamp differs.
func (p *Pipeline) stampHead(ctx context.Context, c *Context, s stamp.Stamp) error {
	message, err := p.commitMessage(ctx, c, c.Branch)
	if err != nil {
		return err
	}
	stamped := stamp.Apply(message, s.Trailers())
	if stamped == message {
		return nil
	}
	path, cleanup, err := messageFile(stamped)
	if err != nil {
		return err
	}
	defer cleanup()

	exit, output, err := p.git(ctx, c, c.Worktree, "commit", "--amend", "-F", path)
	if err != nil || exit != 0 {
		return replayFailure("amend commit", exit, output, err)
	}
	return nil
}

func (p *Pipeline) replayCommits(ctx context.Context, c *Context, revs []string, s stamp.Stamp) ([]replayCommit, bool, error) {
	commits := make([]replayCommit, 0, len(revs))
	changed := false
	for _, rev := range revs {
		message, err := p.commitMessage(ctx, c, rev)
		if err != nil {
			return nil, false, err
		}
		exit, output, err := p.git(ctx, c, c.Worktree, "log", "-1", "--format=%an%x00%ae%x00%aI", rev)
		if err != nil || exit != 0 {
			return nil, false, replayFailure("commit identity", exit, output, err)
		}
		parts := strings.Split(output, "\x00")
		if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
			return nil, false, fmt.Errorf("commit identity for %s is malformed", rev)
		}
		commits = append(commits, replayCommit{
			rev:     rev,
			message: message,
			author:  strings.TrimSpace(parts[0]) + " " + strings.TrimSpace(parts[1]),
			date:    strings.TrimSpace(parts[2]),
		})
		changed = changed || stamp.Apply(message, s.Trailers()) != message
	}
	return commits, changed, nil
}

func (p *Pipeline) restoreStamp(ctx context.Context, c *Context, tip string, pickInProgress, branchMoved bool) {
	if pickInProgress {
		p.status(ctx, c, c.Worktree, "cherry-pick", "--abort")
	}
	if branchMoved {
		p.status(ctx, c, c.Worktree, "branch", "-f", c.Branch, tip)
	}
	p.status(ctx, c, c.Worktree, "checkout", c.Branch)
	if !branchMoved {
		p.status(ctx, c, c.Worktree, "branch", "-f", c.Branch, tip)
	}
}

func (p *Pipeline) stampError(err error) error {
	p.warnf("%v", err)
	return err
}

func replayFailure(operation string, exit int, output string, err error) error {
	failure := fmt.Errorf("%s failed (exit %d, output %q)", operation, exit, output)
	if err != nil {
		return fmt.Errorf("%w: %w", failure, err)
	}
	return failure
}

func messageFile(message string) (string, func(), error) {
	file, err := os.CreateTemp("", "magicite-land-")
	if err != nil {
		return "", nil, fmt.Errorf("create commit message file: %w", err)
	}
	path := file.Name()
	cleanup := func() { _ = os.Remove(path) }
	if _, err := file.WriteString(message); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("write commit message file: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close commit message file: %w", err)
	}
	return path, cleanup, nil
}

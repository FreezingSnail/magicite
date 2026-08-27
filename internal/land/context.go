package land

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Context is the resolved repository and seat state for one landing operation.
type Context struct {
	Repo        Repo
	Root        string
	Worktree    string
	Branch      string
	Integration string
}

func (p *Pipeline) resolve(_ context.Context, repo Repo, seat string, needWorktree bool) (*Context, error) {
	if repo == nil || strings.TrimSpace(repo.Name()) == "" || strings.TrimSpace(repo.Integration()) == "" {
		return nil, ErrUnresolvedRepo
	}
	info, err := os.Stat(repo.Root())
	if err != nil || !info.IsDir() {
		return nil, ErrUnresolvedRepo
	}
	if strings.TrimSpace(seat) == "" || seat == "." || seat == ".." {
		return nil, ErrInvalidSeat
	}

	branch, err := p.workspace.Branch(repo, seat)
	if err != nil {
		return nil, err
	}
	worktree, err := p.workspace.Path(repo, seat)
	if err != nil {
		return nil, err
	}
	if needWorktree {
		info, err := os.Stat(worktree)
		if worktree == "" || err != nil || !info.IsDir() {
			return nil, ErrMissingWorktree
		}
	}

	return &Context{
		Repo:        repo,
		Root:        repo.Root(),
		Worktree:    worktree,
		Branch:      branch,
		Integration: repo.Integration(),
	}, nil
}

func (p *Pipeline) git(ctx context.Context, c *Context, dir string, args ...string) (int, string, error) {
	gitArgs := args
	root := ""
	if c != nil {
		root = c.Root
	}
	if dir != root {
		gitArgs = make([]string, 0, len(args)+2)
		gitArgs = append(gitArgs, "-C", dir)
		gitArgs = append(gitArgs, args...)
	}
	return p.runner.Git(ctx, root, gitArgs...)
}

func (p *Pipeline) status(ctx context.Context, c *Context, dir string, args ...string) int {
	code, _, err := p.git(ctx, c, dir, args...)
	if err != nil {
		return 1
	}
	return code
}

func (p *Pipeline) warnf(format string, args ...any) {
	p.log("warn", fmt.Sprintf(format, args...))
}

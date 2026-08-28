package land

import (
	"context"
	"fmt"
	"strings"

	"github.com/connorfranc/magicite/internal/stamp"
)

// Landed reports whether a seat branch is an ancestor of the integration branch.
func (p *Pipeline) Landed(ctx context.Context, repo Repo, seat string) (bool, error) {
	c, err := p.resolve(ctx, repo, seat, false)
	if err != nil {
		return false, err
	}

	exit, output, err := p.git(ctx, c, c.Root, "merge-base", "--is-ancestor", c.Branch, c.Integration)
	if err == nil && exit == 0 {
		return true, nil
	}
	if err == nil && exit == 1 {
		return false, nil
	}
	return false, queryError("branch ancestry", exit, output, err)
}

// TaskLanded reports whether the integration branch contains an exact task stamp.
func (p *Pipeline) TaskLanded(ctx context.Context, repo Repo, task string) (bool, error) {
	if repo == nil {
		return false, ErrUnresolvedRepo
	}
	if strings.TrimSpace(task) == "" {
		return false, fmt.Errorf("empty task: %w", ErrTaskUnstamped)
	}

	c := &Context{Repo: repo, Root: repo.Root(), Integration: repo.Integration()}
	exit, output, err := p.git(ctx, c, c.Root, "log", c.Integration, fmt.Sprintf("--format=%%(trailers:key=%s,valueonly)", stamp.KeyTask))
	if err != nil || exit != 0 {
		return false, queryError("task provenance", exit, output, err)
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == task {
			return true, nil
		}
	}
	return false, nil
}

// AssertTaskLanded refuses closure unless the integration branch contains the task stamp.
func (p *Pipeline) AssertTaskLanded(ctx context.Context, repo Repo, task string) error {
	landed, err := p.TaskLanded(ctx, repo, task)
	if err != nil {
		return err
	}
	if landed {
		return nil
	}
	name := "<nil>"
	if repo != nil {
		name = repo.Name()
	}
	return fmt.Errorf("%w: task %q not landed in repository %q", ErrTaskUnstamped, task, name)
}

func queryError(query string, exit int, output string, err error) error {
	failure := fmt.Errorf("%s failed (exit %d, output %q)", query, exit, output)
	if err != nil {
		return fmt.Errorf("%w: %w", failure, err)
	}
	return failure
}

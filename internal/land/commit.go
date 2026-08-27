package land

import (
	"context"
	"fmt"
	"strings"
)

func (p *Pipeline) commit(ctx context.Context, c *Context, seat string) error {
	exit, output, err := p.git(ctx, c, c.Worktree, "add", "-A")
	if err != nil {
		return fmt.Errorf("stage worktree: %w", err)
	}
	if exit != 0 {
		return queryError("stage worktree", exit, output, nil)
	}

	exit, output, err = p.git(ctx, c, c.Root, "merge-base", "--is-ancestor", c.Branch, c.Integration)
	if err != nil {
		return fmt.Errorf("check branch ancestry: %w", err)
	}
	if exit != 0 && exit != 1 {
		return queryError("check branch ancestry", exit, output, nil)
	}

	args := []string{"commit", "--amend", "--no-edit"}
	if exit == 0 {
		args = []string{"commit", "-m", fmt.Sprintf("task complete (%s)", seat)}
	}
	exit, output, err = p.git(ctx, c, c.Worktree, args...)
	if err != nil {
		return fmt.Errorf("record worktree commit: %w", err)
	}
	if exit == 0 || strings.Contains(strings.ToLower(output), "nothing to commit") {
		return nil
	}

	failure := queryError("record worktree commit", exit, output, nil)
	p.warnf("worktree commit failed for branch %q: %v", c.Branch, failure)
	return failure
}

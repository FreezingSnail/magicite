package land

import (
	"context"
	"strconv"
	"strings"
)

type rebaseResult int

const (
	rebaseOK rebaseResult = iota
	rebaseConflict
	rebaseFailed
)

func (p *Pipeline) rebase(ctx context.Context, c *Context) (rebaseResult, error) {
	exit, output, err := p.git(ctx, c, c.Worktree, "rebase", c.Integration, c.Branch)
	if err == nil && exit == 0 {
		return rebaseOK, nil
	}

	abortStatus := p.status(ctx, c, c.Worktree, "rebase", "--abort")
	if abortStatus != 0 {
		p.warnf("rebase abort failed with status %d", abortStatus)
	}

	if exit != 0 && strings.Contains(strings.ToLower(output), "conflict") {
		return rebaseConflict, nil
	}

	p.warnf("rebase failed (exit %d, output %q)", exit, output)
	if err != nil {
		return rebaseFailed, queryError("rebase", exit, output, err)
	}
	return rebaseFailed, nil
}

func (p *Pipeline) linear(ctx context.Context, c *Context) (bool, error) {
	exit, output, err := p.git(ctx, c, c.Worktree, "rev-list", "--count", "--merges", c.Integration+".."+c.Branch)
	if err != nil || exit != 0 {
		return false, queryError("linear history", exit, output, err)
	}

	trimmed := strings.TrimSpace(output)
	if _, err := strconv.Atoi(trimmed); err != nil {
		return false, ErrNotLinear
	}
	return trimmed == "0", nil
}

package land

import (
	"context"
	"errors"
	"fmt"

	magicexec "github.com/connorfranc/magicite/internal/exec"
)

func (p *Pipeline) gate(ctx context.Context, c *Context) error {
	var status int
	var runErr error
	if p.gateFunc != nil {
		status, runErr = p.gateFunc(ctx, c)
	} else {
		status, runErr = p.runGate(ctx, c)
	}
	if runErr == nil && status == 0 {
		return nil
	}

	p.warnf("verification gate failed for branch %q with status %d", c.Branch, status)
	if runErr != nil {
		return fmt.Errorf("%w: gate status %d: %w", ErrGateFailed, status, runErr)
	}
	return fmt.Errorf("%w: gate status %d", ErrGateFailed, status)
}

func (p *Pipeline) runGate(ctx context.Context, c *Context) (int, error) {
	if len(p.gateArgv) == 0 {
		return -1, errors.New("empty verification gate")
	}
	if c == nil {
		return -1, errors.New("nil landing context")
	}

	_, _, status, runErr := magicexec.Run(ctx, c.Worktree, p.gateArgv[0], p.gateArgv[1:]...)
	if runErr != nil && status < 0 {
		return status, runErr
	}
	return status, nil
}

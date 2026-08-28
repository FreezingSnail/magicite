package land

import (
	"context"
	"fmt"
	"strings"

	"github.com/connorfranc/magicite/internal/stamp"
)

// fastForward advances integration to the seat branch without creating a merge commit.
func (p *Pipeline) fastForward(ctx context.Context, c *Context) (int, string, error) {
	return p.git(ctx, c, c.Root, "merge", "--ff-only", c.Branch)
}

func divergedFF(output string) bool {
	output = strings.ToLower(output)
	return strings.Contains(output, "not possible to fast-forward") || strings.Contains(output, "diverging branches")
}

// landRebased gates and fast-forwards a rebased seat, retrying one concurrent divergence.
func (p *Pipeline) landRebased(ctx context.Context, c *Context, s *stamp.Stamp) error {
	if err := p.gate(ctx, c); err != nil {
		return err
	}

	exit, output, err := p.fastForward(ctx, c)
	if exit == 0 {
		return nil
	}
	if !divergedFF(output) {
		return p.fastForwardFailure(c, exit, output, err)
	}

	result, rebaseErr := p.rebase(ctx, c)
	if result == rebaseConflict {
		return p.conflictFailure(c, "rebase conflict")
	}
	if result != rebaseOK {
		if rebaseErr != nil {
			return p.refuse(c, fmt.Errorf("rebase retry failed: %w", rebaseErr))
		}
		return p.refuse(c, fmt.Errorf("rebase retry failed"))
	}
	if s != nil {
		if err := p.stampRange(ctx, c, *s); err != nil {
			return p.refuse(c, err)
		}
	}
	if err := p.gate(ctx, c); err != nil {
		return err
	}

	exit, output, err = p.fastForward(ctx, c)
	if exit == 0 {
		return nil
	}
	if divergedFF(output) {
		return p.conflictFailure(c, fmt.Sprintf("fast-forward retry diverged (exit %d, output %q)", exit, output))
	}
	return p.fastForwardFailure(c, exit, output, err)
}

func (p *Pipeline) fastForwardFailure(c *Context, exit int, output string, err error) error {
	return p.refuse(c, queryError("fast-forward", exit, output, err))
}

func (p *Pipeline) conflictFailure(c *Context, detail string) error {
	return p.refuse(c, fmt.Errorf("%w: %s", ErrConflict, detail))
}

func (p *Pipeline) refuse(c *Context, err error) error {
	p.warnf("landing branch %q refused: %v", c.Branch, err)
	return err
}

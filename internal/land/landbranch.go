package land

import (
	"context"
	"errors"
	"fmt"

	"github.com/FreezingSnail/magicite/internal/stamp"
)

// Outcome describes the result of a branch landing attempt.
type Outcome int

const (
	OutcomeFailed Outcome = iota
	OutcomeLanded
	OutcomeConflict
	OutcomeGateFailed
)

// String returns the stable name of an outcome.
func (o Outcome) String() string {
	switch o {
	case OutcomeFailed:
		return "failed"
	case OutcomeLanded:
		return "landed"
	case OutcomeConflict:
		return "conflict"
	case OutcomeGateFailed:
		return "gate failed"
	default:
		return "unknown"
	}
}

// LandBranch commits, rebases, verifies, stamps, gates, and fast-forwards a seat branch.
func (p *Pipeline) LandBranch(ctx context.Context, repo Repo, seat string, s *stamp.Stamp) (Outcome, error) {
	c, err := p.resolve(ctx, repo, seat, true)
	if err != nil {
		return OutcomeFailed, err
	}

	var landingStamp *stamp.Stamp
	if s != nil {
		copied := *s
		copied.Repo = repo.Name()
		landingStamp = &copied
	}

	if err := p.commit(ctx, c, seat); err != nil {
		return outcome(err)
	}
	if err := p.verifyBranch(ctx, c); err != nil {
		return outcome(err)
	}

	result, err := p.rebase(ctx, c)
	switch result {
	case rebaseConflict:
		return outcome(fmt.Errorf("%w: rebase conflict", ErrConflict))
	case rebaseFailed:
		if err == nil {
			err = errors.New("rebase failed")
		}
		return outcome(err)
	case rebaseOK:
	default:
		return outcome(fmt.Errorf("unknown rebase result %d", result))
	}

	linear, err := p.linear(ctx, c)
	if err != nil {
		return outcome(err)
	}
	if !linear {
		return outcome(fmt.Errorf("%w: %w", ErrConflict, ErrNotLinear))
	}

	if landingStamp != nil {
		if err := p.stampRange(ctx, c, *landingStamp); err != nil {
			return outcome(err)
		}
	}
	return outcome(p.landRebased(ctx, c, landingStamp))
}

func (p *Pipeline) verifyBranch(ctx context.Context, c *Context) error {
	exit, output, err := p.git(ctx, c, c.Root, "rev-parse", "--verify", c.Branch)
	if err == nil && exit == 0 {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrBranchMissing, queryError("verify branch", exit, output, err))
}

func outcome(err error) (Outcome, error) {
	switch {
	case err == nil:
		return OutcomeLanded, nil
	case errors.Is(err, ErrConflict):
		return OutcomeConflict, err
	case errors.Is(err, ErrGateFailed):
		return OutcomeGateFailed, err
	default:
		return OutcomeFailed, err
	}
}

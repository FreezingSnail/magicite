package dispatch

import (
	"context"
	"errors"

	"github.com/FreezingSnail/magicite/internal/decomp"
	"github.com/FreezingSnail/magicite/internal/repo"
)

// ErrReviewUnsupported reports review work requested without a configured gate.
var ErrReviewUnsupported = errors.New("dispatch: review gate unsupported")

// PermissiveGate preserves dispatch behavior when review is disabled.
type PermissiveGate struct{}

var _ Gate = PermissiveGate{}

func (PermissiveGate) Hold(context.Context, repo.Repo) (bool, error) { return false, nil }

func (PermissiveGate) DueEpic(context.Context, repo.Repo, string) (string, error) { return "", nil }

func (PermissiveGate) GateEpic(context.Context, repo.Repo, string) (string, error) { return "", nil }

func (PermissiveGate) ReviewPlan(context.Context, repo.Repo, string) (RunSpec, error) {
	return RunSpec{}, ErrReviewUnsupported
}

func (PermissiveGate) NoteSession(string, repo.Repo, string) {}

func (PermissiveGate) CompleteReview(context.Context, string, string) error { return nil }

func (PermissiveGate) AbortReview(context.Context, string, string) error { return nil }

func (PermissiveGate) DecompositionVerdict(context.Context, repo.Repo, string) ([]decomp.Violation, error) {
	return nil, nil
}

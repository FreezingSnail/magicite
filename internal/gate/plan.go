package gate

import (
	"context"
	"fmt"
	"strings"

	"github.com/connorfranc/magicite/internal/dispatch"
	"github.com/connorfranc/magicite/internal/repo"
)

const (
	MaxPlanDiffBytes = 262144
	TruncationNotice = "[diff truncated at 262144 bytes]"
)

type Plan struct {
	Text, Model, Agent string
}

// BuildPlan renders the deterministic reviewer instruction and its inputs.
func BuildPlan(diff, goal string) string {
	if len(diff) > MaxPlanDiffBytes {
		diff = diff[:MaxPlanDiffBytes] + TruncationNotice
	}
	return strings.Join([]string{
		"Review the epic diff against the stated goal.",
		"Emit exactly one verdict line and nothing else. The only legal outputs are:",
		MarkerApproved,
		MarkerDrift + " followed by feedback",
		"Diff:",
		"```",
		diff,
		"```",
		"Goal:",
		"```",
		goal,
		"```",
	}, "\n")
}

// EpicGoal reads the review goal from the epic's typed bead fields.
func (g *Gate) EpicGoal(ctx context.Context, r repo.Repo, epic string) (string, error) {
	bead, err := g.beads.Show(ctx, r, epic)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(bead.Design + "\n\n" + bead.AcceptanceCriteria), nil
}

// ReviewPlan builds the reviewer plan and carries its configured session settings.
func (g *Gate) ReviewPlan(ctx context.Context, r repo.Repo, epic string) (dispatch.RunSpec, error) {
	if _, ok := g.key(r, epic); !ok {
		return dispatch.RunSpec{}, fmt.Errorf("gate: invalid review target")
	}
	if !g.config.Enabled {
		return dispatch.RunSpec{}, fmt.Errorf("gate: review disabled")
	}
	diff, err := g.EpicDiff(ctx, r, epic)
	if err != nil {
		return dispatch.RunSpec{}, err
	}
	goal, err := g.EpicGoal(ctx, r, epic)
	if err != nil {
		return dispatch.RunSpec{}, err
	}
	return dispatch.RunSpec{
		Plan:  BuildPlan(diff, goal),
		Model: g.config.Model,
		Agent: g.config.Agent,
	}, nil
}

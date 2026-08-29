package gate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/dispatch"
	"github.com/FreezingSnail/magicite/internal/repo"
)

func TestBuildPlanRendersVerdictsAndGoal(t *testing.T) {
	plan := BuildPlan("changed", "typed goal")
	for _, want := range []string{MarkerApproved, MarkerDrift, "changed", "typed goal", "```"} {
		if !strings.Contains(plan, want) {
			t.Fatalf("BuildPlan() = %q, missing %q", plan, want)
		}
	}
	if !strings.Contains(plan, "exactly one verdict line") || !strings.Contains(plan, "nothing else") {
		t.Fatalf("BuildPlan() omitted single-verdict instruction: %q", plan)
	}
}

func TestBuildPlanTruncatesDiffWithoutTruncatingGoal(t *testing.T) {
	diff := strings.Repeat("d", MaxPlanDiffBytes+1)
	goal := "goal after oversized diff"
	plan := BuildPlan(diff, goal)
	if !strings.Contains(plan, TruncationNotice) {
		t.Fatalf("BuildPlan() missing truncation notice")
	}
	if !strings.Contains(plan, goal) {
		t.Fatalf("BuildPlan() lost goal: %q", plan)
	}
	if strings.Contains(plan, strings.Repeat("d", MaxPlanDiffBytes+1)) {
		t.Fatalf("BuildPlan() retained oversized diff")
	}
}

func TestEpicGoalUsesTypedBeadFields(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{
		show: func(context.Context, repo.Repo, string) (bd.Bead, error) {
			return bd.Bead{
				Design:             "design field",
				AcceptanceCriteria: "acceptance field",
			}, nil
		},
	}
	g := dueGate(t, beads, Config{Enabled: true})
	goal, err := g.EpicGoal(context.Background(), r, "epic")
	if err != nil || goal != "design field\n\nacceptance field" {
		t.Fatalf("EpicGoal() = (%q, %v)", goal, err)
	}
}

func TestEpicGoalEmptyAndShowError(t *testing.T) {
	r := testRepo(t, "repo")
	empty := dueGate(t, &fakeBeads{beads: map[string]bd.Bead{"epic": {}}}, Config{Enabled: true})
	if goal, err := empty.EpicGoal(context.Background(), r, "epic"); goal != "" || err != nil {
		t.Fatalf("empty EpicGoal() = (%q, %v)", goal, err)
	}
	wantErr := errors.New("show failed")
	failed := dueGate(t, &fakeBeads{show: func(context.Context, repo.Repo, string) (bd.Bead, error) {
		return bd.Bead{}, wantErr
	}}, Config{Enabled: true})
	if _, err := failed.EpicGoal(context.Background(), r, "epic"); !errors.Is(err, wantErr) {
		t.Fatalf("failed EpicGoal() error = %v, want %v", err, wantErr)
	}
}

func TestReviewPlanCarriesConfigAndPropagatesDiffError(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{beads: map[string]bd.Bead{"epic": {Design: "goal"}}}
	g := dueGate(t, beads, Config{Enabled: true})
	g.config.Model, g.config.Agent = "model", "agent"
	plan, err := g.ReviewPlan(context.Background(), r, "epic")
	if err != nil || plan.Model != "model" || plan.Agent != "agent" || !strings.Contains(plan.Plan, "goal") {
		t.Fatalf("ReviewPlan() = (%+v, %v)", plan, err)
	}

	wantErr := errors.New("diff failed")
	failed := dueGate(t, beads, Config{Enabled: true})
	failed.git = &fakeGit{output: func(context.Context, repo.Repo, ...string) (int, string, error) {
		return 1, "", wantErr
	}}
	plan, err = failed.ReviewPlan(context.Background(), r, "epic")
	if !errors.Is(err, wantErr) || plan != (dispatch.RunSpec{}) {
		t.Fatalf("failed ReviewPlan() = (%+v, %v)", plan, err)
	}
}

func TestReviewPlanRefusesDisabledGate(t *testing.T) {
	r := testRepo(t, "repo")
	g := dueGate(t, &fakeBeads{}, Config{})
	plan, err := g.ReviewPlan(context.Background(), r, "epic")
	if err == nil || plan != (dispatch.RunSpec{}) {
		t.Fatalf("disabled ReviewPlan() = (%+v, %v), want error and empty plan", plan, err)
	}
}

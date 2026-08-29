package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/repo"
)

func planRepo(t *testing.T) repo.Repo {
	t.Helper()
	record, ok := repo.Make("/repo", "magicite", "magicite", "main")
	if !ok {
		t.Fatal("repo.Make failed")
	}
	return record
}

func TestSpecSectionsOmitsEmptyFieldsInOrder(t *testing.T) {
	got := SpecSections(Spec{
		Title:       "  title  ",
		Description: " \t",
		Design:      "design",
		Acceptance:  "acceptance",
	})
	want := "Title:\ntitle\n\nDesign:\ndesign\n\nAcceptance:\nacceptance"
	if got != want {
		t.Fatalf("SpecSections() = %q, want %q", got, want)
	}
}

func TestPlanForRolePlans(t *testing.T) {
	r := planRepo(t)
	spec := Spec{Title: "Title", Description: "Description", Design: "Design", Acceptance: "Acceptance"}
	beads := &fakeBeads{show: func(context.Context, repo.Repo, string) (Spec, error) { return spec, nil }}
	dispatcher := &Dispatcher{beads: beads, gate: &fakeGate{}}

	implementer, err := dispatcher.PlanFor(context.Background(), r, Implementer, "task-1", "ifrit")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(implementer, "Implement the task") || !strings.Contains(implementer, "Commit the change to the ifrit seat branch") {
		t.Fatalf("implementer plan missing instructions: %q", implementer)
	}
	if !strings.HasSuffix(implementer, BDInputRules) {
		t.Fatal("implementer plan does not end with BDInputRules")
	}

	designer, err := dispatcher.PlanFor(context.Background(), r, Designer, "task-1", "ramuh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(designer, "Produce the design") || !strings.Contains(designer, "Record the design and acceptance criteria") {
		t.Fatalf("designer plan missing instructions: %q", designer)
	}
	if !strings.HasSuffix(designer, BDInputRules) {
		t.Fatal("designer plan does not end with BDInputRules")
	}

	repairer, err := dispatcher.PlanFor(context.Background(), r, Repairer, "task-1", "ifrit")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"magicite", "ifrit", "task-1", "main", "git add -A", "git rebase --continue", "Never merge"} {
		if !strings.Contains(repairer, want) {
			t.Errorf("repairer plan missing %q: %q", want, repairer)
		}
	}
}

func TestPlanForDegradesShowFailure(t *testing.T) {
	r := planRepo(t)
	showErr := errors.New("bd unavailable")
	dispatcher := &Dispatcher{
		beads: &fakeBeads{show: func(context.Context, repo.Repo, string) (Spec, error) { return Spec{}, showErr }},
		gate:  &fakeGate{},
	}
	plan, err := dispatcher.PlanFor(context.Background(), r, Implementer, "task-1", "ifrit")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan, "task-1") || !strings.Contains(plan, "no readable specification") {
		t.Fatalf("degraded plan = %q", plan)
	}
}

func TestPlanForDelegatesReviewer(t *testing.T) {
	r := planRepo(t)
	gate := &fakeGate{reviewPlan: func(context.Context, repo.Repo, string) (RunSpec, error) {
		return RunSpec{Plan: "review plan"}, nil
	}}
	dispatcher := &Dispatcher{beads: &fakeBeads{}, gate: gate}
	plan, err := dispatcher.PlanFor(context.Background(), r, Reviewer, "epic-1", "ifrit")
	if err != nil || plan != "review plan" {
		t.Fatalf("PlanFor() = (%q, %v), want review plan", plan, err)
	}
	calls := gate.Calls()
	if len(calls) != 1 || calls[0].Method != "ReviewPlan" {
		t.Fatalf("gate calls = %#v, want ReviewPlan", calls)
	}
}

func TestPlanForReturnsTypedErrorForUnknownRole(t *testing.T) {
	dispatcher := &Dispatcher{beads: &fakeBeads{}, gate: &fakeGate{}}
	_, err := dispatcher.PlanFor(context.Background(), planRepo(t), Role("unknown"), "task-1", "ifrit")
	var planErr *PlanError
	if !errors.As(err, &planErr) || !errors.Is(err, ErrUnknownRole) {
		t.Fatalf("PlanFor() error = %v, want typed unknown-role error", err)
	}
}

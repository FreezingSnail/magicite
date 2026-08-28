package gate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/bd"
	"github.com/connorfranc/magicite/internal/repo"
)

func dueGate(t *testing.T, beads *fakeBeads, config Config) *Gate {
	t.Helper()
	if config.Enabled {
		config.Model = "model"
		config.Agent = "agent"
	}
	g, err := New(Deps{
		Beads: beads,
		Git: &fakeGit{output: func(context.Context, repo.Repo, ...string) (int, string, error) {
			return 0, "base\n", nil
		}},
		Repos:  &fakeRepos{},
		Config: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestHoldWithInFlightDriftRefusalAndClear(t *testing.T) {
	r := testRepo(t, "repo")
	g := dueGate(t, &fakeBeads{}, Config{Enabled: true})
	k, ok := g.key(r, "epic")
	if !ok {
		t.Fatal("key rejected")
	}
	g.track("review", k)
	if !g.HoldWith(r, nil) {
		t.Fatal("HoldWith() = false with review in flight")
	}
	g.drop("review")
	if !g.HoldWith(r, []string{"fix"}) {
		t.Fatal("HoldWith() = false with drift fix")
	}
	if !g.HoldWith(repo.Repo{}, nil) {
		t.Fatal("HoldWith() = false with refused repository")
	}
	if g.HoldWith(r, nil) {
		t.Fatal("HoldWith() = true after review and drift fix cleared")
	}
}

func TestHoldQueriesDriftFixesAndFailsSafe(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{queries: map[string][]bd.Bead{bd.DriftFixQuery(): {{ID: "fix"}}}}
	g := dueGate(t, beads, Config{Enabled: true})
	if hold, err := g.Hold(context.Background(), r); err != nil || !hold {
		t.Fatalf("Hold() = (%t, %v), want (true, nil)", hold, err)
	}
	failed := dueGate(t, &fakeBeads{query: func(context.Context, repo.Repo, string) ([]bd.Bead, error) {
		return nil, errors.New("query failed")
	}}, Config{Enabled: true})
	if hold, err := failed.Hold(context.Background(), r); !hold || err == nil {
		t.Fatalf("failed Hold() = (%t, %v), want (true, error)", hold, err)
	}
}

func TestDisabledGateHoldsNothingOrQueriesNothing(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{query: func(context.Context, repo.Repo, string) ([]bd.Bead, error) {
		t.Fatal("disabled DriftFixTasks queried beads")
		return nil, nil
	}}
	g := dueGate(t, beads, Config{})
	if fixes, err := g.DriftFixTasks(context.Background(), r); err != nil || len(fixes) != 0 {
		t.Fatalf("DriftFixTasks() = (%q, %v), want empty nil", fixes, err)
	}
	if hold, err := g.Hold(context.Background(), repo.Repo{}); err != nil || hold {
		t.Fatalf("Hold() = (%t, %v), want (false, nil)", hold, err)
	}
}

func TestDueEpicAndGateEpicShareCompletedPredicate(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{
		beads:    map[string]bd.Bead{"task": {ID: "task", Parent: "epic"}},
		children: map[string][]bd.Bead{"epic": {{ID: "task", Status: "closed"}}},
	}
	event := dueGate(t, beads, Config{Enabled: true})
	if epic, err := event.DueEpic(context.Background(), r, "task"); err != nil || epic != "epic" {
		t.Fatalf("DueEpic() = (%q, %v), want (epic, nil)", epic, err)
	}
	sweep := dueGate(t, beads, Config{Enabled: true})
	if epic, err := sweep.GateEpic(context.Background(), r, "epic"); err != nil || epic != "epic" {
		t.Fatalf("GateEpic() = (%q, %v), want (epic, nil)", epic, err)
	}
}

func TestIncompleteOrUndecomposedEpicIsNotDue(t *testing.T) {
	r := testRepo(t, "repo")
	for _, test := range []struct {
		name     string
		children []bd.Bead
	}{
		{"open child", []bd.Bead{{ID: "closed", Status: "closed"}, {ID: "open", Status: "open"}}},
		{"no children", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			beads := &fakeBeads{
				beads:    map[string]bd.Bead{"task": {ID: "task", Parent: "epic"}},
				children: map[string][]bd.Bead{"epic": test.children},
			}
			g := dueGate(t, beads, Config{Enabled: true})
			if epic, err := g.DueEpic(context.Background(), r, "task"); err != nil || epic != "" {
				t.Fatalf("DueEpic() = (%q, %v), want empty nil", epic, err)
			}
			if epic, err := g.GateEpic(context.Background(), r, "epic"); err != nil || epic != "" {
				t.Fatalf("GateEpic() = (%q, %v), want empty nil", epic, err)
			}
		})
	}
}

func TestGateEpicExhaustsBudgetOnce(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{children: map[string][]bd.Bead{"epic": {{ID: "task", Status: "closed"}}}}
	g := dueGate(t, beads, Config{Enabled: true, MaxRetries: 2})
	k, _ := g.key(r, "epic")
	g.noteAttempt(k)
	g.noteAttempt(k)
	for range 2 {
		if epic, err := g.GateEpic(context.Background(), r, "epic"); err != nil || epic != "" {
			t.Fatalf("GateEpic() = (%q, %v), want empty nil", epic, err)
		}
	}
	calls := beads.Calls()
	if len(calls) != 3 || calls[1].method != "Comment" || !strings.Contains(calls[1].args[2].(string), "2 attempts") || !strings.Contains(calls[1].args[2].(string), "human attention") {
		t.Fatalf("calls = %#v, want one exhaustion comment", calls)
	}
}

func TestDisabledGateClosesCompletedEpic(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{children: map[string][]bd.Bead{"epic": {{ID: "task", Status: "closed"}}}}
	g := dueGate(t, beads, Config{})
	if epic, err := g.GateEpic(context.Background(), r, "epic"); err != nil || epic != "" {
		t.Fatalf("GateEpic() = (%q, %v), want empty nil", epic, err)
	}
	calls := beads.Calls()
	if len(calls) != 2 || calls[1].method != "Close" || calls[1].args[2] != disabledCloseReason {
		t.Fatalf("calls = %#v, want disabled close", calls)
	}
}

func TestDueEpicReturnsQueryErrorsUngated(t *testing.T) {
	r := testRepo(t, "repo")
	showFailed := dueGate(t, &fakeBeads{show: func(context.Context, repo.Repo, string) (bd.Bead, error) {
		return bd.Bead{}, errors.New("show failed")
	}}, Config{Enabled: true})
	if epic, err := showFailed.DueEpic(context.Background(), r, "task"); epic != "" || err == nil {
		t.Fatalf("DueEpic() = (%q, %v), want empty error", epic, err)
	}
	childrenFailed := dueGate(t, &fakeBeads{epicChildren: func(context.Context, repo.Repo, string) ([]bd.Bead, error) {
		return nil, errors.New("children failed")
	}}, Config{Enabled: true})
	if epic, err := childrenFailed.GateEpic(context.Background(), r, "epic"); epic != "" || err == nil {
		t.Fatalf("GateEpic() = (%q, %v), want empty error", epic, err)
	}
}

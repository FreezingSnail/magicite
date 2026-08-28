package gate

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/connorfranc/magicite/internal/bd"
	"github.com/connorfranc/magicite/internal/decomp"
	"github.com/connorfranc/magicite/internal/repo"
)

func TestDecompChildrenMapsCompleteBeadData(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{
		children: map[string][]bd.Bead{"epic": {{
			ID: "child", Description: "description", Design: "design", AcceptanceCriteria: "acceptance",
			Dependencies: []bd.Dependency{{ID: "first"}, {ID: "second"}}, Status: "closed",
		}}},
		labels: map[string][]string{"child": {"difficulty:low", "staged"}},
	}
	g := dueGate(t, beads, Config{Enabled: true})
	got, err := g.DecompChildren(context.Background(), r, "epic")
	want := []decomp.Child{{
		ID: "child", Description: "description", Design: "design", Acceptance: "acceptance",
		Labels: []string{"difficulty:low", "staged"}, Deps: []string{"first", "second"},
	}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("DecompChildren() = %#v, %v; want %#v, nil", got, err, want)
	}
}

func TestDecompChildrenRefusesPartialData(t *testing.T) {
	r := testRepo(t, "repo")
	want := errors.New("labels failed")
	beads := &fakeBeads{
		children: map[string][]bd.Bead{"epic": {{ID: "first"}, {ID: "second"}}},
		labelsFor: func(_ context.Context, _ repo.Repo, id string) ([]string, error) {
			if id == "second" {
				return nil, want
			}
			return []string{"staged"}, nil
		},
	}
	got, err := dueGate(t, beads, Config{Enabled: true}).DecompChildren(context.Background(), r, "epic")
	if !errors.Is(err, want) || got != nil {
		t.Fatalf("DecompChildren() = %#v, %v; want nil, %v", got, err, want)
	}

	childrenFailed := &fakeBeads{epicChildren: func(context.Context, repo.Repo, string) ([]bd.Bead, error) {
		return nil, want
	}}
	got, err = dueGate(t, childrenFailed, Config{Enabled: true}).DecompChildren(context.Background(), r, "epic")
	if !errors.Is(err, want) || got != nil {
		t.Fatalf("failed children DecompChildren() = %#v, %v", got, err)
	}
}

func TestDecompositionVerdictPassesOrFilesOneFix(t *testing.T) {
	r := testRepo(t, "repo")
	t.Run("pass", func(t *testing.T) {
		beads := &fakeBeads{children: map[string][]bd.Bead{"epic": nil}}
		violations, err := dueGate(t, beads, Config{Enabled: true}).DecompositionVerdict(context.Background(), r, "epic")
		if err != nil || violations != nil {
			t.Fatalf("DecompositionVerdict() = %#v, %v", violations, err)
		}
		calls := beads.Calls()
		if len(calls) != 2 || calls[0].method != "EpicChildren" || calls[1].method != "Comment" || calls[1].args[2] != decompPassComment {
			t.Fatalf("calls = %#v", calls)
		}
	})
	t.Run("violation", func(t *testing.T) {
		beads := &fakeBeads{nextID: "fix", children: map[string][]bd.Bead{"epic": {{ID: "child"}}}}
		violations, err := dueGate(t, beads, Config{Enabled: true}).DecompositionVerdict(context.Background(), r, "epic")
		if err != nil || len(violations) == 0 {
			t.Fatalf("DecompositionVerdict() = %#v, %v", violations, err)
		}
		calls := beads.Calls()
		if len(calls) != 4 || calls[2].method != "Create" || calls[3].method != "Comment" {
			t.Fatalf("calls = %#v", calls)
		}
		req := calls[2].args[1].(bd.CreateRequest)
		if req.Title != "decomp-fix: epic" || req.Type != "task" || req.Parent != "epic" || req.Priority != "P1" || !reflect.DeepEqual(req.Labels, []string{"decomp-fix"}) || req.Body != decomp.Format(violations) {
			t.Fatalf("Create() = %#v", req)
		}
		for _, call := range calls {
			if call.method == "Close" {
				t.Fatal("DecompositionVerdict() closed epic")
			}
		}
	})
}

func TestDecompositionVerdictSkipsDisabledAndRefusedTargets(t *testing.T) {
	r := testRepo(t, "repo")
	for _, test := range []struct {
		name string
		gate *Gate
		repo repo.Repo
		epic string
	}{
		{"disabled", dueGate(t, &fakeBeads{}, Config{}), r, "epic"},
		{"refused", dueGate(t, &fakeBeads{}, Config{Enabled: true}), repo.Repo{}, "epic"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.gate.DecompositionVerdict(context.Background(), test.repo, test.epic)
			if got != nil || err != nil {
				t.Fatalf("DecompositionVerdict() = %#v, %v", got, err)
			}
		})
	}
}

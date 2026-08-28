package gate

import (
	"context"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/bd"
	"github.com/connorfranc/magicite/internal/dispatch"
	"github.com/connorfranc/magicite/internal/repo"
)

var _ dispatch.Gate = (*Gate)(nil)

func TestGateConformsEndToEnd(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{
		beads: map[string]bd.Bead{"epic": {ID: "epic", Design: "goal"}},
		children: map[string][]bd.Bead{
			"epic": {{ID: "child", Status: "closed"}},
			"other": {{ID: "other-child", Status: "closed"}},
		},
		queries: map[string][]bd.Bead{},
		nextID:  "drift-fix",
	}
	git := &fakeGit{output: func(_ context.Context, _ repo.Repo, args ...string) (int, string, error) {
		switch args[0] {
		case "rev-parse":
			return 0, "base\n", nil
		case "log":
			return 0, "epic-commit\x00child\nother-commit\x00other-child\n", nil
		case "show":
			return 0, "[" + args[3] + "]", nil
		}
		t.Fatalf("unexpected git argv %q", args)
		return 0, "", nil
	}}
	g, err := New(Deps{
		Beads: beads, Git: git, Repos: &fakeRepos{records: map[string]repo.Repo{r.Name: r}},
		Config: Config{Enabled: true, Model: "model", Agent: "agent"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if epic, err := g.GateEpic(context.Background(), r, "epic"); err != nil || epic != "epic" {
		t.Fatalf("GateEpic() = (%q, %v)", epic, err)
	}
	plan, err := g.ReviewPlan(context.Background(), r, "epic")
	if err != nil || !strings.Contains(plan.Plan, "[epic-commit]") || strings.Contains(plan.Plan, "[other-commit]") {
		t.Fatalf("ReviewPlan() = (%#v, %v)", plan, err)
	}
	g.NoteSession("first", r, "epic")
	if err := g.CompleteReview(context.Background(), "first", strings.Join([]string{"review", MarkerDrift + " fix contract"}, "\n")); err != nil {
		t.Fatal(err)
	}
	beads.mu.Lock()
	beads.queries[bd.DriftFixQuery()] = []bd.Bead{{ID: "drift-fix"}}
	beads.mu.Unlock()
	if hold, err := g.Hold(context.Background(), r); err != nil || !hold {
		t.Fatalf("Hold() = (%t, %v)", hold, err)
	}
	if bead := beads.beads["epic"]; bead.Status == "closed" {
		t.Fatal("drift review closed epic")
	}

	g.NoteSession("second", r, "epic")
	if err := g.CompleteReview(context.Background(), "second", strings.Join([]string{"review", MarkerApproved}, "\n")); err != nil {
		t.Fatal(err)
	}
	beads.mu.Lock()
	beads.queries[bd.DriftFixQuery()] = nil
	beads.mu.Unlock()
	if bead := beads.beads["epic"]; bead.Status != "closed" {
		t.Fatalf("epic status = %q, want closed", bead.Status)
	}
	if hold, err := g.Hold(context.Background(), r); err != nil || hold {
		t.Fatalf("Hold() = (%t, %v)", hold, err)
	}

	creates := 0
	for _, call := range beads.Calls() {
		if call.method == "Create" {
			creates++
		}
	}
	if creates != 1 {
		t.Fatalf("drift fixes = %d, want 1", creates)
	}
}

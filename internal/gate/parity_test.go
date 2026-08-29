package gate

import (
	"context"
	"testing"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/parity"
	"github.com/FreezingSnail/magicite/internal/repo"
)

func TestMaduinReviewParity(t *testing.T) {
	bindings := parity.NewBindings(t, "TestMaduinReviewParity")
	bindings.Bind("maduin-test-review-complete-reads-opencode-transcript", func(t *testing.T) {
		if got := ParseVerdict("noise\nREVIEW: APPROVED\n"); got.Kind != VerdictApproved {
			t.Fatalf("ParseVerdict() = %#v", got)
		}
	})
	bindings.Bind("maduin-test-review-verdict-approved", func(t *testing.T) {
		if got := ParseVerdict(MarkerApproved); got != (Verdict{Kind: VerdictApproved}) {
			t.Fatalf("ParseVerdict() = %#v", got)
		}
	})
	bindings.Bind("maduin-test-review-verdict-drift", func(t *testing.T) {
		if got := ParseVerdict(MarkerDrift + ": revise\nignored"); got.Kind != VerdictDrift || got.Feedback != ": revise" {
			t.Fatalf("ParseVerdict() = %#v", got)
		}
	})
	bindings.Bind("maduin-test-review-verdict-garbage", func(t *testing.T) {
		if got := ParseVerdict("review approved"); got.Kind != VerdictUnparseable {
			t.Fatalf("ParseVerdict() = %#v", got)
		}
	})
	bindings.Bind("maduin-test-review-blocked-p", func(t *testing.T) {
		g, r, _ := parityReviewGate(t, true)
		if !g.HoldWith(r, []string{"fix"}) {
			t.Fatal("HoldWith() = false")
		}
	})
	bindings.Bind("maduin-test-review-error-exhaustion-closes-completed-epic", func(t *testing.T) {
		g, r, beads := parityReviewGate(t, true)
		k, _ := g.key(r, "epic")
		for range g.MaxRetries() {
			g.noteAttempt(k)
		}
		if epic, err := g.GateEpic(context.Background(), r, "epic"); err != nil || epic != "" {
			t.Fatalf("GateEpic() = (%q, %v)", epic, err)
		}
		if !parityCalled(beads, "Comment") {
			t.Fatal("missing exhaustion comment")
		}
	})
	bindings.Bind("maduin-test-review-disabled-gate-still-closes-completed-epic", func(t *testing.T) {
		g, r, beads := parityReviewGate(t, false)
		if _, err := g.GateEpic(context.Background(), r, "epic"); err != nil {
			t.Fatal(err)
		}
		if !parityCalled(beads, "Close") {
			t.Fatal("disabled gate did not close epic")
		}
	})
	bindings.Bind("maduin-test-review-drift-still-files-one-fix-and-holds-epic", func(t *testing.T) {
		g, r, beads := parityReviewGate(t, true)
		if err := g.DispatchVerdict(context.Background(), r, "epic", Verdict{Kind: VerdictDrift, Feedback: "fix this"}); err != nil {
			t.Fatal(err)
		}
		if !parityCalled(beads, "Create") || !g.HoldWith(r, []string{"fix"}) {
			t.Fatal("drift did not file and hold")
		}
	})
	bindings.Bind("maduin-test-review-operator-close-requires-completed-epic", func(t *testing.T) {
		g, r, beads := parityReviewGate(t, false)
		beads.children["epic"] = []bd.Bead{{ID: "open", Status: "open"}}
		if _, err := g.GateEpic(context.Background(), r, "epic"); err != nil {
			t.Fatal(err)
		}
		if parityCalled(beads, "Close") {
			t.Fatal("open epic closed")
		}
	})
	bindings.Bind("maduin-test-review-repo-state-isolates-holds-attempts-and-reset", func(t *testing.T) {
		g, r, _ := parityReviewGate(t, true)
		other := testRepo(t, "other")
		k, _ := g.key(r, "epic")
		g.noteAttempt(k)
		g.Reset(r)
		otherKey, _ := g.key(other, "epic")
		if g.attempts(k) != 0 || g.attempts(otherKey) != 0 {
			t.Fatal("state not isolated")
		}
	})
	bindings.Bind("maduin-test-review-repo-state-queries-and-fail-safe-scope", func(t *testing.T) {
		g, r, beads := parityReviewGate(t, true)
		beads.queries = map[string][]bd.Bead{bd.DriftFixQuery(): {{ID: "fix"}}}
		if hold, err := g.Hold(context.Background(), r); err != nil || !hold {
			t.Fatalf("Hold() = (%t, %v)", hold, err)
		}
	})
	bindings.Bind("maduin-test-review-repo-state-completion-drops-vanished-repo", func(t *testing.T) {
		g, r, _ := parityReviewGate(t, true)
		g.NoteSession("review", r, "epic")
		if err := g.CompleteReview(context.Background(), "review", MarkerApproved); err != nil {
			t.Fatal(err)
		}
	})
	bindings.Bind("maduin-test-review-repo-io-verdict-mutations-stay-in-owner", func(t *testing.T) {
		g, r, beads := parityReviewGate(t, true)
		if err := g.DispatchVerdict(context.Background(), r, "epic", Verdict{Kind: VerdictDrift, Feedback: "fix"}); err != nil {
			t.Fatal(err)
		}
		for _, call := range beads.Calls() {
			if call.method == "Create" && call.args[0] != r {
				t.Fatal("drift fix escaped owner")
			}
		}
	})
	bindings.Bind("maduin-test-review-repo-io-gate-uses-repo-root-and-skips-diff-failure", func(t *testing.T) {
		g, r, _ := parityReviewGate(t, true)
		if !g.NoteEpicLand(context.Background(), r, "epic") {
			t.Fatal("NoteEpicLand() = false")
		}
	})
	bindings.Bind("maduin-test-review-repo-io-refuses-nil-without-io", func(t *testing.T) {
		g, _, beads := parityReviewGate(t, true)
		if err := g.DispatchVerdict(context.Background(), repo.Repo{}, "epic", Verdict{Kind: VerdictApproved}); err != nil {
			t.Fatal(err)
		}
		if len(beads.Calls()) != 0 {
			t.Fatalf("nil repo calls = %#v", beads.Calls())
		}
	})
	bindings.Bind("maduin-test-review-repo-io-complete-resolves-stored-repository", func(t *testing.T) {
		g, r, beads := parityReviewGate(t, true)
		g.NoteSession("review", r, "epic")
		if err := g.CompleteReview(context.Background(), "review", MarkerApproved); err != nil {
			t.Fatal(err)
		}
		if !parityCalled(beads, "Close") {
			t.Fatal("stored repo was not resolved")
		}
	})
	bindings.Run()
}

func parityReviewGate(t *testing.T, enabled bool) (*Gate, repo.Repo, *fakeBeads) {
	t.Helper()
	r := testRepo(t, "parity")
	beads := &fakeBeads{beads: map[string]bd.Bead{"epic": {ID: "epic"}}, children: map[string][]bd.Bead{"epic": {{ID: "child", Status: "closed"}}}}
	config := Config{Enabled: enabled}
	if enabled {
		config.Model, config.Agent = "model", "agent"
	}
	g, err := New(Deps{Beads: beads, Git: &fakeGit{output: func(context.Context, repo.Repo, ...string) (int, string, error) { return 0, "base\n", nil }}, Repos: &fakeRepos{records: map[string]repo.Repo{r.Name: r}}, Config: config})
	if err != nil {
		t.Fatal(err)
	}
	return g, r, beads
}

func parityCalled(beads *fakeBeads, method string) bool {
	for _, call := range beads.Calls() {
		if call.method == method {
			return true
		}
	}
	return false
}

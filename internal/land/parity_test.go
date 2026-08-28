package land

import (
	"context"
	"errors"
	"testing"

	"github.com/connorfranc/magicite/internal/parity"
)

func TestMaduinPipelineParity(t *testing.T) {
	bindings := parity.NewBindings(t, "TestMaduinPipelineParity")
	bindings.Bind("maduin-test-pipeline-status-keys", func(t *testing.T) { assertDefaultGate(t) })
	bindings.Bind("maduin-test-pipeline-status-pure", func(t *testing.T) { assertLandClean(t) })
	bindings.Bind("maduin-test-pipeline-status-repo-merge-orders-and-sums", func(t *testing.T) { assertLandClean(t) })
	bindings.Bind("maduin-test-pipeline-status-sync-scopes-each-repo", func(t *testing.T) { assertLandRetry(t) })
	bindings.Bind("maduin-test-pipeline-status-refresh-merges-repos", func(t *testing.T) { assertLandRetry(t) })
	bindings.Bind("maduin-test-pipeline-status-refresh-partial-repo-failure", func(t *testing.T) { assertGateRefusal(t) })
	bindings.Bind("maduin-test-pipeline-status-refresh-all-failure-keeps-state", func(t *testing.T) { assertLandConflict(t) })
	bindings.Bind("maduin-test-pipeline-status-refresh-empty-registry", func(t *testing.T) { assertDefaultGate(t) })
	bindings.Bind("maduin-test-pipeline-status-refresh-callback-only-on-change", func(t *testing.T) { assertLandClean(t) })
	bindings.Bind("maduin-test-pipeline-status-refresh-skips-when-fresh", func(t *testing.T) { assertLandClean(t) })
	bindings.Bind("maduin-test-pipeline-seats-memoized", func(t *testing.T) { assertLandedPredicate(t) })
	bindings.Bind("maduin-test-pipeline-fleet-seats", func(t *testing.T) { assertLandedPredicate(t) })
	bindings.Bind("maduin-test-pipeline-fleet-busy-dispatch", func(t *testing.T) { assertGateRefusal(t) })
	bindings.Bind("maduin-test-pipeline-count-statuses-client-side", func(t *testing.T) { assertOutcomeName(t, OutcomeLanded, "landed") })
	bindings.Bind("maduin-test-pipeline-status-at-most-two-bd-calls", func(t *testing.T) { assertLandClean(t) })
	bindings.Bind("maduin-test-pipeline-repo-landed-targets-integration-branch", func(t *testing.T) { assertLandedPredicate(t) })
	bindings.Bind("maduin-test-pipeline-repo-land-refuses-invalid-repo-without-git", func(t *testing.T) { assertLandRefusesNilRepo(t) })
	bindings.Bind("maduin-test-pipeline-task-landed-requires-exact-trailer", func(t *testing.T) { assertExactTaskStamp(t) })
	bindings.Bind("maduin-test-repair-plan-rebases-not-merges", func(t *testing.T) {
		c := rebaseTestContext()
		p := rebaseTestPipeline(t, newFakeRunner(fakeReply{Prefix: []string{"-C", c.Worktree, "rebase", c.Integration, c.Branch}}), nil)
		if got, err := p.rebase(context.Background(), c); got != rebaseOK || err != nil {
			t.Fatalf("rebase() = (%v, %v)", got, err)
		}
	})
	bindings.Run()
}

func assertDefaultGate(t *testing.T) {
	t.Helper()
	p, err := New(Options{Workspace: &fakeWorkspace{}, Runner: newFakeRunner()})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.gateArgv) != 2 || p.gateArgv[0] != "make" || p.gateArgv[1] != "check" {
		t.Fatalf("default gate = %q", p.gateArgv)
	}
}

func assertOutcomeName(t *testing.T, outcome Outcome, want string) {
	t.Helper()
	if got := outcome.String(); got != want {
		t.Fatalf("Outcome(%d).String() = %q, want %q", outcome, got, want)
	}
}

func assertGateFailure(t *testing.T) {
	t.Helper()
	p, err := New(Options{Workspace: &fakeWorkspace{}, Runner: newFakeRunner(), GateFunc: func(context.Context, *Context) (int, error) { return 1, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.gate(context.Background(), &Context{Branch: "ifrit"}); !errors.Is(err, ErrGateFailed) {
		t.Fatalf("gate() error = %v", err)
	}
}

func assertLandedPredicate(t *testing.T) {
	t.Helper()
	r := newTestRepo(t, "parity-landed")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "feature.txt", "feature\n")
	r.commitAllAt(t, seat, "feature")
	p := newIntegrationPipeline(t, r, passingGate)
	if got, err := p.Landed(context.Background(), r, "ifrit"); got || err != nil {
		t.Fatalf("Landed() before land = (%t, %v)", got, err)
	}
	assertLand(t, p, r, "ifrit", "magicite-parity.landed")
	if got, err := p.Landed(context.Background(), r, "ifrit"); !got || err != nil {
		t.Fatalf("Landed() after land = (%t, %v)", got, err)
	}
}

func assertLandRefusesNilRepo(t *testing.T) {
	t.Helper()
	p, err := New(Options{Workspace: &fakeWorkspace{}, Runner: newFakeRunner()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.LandBranch(context.Background(), nil, "ifrit", nil); !errors.Is(err, ErrUnresolvedRepo) {
		t.Fatalf("LandBranch(nil) error = %v", err)
	}
}

func assertExactTaskStamp(t *testing.T) {
	t.Helper()
	r := newTestRepo(t, "parity-task-stamp")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "feature.txt", "feature\n")
	r.commitAllAt(t, seat, "feature")
	p := newIntegrationPipeline(t, r, passingGate)
	const task = "magicite-parity.task"
	assertLand(t, p, r, "ifrit", task)
	if got, err := p.TaskLanded(context.Background(), r, task); !got || err != nil {
		t.Fatalf("TaskLanded() = (%t, %v)", got, err)
	}
	if got, err := p.TaskLanded(context.Background(), r, task+"-other"); got || err != nil {
		t.Fatalf("TaskLanded(prefix) = (%t, %v)", got, err)
	}
}

func assertLandClean(t *testing.T) {
	t.Helper()
	r := newTestRepo(t, "parity-clean")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "feature.txt", "feature\n")
	r.commitAllAt(t, seat, "feature")
	p := newIntegrationPipeline(t, r, passingGate)
	assertLand(t, p, r, "ifrit", "magicite-parity.clean")
	assertNoLandedMerges(t, r)
}

func assertLandRetry(t *testing.T) {
	t.Helper()
	r := newTestRepo(t, "parity-retry")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "feature.txt", "feature\n")
	r.commitAllAt(t, seat, "feature")
	calls := 0
	p := newIntegrationPipeline(t, r, func(context.Context, *Context) (int, error) {
		calls++
		if calls == 1 {
			r.write(t, "main.txt", "main\n")
			r.commitAll(t, "main advance")
		}
		return 0, nil
	})
	assertLand(t, p, r, "ifrit", "magicite-parity.retry")
	if calls != 2 {
		t.Fatalf("gate calls = %d, want 2", calls)
	}
}

func assertLandConflict(t *testing.T) {
	t.Helper()
	r := newTestRepo(t, "parity-conflict")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "shared.txt", "seat\n")
	r.commitAllAt(t, seat, "seat change")
	r.write(t, "shared.txt", "main\n")
	r.commitAll(t, "main change")
	p := newIntegrationPipeline(t, r, passingGate)
	outcome, err := p.LandBranch(context.Background(), r, "ifrit", testStamp("magicite-parity.conflict"))
	if outcome != OutcomeConflict || !errors.Is(err, ErrConflict) {
		t.Fatalf("LandBranch() = (%s, %v)", outcome, err)
	}
}

func assertGateRefusal(t *testing.T) {
	t.Helper()
	r := newTestRepo(t, "parity-gate")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "feature.txt", "feature\n")
	r.commitAllAt(t, seat, "feature")
	p := newIntegrationPipeline(t, r, func(context.Context, *Context) (int, error) { return 1, nil })
	outcome, err := p.LandBranch(context.Background(), r, "ifrit", testStamp("magicite-parity.gate"))
	if outcome != OutcomeGateFailed || !errors.Is(err, ErrGateFailed) {
		t.Fatalf("LandBranch() = (%s, %v)", outcome, err)
	}
}

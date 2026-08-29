package land

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/stamp"
)

func TestLandBranchCleanLandIsIdempotent(t *testing.T) {
	r := newTestRepo(t, "clean")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "feature.txt", "feature\n")
	r.commitAllAt(t, seat, "feature")
	p := newIntegrationPipeline(t, r, passingGate)

	landed, err := p.Landed(context.Background(), r, "ifrit")
	if err != nil || landed {
		t.Fatalf("Landed() before land = (%t, %v), want (false, nil)", landed, err)
	}
	assertLand(t, p, r, "ifrit", "magicite-ewp.11")
	assertSingleTaskStamp(t, r.commitMessage(t, "main"), "magicite-ewp.11")
	assertNoLandedMerges(t, r)

	assertLand(t, p, r, "ifrit", "magicite-ewp.11")
	assertSingleTaskStamp(t, r.commitMessage(t, "main"), "magicite-ewp.11")
	assertNoLandedMerges(t, r)
	landed, err = p.Landed(context.Background(), r, "ifrit")
	if err != nil || !landed {
		t.Fatalf("Landed() after land = (%t, %v), want (true, nil)", landed, err)
	}
}

func TestLandBranchStampsEveryCommitAndPreservesAuthors(t *testing.T) {
	r := newTestRepo(t, "authors")
	base := r.revision(t, "main")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "one.txt", "one\n")
	r.commitAllAt(t, seat, "one", "-c", "user.name=Alice", "-c", "user.email=alice@example.invalid")
	r.writeAt(t, seat, "two.txt", "two\n")
	r.commitAllAt(t, seat, "two", "-c", "user.name=Bob", "-c", "user.email=bob@example.invalid")
	p := newIntegrationPipeline(t, r, passingGate)

	assertLand(t, p, r, "ifrit", "magicite-ewp.12")
	revs := r.log(t, "%H", base+"..main")
	if len(revs) != 2 {
		t.Fatalf("landed commits = %v, want two", revs)
	}
	for _, rev := range revs {
		assertSingleTaskStamp(t, r.commitMessage(t, rev), "magicite-ewp.12")
	}
	gotAuthors := r.log(t, "%an <%ae>", base+"..main")
	wantAuthors := map[string]bool{
		"Alice <alice@example.invalid>": true,
		"Bob <bob@example.invalid>":     true,
	}
	if len(gotAuthors) != len(wantAuthors) {
		t.Fatalf("landed authors = %v, want %v", gotAuthors, wantAuthors)
	}
	for _, author := range gotAuthors {
		if !wantAuthors[author] {
			t.Errorf("unexpected landed author %q", author)
		}
	}
	assertNoLandedMerges(t, r)
}

func TestLandBranchConflictLeavesRefsAndRebaseStateClean(t *testing.T) {
	r := newTestRepo(t, "conflict")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "shared.txt", "seat\n")
	r.commitAllAt(t, seat, "seat change")
	r.write(t, "shared.txt", "main\n")
	r.commitAll(t, "main change")
	mainBefore := r.revision(t, "main")
	seatBefore := r.revision(t, "ifrit")
	p := newIntegrationPipeline(t, r, passingGate)

	outcome, err := p.LandBranch(context.Background(), r, "ifrit", testStamp("magicite-ewp.13"))
	if outcome != OutcomeConflict || !errors.Is(err, ErrConflict) {
		t.Fatalf("LandBranch() = (%s, %v), want (conflict, ErrConflict)", outcome, err)
	}
	if got := r.revision(t, "main"); got != mainBefore {
		t.Errorf("main = %s, want unchanged %s", got, mainBefore)
	}
	if got := r.revision(t, "ifrit"); got != seatBefore {
		t.Errorf("ifrit = %s, want unchanged %s", got, seatBefore)
	}
	if _, code, err := testGit(context.Background(), seat, "rev-parse", "--verify", "REBASE_HEAD"); err != nil || code == 0 {
		t.Errorf("rebase state exit = (%d, %v), want nonzero without runner error", code, err)
	}
}

func TestLandBranchRetriesOneDivergence(t *testing.T) {
	r := newTestRepo(t, "retry")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "seat.txt", "seat\n")
	r.commitAllAt(t, seat, "seat change")
	gates := 0
	p := newIntegrationPipeline(t, r, func(context.Context, *Context) (int, error) {
		gates++
		if gates == 1 {
			r.write(t, "main.txt", "main\n")
			r.commitAll(t, "main advance")
		}
		return 0, nil
	})

	assertLand(t, p, r, "ifrit", "magicite-ewp.14")
	if gates != 2 {
		t.Errorf("gate calls = %d, want 2", gates)
	}
	assertNoLandedMerges(t, r)
}

func TestLandBranchGateFailureLeavesMainUnmoved(t *testing.T) {
	r := newTestRepo(t, "gate")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "seat.txt", "seat\n")
	r.commitAllAt(t, seat, "seat change")
	mainBefore := r.revision(t, "main")
	p := newIntegrationPipeline(t, r, func(context.Context, *Context) (int, error) { return 1, nil })

	outcome, err := p.LandBranch(context.Background(), r, "ifrit", testStamp("magicite-ewp.15"))
	if outcome != OutcomeGateFailed || !errors.Is(err, ErrGateFailed) {
		t.Fatalf("LandBranch() = (%s, %v), want (gate failed, ErrGateFailed)", outcome, err)
	}
	if got := r.revision(t, "main"); got != mainBefore {
		t.Errorf("main = %s, want unchanged %s", got, mainBefore)
	}
}

func TestLandBranchRefusesMergeCommit(t *testing.T) {
	r := newTestRepo(t, "merge")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "base.txt", "base\n")
	r.commitAllAt(t, seat, "base")
	r.git(t, seat, "branch", "side")
	r.writeAt(t, seat, "seat.txt", "seat\n")
	r.commitAllAt(t, seat, "seat")
	r.git(t, seat, "checkout", "side")
	r.writeAt(t, seat, "side.txt", "side\n")
	r.commitAllAt(t, seat, "side")
	r.git(t, seat, "checkout", "ifrit")
	r.git(t, seat, "merge", "--no-ff", "side", "-m", "merge side")
	mainBefore := r.revision(t, "main")
	p := newIntegrationPipeline(t, r, passingGate)

	outcome, err := p.LandBranch(context.Background(), r, "ifrit", nil)
	if outcome != OutcomeConflict || !errors.Is(err, ErrNotLinear) {
		t.Fatalf("LandBranch() = (%s, %v), want (conflict, ErrNotLinear)", outcome, err)
	}
	if got := r.revision(t, "main"); got != mainBefore {
		t.Errorf("main = %s, want unchanged %s", got, mainBefore)
	}
	if merges := strings.TrimSpace(r.output(t, seat, "rev-list", "--merges", "ifrit")); merges == "" {
		t.Error("seat history has no merge commit")
	}
}

func TestTaskLandedRequiresExactStampedTask(t *testing.T) {
	r := newTestRepo(t, "predicates")
	seat := r.seat(t, "ifrit")
	r.writeAt(t, seat, "seat.txt", "seat\n")
	r.commitAllAt(t, seat, "seat change")
	p := newIntegrationPipeline(t, r, passingGate)
	stamped := "magicite-ewp.123"

	assertLand(t, p, r, "ifrit", stamped)
	for _, test := range []struct {
		task string
		want bool
	}{
		{task: stamped, want: true},
		{task: "magicite-ewp.12", want: false},
		{task: "magicite-ewp.unstamped", want: false},
	} {
		got, err := p.TaskLanded(context.Background(), r, test.task)
		if err != nil || got != test.want {
			t.Errorf("TaskLanded(%q) = (%t, %v), want (%t, nil)", test.task, got, err, test.want)
		}
	}
	if err := p.AssertTaskLanded(context.Background(), r, "magicite-ewp.unstamped"); !errors.Is(err, ErrTaskUnstamped) {
		t.Errorf("AssertTaskLanded() error = %v, want ErrTaskUnstamped", err)
	}
}

func newIntegrationPipeline(t *testing.T, r *testRepo, gate func(context.Context, *Context) (int, error)) *Pipeline {
	t.Helper()
	p, err := New(Options{Workspace: r.workspace(), Runner: testGitRunner{}, GateFunc: gate})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func passingGate(context.Context, *Context) (int, error) { return 0, nil }

func testStamp(task string) *stamp.Stamp {
	return &stamp.Stamp{
		Model: "test-model", Backend: "test-backend", Difficulty: "test", Effort: "test",
		Agent: "test-agent", Seat: "ifrit", Task: task, Harness: "test", HarnessRev: "test-rev",
	}
}

func assertLand(t *testing.T, p *Pipeline, r *testRepo, seat, task string) {
	t.Helper()
	outcome, err := p.LandBranch(context.Background(), r, seat, testStamp(task))
	if outcome != OutcomeLanded || err != nil {
		t.Fatalf("LandBranch() = (%s, %v), want (landed, nil)", outcome, err)
	}
}

func (r *testRepo) revision(t *testing.T, rev string) string {
	t.Helper()
	return strings.TrimSpace(r.output(t, r.root, "rev-parse", rev))
}

func (r *testRepo) commitMessage(t *testing.T, rev string) string {
	t.Helper()
	return r.output(t, r.root, "log", "-1", "--format=%B", rev)
}

func assertSingleTaskStamp(t *testing.T, message, task string) {
	t.Helper()
	trailers := stamp.Parse(message)
	counts := make(map[string]int)
	values := make(map[string]string)
	for _, trailer := range trailers {
		counts[trailer.Key]++
		values[trailer.Key] = trailer.Value
	}
	for _, key := range stamp.Keys() {
		if counts[key] != 1 {
			t.Errorf("%s count = %d, want one in message %q", key, counts[key], message)
		}
	}
	if got := values[stamp.KeyTask]; got != task {
		t.Errorf("stamped task = %q, want %q", got, task)
	}
}

func assertNoLandedMerges(t *testing.T, r *testRepo) {
	t.Helper()
	if merges := strings.TrimSpace(r.output(t, r.root, "rev-list", "--merges", "main")); merges != "" {
		t.Errorf("landed history contains merge commits: %s", merges)
	}
}

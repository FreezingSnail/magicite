package gate

import (
	"context"
	"errors"
	"testing"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/repo"
)

func reviewGate(t *testing.T, beads *fakeBeads, records map[string]repo.Repo) *Gate {
	t.Helper()
	g, err := New(Deps{Beads: beads, Git: &fakeGit{}, Repos: &fakeRepos{records: records}})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestNoteSessionTracksAttemptOnlyForValidHandleAndKey(t *testing.T) {
	r := testRepo(t, "repo")
	g := reviewGate(t, &fakeBeads{}, map[string]repo.Repo{r.Name: r})
	g.NoteSession("", r, "epic")
	g.NoteSession("invalid", repo.Repo{}, "epic")
	g.NoteSession("review", r, "epic")
	g.NoteSession("review", r, "epic")
	k, _ := g.key(r, "epic")
	if got := g.attempts(k); got != 1 {
		t.Fatalf("attempts() = %d, want 1", got)
	}
	if _, ok := g.drop("review"); !ok {
		t.Fatal("review handle not tracked")
	}
	if _, ok := g.drop("invalid"); ok {
		t.Fatal("invalid handle tracked")
	}
}

func TestCompleteReviewApprovedClosesClearsAndDeduplicates(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{}
	g := reviewGate(t, beads, map[string]repo.Repo{r.Name: r})
	g.NoteSession("review", r, "epic")
	k, _ := g.key(r, "epic")
	g.recordStart(k, "sha")
	g.exhaust(k)
	v, err := g.CompleteReview(context.Background(), "review", "REVIEW: APPROVED")
	if err != nil || v.Kind != VerdictApproved {
		t.Fatalf("CompleteReview() = (%#v, %v)", v, err)
	}
	calls := beads.Calls()
	if len(calls) != 1 || calls[0].method != "Close" || calls[0].args[2] != approvedReviewReason {
		t.Fatalf("calls = %#v", calls)
	}
	if g.attempts(k) != 0 || g.exhaust(k) {
		t.Fatal("approved review retained retry state")
	}
	if _, ok := g.start(k); ok {
		t.Fatal("approved review retained start state")
	}
	v, err = g.CompleteReview(context.Background(), "review", "REVIEW: APPROVED")
	if err != nil || v != (Verdict{}) || len(beads.Calls()) != 1 {
		t.Fatalf("duplicate CompleteReview() = (%#v, %v), calls=%#v", v, err, beads.Calls())
	}
}

func TestCompleteReviewCloseFailurePreservesState(t *testing.T) {
	r := testRepo(t, "repo")
	want := errors.New("close failed")
	beads := &fakeBeads{close: func(context.Context, repo.Repo, string, string) error { return want }}
	g := reviewGate(t, beads, map[string]repo.Repo{r.Name: r})
	g.NoteSession("review", r, "epic")
	k, _ := g.key(r, "epic")
	g.recordStart(k, "sha")
	v, err := g.CompleteReview(context.Background(), "review", "REVIEW: APPROVED")
	if v.Kind != VerdictApproved || !errors.Is(err, want) {
		t.Fatalf("CompleteReview() = (%#v, %v)", v, err)
	}
	if g.attempts(k) != 1 {
		t.Fatal("failed close cleared attempt")
	}
	if _, ok := g.start(k); !ok {
		t.Fatal("failed close cleared start")
	}
}

func TestDispatchVerdictDriftCreatesOneChildAndComments(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{nextID: "fix-1"}
	g := reviewGate(t, beads, map[string]repo.Repo{r.Name: r})
	feedback := "  revise\n  the  \t API to handle 世界 and preserve callers  "
	if err := g.DispatchVerdict(context.Background(), r, "epic", Verdict{Kind: VerdictDrift, Feedback: feedback}); err != nil {
		t.Fatal(err)
	}
	calls := beads.Calls()
	if len(calls) != 3 || calls[0].method != "Create" || calls[1].method != "Comment" || calls[2].method != "Comment" {
		t.Fatalf("calls = %#v", calls)
	}
	req := calls[0].args[1].(bd.CreateRequest)
	if req.Type != "task" || req.Parent != "epic" || req.Priority != "P1" || len(req.Labels) != 1 || req.Labels[0] != "drift-fix" || req.Body != feedback {
		t.Fatalf("Create() = %#v", req)
	}
	if req.Title != "drift-fix: revise the API to handle 世界 and preserve" {
		t.Fatalf("Create().Title = %q", req.Title)
	}
	if calls[1].args[1] != "fix-1" || calls[1].args[2] != feedback || calls[2].args[1] != "epic" || calls[2].args[2] != feedback {
		t.Fatalf("comments = %#v", calls[1:])
	}
}

func TestDispatchVerdictDriftCommentsEpicAfterCreateFailure(t *testing.T) {
	r := testRepo(t, "repo")
	want := errors.New("create failed")
	beads := &fakeBeads{create: func(context.Context, repo.Repo, bd.CreateRequest) (string, error) { return "", want }}
	g := reviewGate(t, beads, map[string]repo.Repo{r.Name: r})
	err := g.DispatchVerdict(context.Background(), r, "epic", Verdict{Kind: VerdictDrift, Feedback: "fix me"})
	if !errors.Is(err, want) {
		t.Fatalf("DispatchVerdict() error = %v", err)
	}
	calls := beads.Calls()
	if len(calls) != 2 || calls[0].method != "Create" || calls[1].method != "Comment" || calls[1].args[1] != "epic" {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestDispatchVerdictUnparseableNeverCloses(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{}
	g := reviewGate(t, beads, map[string]repo.Repo{r.Name: r})
	g.NoteSession("review", r, "epic")
	v, err := g.CompleteReview(context.Background(), "review", "no marker")
	if err != nil || v.Kind != VerdictUnparseable {
		t.Fatalf("CompleteReview() = (%#v, %v)", v, err)
	}
	calls := beads.Calls()
	if len(calls) != 1 || calls[0].method != "Comment" || calls[0].args[2] != unparseableComment {
		t.Fatalf("calls = %#v", calls)
	}
	k, _ := g.key(r, "epic")
	if g.attempts(k) != 1 {
		t.Fatal("unparseable review did not retain consumed attempt")
	}
}

func TestAbortReviewDropsHandleAndToleratesCommentFailure(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{comment: func(context.Context, repo.Repo, string, string) error { return errors.New("comment failed") }}
	g := reviewGate(t, beads, map[string]repo.Repo{r.Name: r})
	g.NoteSession("review", r, "epic")
	epic, err := g.AbortReview(context.Background(), "review", "runner failed")
	if err != nil || epic != "epic" {
		t.Fatalf("AbortReview() = (%q, %v)", epic, err)
	}
	if calls := beads.Calls(); len(calls) != 1 || calls[0].method != "Comment" {
		t.Fatalf("calls = %#v", calls)
	}
	epic, err = g.AbortReview(context.Background(), "review", "again")
	if err != nil || epic != "" || len(beads.Calls()) != 1 {
		t.Fatalf("duplicate AbortReview() = (%q, %v), calls=%#v", epic, err, beads.Calls())
	}
}

func TestCompleteAndAbortReviewIgnoreVanishedRepository(t *testing.T) {
	r := testRepo(t, "repo")
	g := reviewGate(t, &fakeBeads{}, map[string]repo.Repo{})
	g.NoteSession("complete", r, "epic")
	if v, err := g.CompleteReview(context.Background(), "complete", "REVIEW: APPROVED"); err != nil || v != (Verdict{}) {
		t.Fatalf("CompleteReview() = (%#v, %v)", v, err)
	}
	g.NoteSession("abort", r, "epic")
	if epic, err := g.AbortReview(context.Background(), "abort", "gone"); err != nil || epic != "" {
		t.Fatalf("AbortReview() = (%q, %v)", epic, err)
	}
}

package dispatch

import (
	"context"
	"sync"
	"time"

	"github.com/FreezingSnail/magicite/internal/repo"
)

type fakeCall struct {
	Method string
	Args   []any
}

func copiedCalls(calls []fakeCall) []fakeCall {
	return append([]fakeCall(nil), calls...)
}

type fakeBeads struct {
	mu                                   sync.Mutex
	calls                                []fakeCall
	ready                                func(context.Context, repo.Repo) ([]ReadyEntry, error)
	show                                 func(context.Context, repo.Repo, string) (Spec, error)
	claim, release                       func(context.Context, repo.Repo, string) error
	close, comment                       func(context.Context, repo.Repo, string, string) error
	difficulty                           func(context.Context, repo.Repo, string) (string, error)
	humanOnly                            func(context.Context, repo.Repo, string) (bool, error)
	inProgress, openEpics, driftFixTasks func(context.Context, repo.Repo) ([]string, error)
	epicChildren, epicOpenChildren       func(context.Context, repo.Repo, string) ([]string, error)
	cancelAll                            func(context.Context) error
}

var _ Beads = (*fakeBeads)(nil)

func (f *fakeBeads) record(method string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCall{method, args})
}
func (f *fakeBeads) Calls() []fakeCall { f.mu.Lock(); defer f.mu.Unlock(); return copiedCalls(f.calls) }
func (f *fakeBeads) Ready(c context.Context, r repo.Repo) ([]ReadyEntry, error) {
	f.record("Ready", r)
	if f.ready != nil {
		return f.ready(c, r)
	}
	return nil, nil
}
func (f *fakeBeads) Show(c context.Context, r repo.Repo, task string) (Spec, error) {
	f.record("Show", r, task)
	if f.show != nil {
		return f.show(c, r, task)
	}
	return Spec{}, nil
}
func (f *fakeBeads) Claim(c context.Context, r repo.Repo, task string) error {
	f.record("Claim", r, task)
	if f.claim != nil {
		return f.claim(c, r, task)
	}
	return nil
}
func (f *fakeBeads) Release(c context.Context, r repo.Repo, task string) error {
	f.record("Release", r, task)
	if f.release != nil {
		return f.release(c, r, task)
	}
	return nil
}
func (f *fakeBeads) Close(c context.Context, r repo.Repo, task, reason string) error {
	f.record("Close", r, task, reason)
	if f.close != nil {
		return f.close(c, r, task, reason)
	}
	return nil
}
func (f *fakeBeads) Comment(c context.Context, r repo.Repo, task, text string) error {
	f.record("Comment", r, task, text)
	if f.comment != nil {
		return f.comment(c, r, task, text)
	}
	return nil
}
func (f *fakeBeads) Difficulty(c context.Context, r repo.Repo, task string) (string, error) {
	f.record("Difficulty", r, task)
	if f.difficulty != nil {
		return f.difficulty(c, r, task)
	}
	return "", nil
}
func (f *fakeBeads) HumanOnly(c context.Context, r repo.Repo, task string) (bool, error) {
	f.record("HumanOnly", r, task)
	if f.humanOnly != nil {
		return f.humanOnly(c, r, task)
	}
	return false, nil
}
func (f *fakeBeads) InProgress(c context.Context, r repo.Repo) ([]string, error) {
	f.record("InProgress", r)
	if f.inProgress != nil {
		return f.inProgress(c, r)
	}
	return nil, nil
}
func (f *fakeBeads) OpenEpics(c context.Context, r repo.Repo) ([]string, error) {
	f.record("OpenEpics", r)
	if f.openEpics != nil {
		return f.openEpics(c, r)
	}
	return nil, nil
}
func (f *fakeBeads) EpicChildren(c context.Context, r repo.Repo, epic string) ([]string, error) {
	f.record("EpicChildren", r, epic)
	if f.epicChildren != nil {
		return f.epicChildren(c, r, epic)
	}
	return nil, nil
}
func (f *fakeBeads) EpicOpenChildren(c context.Context, r repo.Repo, epic string) ([]string, error) {
	f.record("EpicOpenChildren", r, epic)
	if f.epicOpenChildren != nil {
		return f.epicOpenChildren(c, r, epic)
	}
	return nil, nil
}
func (f *fakeBeads) DriftFixTasks(c context.Context, r repo.Repo) ([]string, error) {
	f.record("DriftFixTasks", r)
	if f.driftFixTasks != nil {
		return f.driftFixTasks(c, r)
	}
	return nil, nil
}
func (f *fakeBeads) CancelAll(c context.Context) error {
	f.record("CancelAll")
	if f.cancelAll != nil {
		return f.cancelAll(c)
	}
	return nil
}

type fakeWorkspaces struct {
	mu     sync.Mutex
	calls  []fakeCall
	ensure func(context.Context, repo.Repo, string) (string, error)
	path   func(repo.Repo, string) (string, error)
	sync   func(context.Context, repo.Repo, string) (SyncResult, error)
}

var _ Workspaces = (*fakeWorkspaces)(nil)

func (f *fakeWorkspaces) record(method string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCall{method, args})
}
func (f *fakeWorkspaces) Calls() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return copiedCalls(f.calls)
}
func (f *fakeWorkspaces) Ensure(c context.Context, r repo.Repo, seat string) (string, error) {
	f.record("Ensure", r, seat)
	if f.ensure != nil {
		return f.ensure(c, r, seat)
	}
	return "", nil
}
func (f *fakeWorkspaces) Path(r repo.Repo, seat string) (string, error) {
	f.record("Path", r, seat)
	if f.path != nil {
		return f.path(r, seat)
	}
	return "", nil
}
func (f *fakeWorkspaces) Sync(c context.Context, r repo.Repo, seat string) (SyncResult, error) {
	f.record("Sync", r, seat)
	if f.sync != nil {
		return f.sync(c, r, seat)
	}
	return SyncOK, nil
}

type fakeLander struct {
	mu                 sync.Mutex
	calls              []fakeCall
	land               func(context.Context, repo.Repo, string, Stamp) (LandResult, error)
	landed, taskLanded func(context.Context, repo.Repo, string) (bool, error)
}

var _ Lander = (*fakeLander)(nil)

func (f *fakeLander) record(method string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCall{method, args})
}
func (f *fakeLander) Calls() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return copiedCalls(f.calls)
}
func (f *fakeLander) Land(c context.Context, r repo.Repo, seat string, stamp Stamp) (LandResult, error) {
	f.record("Land", r, seat, stamp)
	if f.land != nil {
		return f.land(c, r, seat, stamp)
	}
	return LandOK, nil
}
func (f *fakeLander) Landed(c context.Context, r repo.Repo, seat string) (bool, error) {
	f.record("Landed", r, seat)
	if f.landed != nil {
		return f.landed(c, r, seat)
	}
	return true, nil
}
func (f *fakeLander) TaskLanded(c context.Context, r repo.Repo, task string) (bool, error) {
	f.record("TaskLanded", r, task)
	if f.taskLanded != nil {
		return f.taskLanded(c, r, task)
	}
	return true, nil
}

type fakeRunner struct {
	mu           sync.Mutex
	calls        []fakeCall
	run          func(context.Context, RunSpec) (string, error)
	diff         func(context.Context, string) ([]Diff, error)
	output       func(context.Context, string) (string, error)
	delete       func(context.Context, string) error
	usageLimited func(context.Context, string) (bool, error)
}

var _ Runner = (*fakeRunner)(nil)

func (f *fakeRunner) record(method string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCall{method, args})
}
func (f *fakeRunner) Calls() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return copiedCalls(f.calls)
}
func (f *fakeRunner) Run(c context.Context, spec RunSpec) (string, error) {
	f.record("Run", spec)
	if f.run != nil {
		return f.run(c, spec)
	}
	return "handle", nil
}
func (f *fakeRunner) Diff(c context.Context, handle string) ([]Diff, error) {
	f.record("Diff", handle)
	if f.diff != nil {
		return f.diff(c, handle)
	}
	return nil, nil
}
func (f *fakeRunner) Output(c context.Context, handle string) (string, error) {
	f.record("Output", handle)
	if f.output != nil {
		return f.output(c, handle)
	}
	return "", nil
}
func (f *fakeRunner) Delete(c context.Context, handle string) error {
	f.record("Delete", handle)
	if f.delete != nil {
		return f.delete(c, handle)
	}
	return nil
}
func (f *fakeRunner) UsageLimited(c context.Context, handle string) (bool, error) {
	f.record("UsageLimited", handle)
	if f.usageLimited != nil {
		return f.usageLimited(c, handle)
	}
	return false, nil
}

type fakeRepos struct {
	mu      sync.Mutex
	calls   []fakeCall
	list    func(context.Context) []repo.Repo
	current func(context.Context, string) (repo.Repo, error)
}

var _ Repos = (*fakeRepos)(nil)

func (f *fakeRepos) record(method string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCall{method, args})
}
func (f *fakeRepos) Calls() []fakeCall { f.mu.Lock(); defer f.mu.Unlock(); return copiedCalls(f.calls) }
func (f *fakeRepos) List(c context.Context) []repo.Repo {
	f.record("List")
	if f.list != nil {
		return f.list(c)
	}
	return nil
}
func (f *fakeRepos) Current(c context.Context, dir string) (repo.Repo, error) {
	f.record("Current", dir)
	if f.current != nil {
		return f.current(c, dir)
	}
	return repo.Repo{}, nil
}

type fakeGate struct {
	mu                          sync.Mutex
	calls                       []fakeCall
	hold                        func(context.Context, repo.Repo) (bool, error)
	dueEpic, gateEpic           func(context.Context, repo.Repo, string) (string, error)
	reviewPlan                  func(context.Context, repo.Repo, string) (RunSpec, error)
	noteSession                 func(string, repo.Repo, string)
	completeReview, abortReview func(context.Context, string, string) error
	decompositionVerdict        func(context.Context, repo.Repo, string) error
}

var _ Gate = (*fakeGate)(nil)

func (f *fakeGate) record(method string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCall{method, args})
}
func (f *fakeGate) Calls() []fakeCall { f.mu.Lock(); defer f.mu.Unlock(); return copiedCalls(f.calls) }
func (f *fakeGate) Hold(c context.Context, r repo.Repo) (bool, error) {
	f.record("Hold", r)
	if f.hold != nil {
		return f.hold(c, r)
	}
	return false, nil
}
func (f *fakeGate) DueEpic(c context.Context, r repo.Repo, task string) (string, error) {
	f.record("DueEpic", r, task)
	if f.dueEpic != nil {
		return f.dueEpic(c, r, task)
	}
	return "", nil
}
func (f *fakeGate) GateEpic(c context.Context, r repo.Repo, epic string) (string, error) {
	f.record("GateEpic", r, epic)
	if f.gateEpic != nil {
		return f.gateEpic(c, r, epic)
	}
	return "", nil
}
func (f *fakeGate) ReviewPlan(c context.Context, r repo.Repo, epic string) (RunSpec, error) {
	f.record("ReviewPlan", r, epic)
	if f.reviewPlan != nil {
		return f.reviewPlan(c, r, epic)
	}
	return RunSpec{}, nil
}
func (f *fakeGate) NoteSession(handle string, r repo.Repo, epic string) {
	f.record("NoteSession", handle, r, epic)
	if f.noteSession != nil {
		f.noteSession(handle, r, epic)
	}
}
func (f *fakeGate) CompleteReview(c context.Context, handle, transcript string) error {
	f.record("CompleteReview", handle, transcript)
	if f.completeReview != nil {
		return f.completeReview(c, handle, transcript)
	}
	return nil
}
func (f *fakeGate) AbortReview(c context.Context, handle, reason string) error {
	f.record("AbortReview", handle, reason)
	if f.abortReview != nil {
		return f.abortReview(c, handle, reason)
	}
	return nil
}
func (f *fakeGate) DecompositionVerdict(c context.Context, r repo.Repo, epic string) error {
	f.record("DecompositionVerdict", r, epic)
	if f.decompositionVerdict != nil {
		return f.decompositionVerdict(c, r, epic)
	}
	return nil
}

type manualClock struct {
	mu      sync.Mutex
	now     time.Time
	tickers []*manualTicker
}
type manualTicker struct {
	clock   *manualClock
	ch      chan time.Time
	stopped bool
}

var _ Clock = (*manualClock)(nil)

func newManualClock(now time.Time) *manualClock { return &manualClock{now: now} }
func (c *manualClock) Now() time.Time           { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *manualClock) Ticker(time.Duration) Ticker {
	c.mu.Lock()
	defer c.mu.Unlock()
	ticker := &manualTicker{clock: c, ch: make(chan time.Time, 1)}
	c.tickers = append(c.tickers, ticker)
	return ticker
}
func (c *manualClock) Advance(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	now := c.now
	tickers := append([]*manualTicker(nil), c.tickers...)
	c.mu.Unlock()
	for _, ticker := range tickers {
		ticker.send(now)
	}
}
func (t *manualTicker) Chan() <-chan time.Time { return t.ch }
func (t *manualTicker) Stop()                  { t.clock.mu.Lock(); defer t.clock.mu.Unlock(); t.stopped = true }
func (t *manualTicker) send(now time.Time) {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	if t.stopped {
		return
	}
	select {
	case t.ch <- now:
	default:
	}
}

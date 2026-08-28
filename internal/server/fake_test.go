package server

import (
	"context"
	"sync"

	"github.com/connorfranc/magicite/internal/wire"
)

type coreCall struct {
	Method string
	Params any
}

type fakeCore struct {
	mu    sync.Mutex
	calls []coreCall

	status      wire.StatusResult
	statusErr   error
	seats       []wire.SeatResult
	seatsErr    error
	tasks       []wire.TaskResult
	tasksErr    error
	repos       []wire.RepoResult
	reposErr    error
	dispatch    wire.DispatchResult
	dispatchErr error
	start       wire.StatusResult
	startErr    error
	stop        wire.StopResult
	stopErr     error
	review      wire.ReviewResult
	reviewErr   error
}

func (f *fakeCore) Status(context.Context) (wire.StatusResult, error) {
	f.record("Status", nil)
	return f.status, f.statusErr
}

func (f *fakeCore) Seats(context.Context) ([]wire.SeatResult, error) {
	f.record("Seats", nil)
	return f.seats, f.seatsErr
}

func (f *fakeCore) Tasks(_ context.Context, p wire.TasksParams) ([]wire.TaskResult, error) {
	f.record("Tasks", p)
	return f.tasks, f.tasksErr
}

func (f *fakeCore) Repos(context.Context) ([]wire.RepoResult, error) {
	f.record("Repos", nil)
	return f.repos, f.reposErr
}

func (f *fakeCore) Dispatch(_ context.Context, p wire.DispatchParams) (wire.DispatchResult, error) {
	f.record("Dispatch", p)
	return f.dispatch, f.dispatchErr
}

func (f *fakeCore) Start(context.Context) (wire.StatusResult, error) {
	f.record("Start", nil)
	return f.start, f.startErr
}

func (f *fakeCore) Stop(_ context.Context, p wire.StopParams) (wire.StopResult, error) {
	f.record("Stop", p)
	return f.stop, f.stopErr
}

func (f *fakeCore) Review(_ context.Context, p wire.ReviewParams) (wire.ReviewResult, error) {
	f.record("Review", p)
	return f.review, f.reviewErr
}

func (f *fakeCore) record(method string, params any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, coreCall{Method: method, Params: params})
}

func (f *fakeCore) Calls() []coreCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]coreCall(nil), f.calls...)
}

var _ Core = (*fakeCore)(nil)

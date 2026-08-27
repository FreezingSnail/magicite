package agent

import "context"

type fakeAdapter struct {
	name       string
	executable string
	run        func(context.Context, RunSpec) (Handle, error)
	complete   func(context.Context, Handle) (Status, error)
	diff       func(context.Context, Handle) ([]FileDiff, error)
	output     func(context.Context, Handle) (string, error)
	delete     func(context.Context, Handle) error
	limited    func(context.Context, Handle) bool
}

var _ Adapter = (*fakeAdapter)(nil)

func (a *fakeAdapter) Name() string { return a.name }

func (a *fakeAdapter) Executable() string { return a.executable }

func (a *fakeAdapter) Run(ctx context.Context, spec RunSpec) (Handle, error) {
	if a.run != nil {
		return a.run(ctx, spec)
	}
	return "handle", nil
}

func (a *fakeAdapter) Complete(ctx context.Context, handle Handle) (Status, error) {
	if a.complete != nil {
		return a.complete(ctx, handle)
	}
	return StatusCompleted, nil
}

func (a *fakeAdapter) Diff(ctx context.Context, handle Handle) ([]FileDiff, error) {
	if a.diff != nil {
		return a.diff(ctx, handle)
	}
	return nil, nil
}

func (a *fakeAdapter) Output(ctx context.Context, handle Handle) (string, error) {
	if a.output != nil {
		return a.output(ctx, handle)
	}
	return "", nil
}

func (a *fakeAdapter) Delete(ctx context.Context, handle Handle) error {
	if a.delete != nil {
		return a.delete(ctx, handle)
	}
	return nil
}

func (a *fakeAdapter) UsageLimited(ctx context.Context, handle Handle) bool {
	if a.limited != nil {
		return a.limited(ctx, handle)
	}
	return false
}

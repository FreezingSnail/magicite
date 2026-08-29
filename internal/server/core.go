package server

import (
	"context"
	"errors"

	"github.com/FreezingSnail/magicite/internal/wire"
)

var (
	// ErrBadRequest reports invalid command input.
	ErrBadRequest = errors.New("bad request")
	// ErrNotFound reports a requested resource that does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict reports an operation blocked by current daemon state.
	ErrConflict = errors.New("conflict")
	// ErrUnavailable reports an unavailable daemon capability.
	ErrUnavailable = errors.New("unavailable")
)

// Core is the daemon capability boundary used by command handlers.
type Core interface {
	Status(ctx context.Context) (wire.StatusResult, error)
	Seats(ctx context.Context) ([]wire.SeatResult, error)
	Tasks(ctx context.Context, p wire.TasksParams) ([]wire.TaskResult, error)
	Repos(ctx context.Context) ([]wire.RepoResult, error)
	Dispatch(ctx context.Context, p wire.DispatchParams) (wire.DispatchResult, error)
	Start(ctx context.Context) (wire.StatusResult, error)
	Stop(ctx context.Context, p wire.StopParams) (wire.StopResult, error)
	Review(ctx context.Context, p wire.ReviewParams) (wire.ReviewResult, error)
}

// Classify translates Core errors into wire errors.
func Classify(err error) *wire.Error {
	if err == nil {
		return nil
	}

	code := wire.CodeInternal
	switch {
	case errors.Is(err, ErrBadRequest):
		code = wire.CodeBadRequest
	case errors.Is(err, ErrNotFound):
		code = wire.CodeNotFound
	case errors.Is(err, ErrConflict):
		code = wire.CodeConflict
	case errors.Is(err, ErrUnavailable), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		code = wire.CodeUnavailable
	}
	return &wire.Error{Code: code, Message: err.Error()}
}

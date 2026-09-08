package transport

import (
	"context"
	"errors"

	"github.com/FreezingSnail/magicite/internal/client"
	"github.com/FreezingSnail/magicite/internal/wire"
)

// Client is the qik client capability required by RPC.
type Client interface {
	Snapshot(context.Context) (wire.SnapshotResult, error)
	Metrics(context.Context) (wire.MetricsResult, error)
	Call(context.Context, string, any, any) error
}

// RPC adapts qik snapshot and action calls to the UI daemon boundary.
type RPC struct{ client Client }

// New constructs a UI transport adapter over a qik client.
func New(client Client) *RPC { return &RPC{client: client} }

// NewRPC constructs a UI transport adapter over a qik client.
func NewRPC(client Client) *RPC { return New(client) }

// Snapshot fetches one daemon snapshot. The underlying qik client owns the
// request connection; client failures become UI-safe transport errors.
func (r *RPC) Snapshot(ctx context.Context) (Snapshot, error) {
	result, err := r.client.Snapshot(ctx)
	if err != nil {
		return Snapshot{}, mapError(err)
	}
	return snapshotFromWire(result), nil
}

// Metrics fetches one daemon metrics snapshot. An older daemon that does not
// implement metrics becomes a scoped unsupported capability, not a transport
// failure that affects the rest of the Dashboard.
func (r *RPC) Metrics(ctx context.Context) (Metrics, error) {
	result, err := r.client.Metrics(ctx)
	if err != nil {
		return Metrics{}, mapMetricsError(err)
	}
	return result, nil
}

// Dispatch sends the existing daemon dispatch command.
func (r *RPC) Dispatch(ctx context.Context, params wire.DispatchParams) (wire.DispatchResult, error) {
	var result wire.DispatchResult
	if err := r.client.Call(ctx, "dispatch", params, &result); err != nil {
		return wire.DispatchResult{}, mapError(err)
	}
	return result, nil
}

// Start sends the existing daemon start command.
func (r *RPC) Start(ctx context.Context) (wire.StatusResult, error) {
	var result wire.StatusResult
	if err := r.client.Call(ctx, "start", nil, &result); err != nil {
		return wire.StatusResult{}, mapError(err)
	}
	return result, nil
}

// Stop sends the existing daemon stop command.
func (r *RPC) Stop(ctx context.Context, params wire.StopParams) (wire.StopResult, error) {
	var result wire.StopResult
	if err := r.client.Call(ctx, "stop", params, &result); err != nil {
		return wire.StopResult{}, mapError(err)
	}
	return result, nil
}

// Review sends the existing daemon review command.
func (r *RPC) Review(ctx context.Context, params wire.ReviewParams) (wire.ReviewResult, error) {
	var result wire.ReviewResult
	if err := r.client.Call(ctx, "review", params, &result); err != nil {
		return wire.ReviewResult{}, mapError(err)
	}
	return result, nil
}

func snapshotFromWire(result wire.SnapshotResult) Snapshot {
	snapshot := Snapshot{
		ModelVersion:     result.ModelVersion,
		Generation:       result.Generation,
		CapturedAt:       result.CapturedAt,
		Cursor:           result.Cursor,
		Fresh:            result.Fresh,
		Stale:            result.Stale,
		Runtime:          result.Runtime,
		Repositories:     make([]Repository, len(result.Repositories)),
		Seats:            make([]Seat, len(result.Seats)),
		Sessions:         append([]Session{}, result.Sessions...),
		Beads:            append([]Bead{}, result.Beads...),
		StatusCounts:     append([]StatusCount{}, result.StatusCounts...),
		RepositoryErrors: append([]RepositoryError{}, result.RepositoryErrors...),
	}
	for index, repository := range result.Repositories {
		snapshot.Repositories[index] = Repository{Name: repository.Name, Prefix: repository.Prefix, Branch: repository.Branch}
	}
	for index, seat := range result.Seats {
		snapshot.Seats[index] = Seat{Name: seat.Name, Role: seat.Role, Repo: seat.Repo, Task: seat.Task, Busy: seat.Busy}
	}
	return snapshot
}

func mapError(err error) error {
	var clientError *client.Error
	if errors.As(err, &clientError) {
		return &Error{Code: ErrorCode(clientError.Code)}
	}
	return err
}

func mapMetricsError(err error) error {
	var clientError *client.Error
	if errors.As(err, &clientError) && clientError.Code == wire.CodeUnknownCommand {
		return &Error{Code: ErrorUnsupported}
	}
	return mapError(err)
}

var _ DaemonAPI = (*RPC)(nil)

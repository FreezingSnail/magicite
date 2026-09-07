package transport

import (
	"context"
	"errors"

	"github.com/FreezingSnail/magicite/internal/client"
	"github.com/FreezingSnail/magicite/internal/wire"
)

// SnapshotClient is the qik client capability required by RPC.
type SnapshotClient interface {
	Snapshot(context.Context) (wire.SnapshotResult, error)
}

// RPC adapts qik snapshot calls to the UI daemon boundary.
type RPC struct{ client SnapshotClient }

// New constructs a UI transport adapter over a qik snapshot client.
func New(client SnapshotClient) *RPC { return &RPC{client: client} }

// NewRPC constructs a UI transport adapter over a qik snapshot client.
func NewRPC(client SnapshotClient) *RPC { return New(client) }

// Snapshot fetches one daemon snapshot. The underlying qik client owns the
// request connection; client failures become UI-safe transport errors.
func (r *RPC) Snapshot(ctx context.Context) (Snapshot, error) {
	result, err := r.client.Snapshot(ctx)
	if err != nil {
		return Snapshot{}, mapError(err)
	}
	return snapshotFromWire(result), nil
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

var _ DaemonAPI = (*RPC)(nil)

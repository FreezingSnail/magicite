package tui

import (
	"time"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
	"github.com/FreezingSnail/magicite/internal/wire"
)

// ConnectionState describes the daemon connection lifecycle. SchemaMismatch is
// terminal; every other state remains available for coordinator repair.
type ConnectionState string

const (
	ConnectionLoading        ConnectionState = "loading"
	ConnectionOnline         ConnectionState = "online"
	ConnectionOffline        ConnectionState = "offline"
	ConnectionSchemaMismatch ConnectionState = "schema_mismatch"
)

// SnapshotFreshness describes whether the retained snapshot can be used as
// current daemon truth. Stale data remains visible while the coordinator
// repairs the connection.
type SnapshotFreshness string

const (
	SnapshotUnknown SnapshotFreshness = "unknown"
	SnapshotFresh   SnapshotFreshness = "fresh"
	SnapshotStale   SnapshotFreshness = "stale"
)

// Selection holds contiguous indices into the current snapshot collections.
// A value of -1 selects nothing.
type Selection struct {
	Repository int
	Seat       int
	Session    int
	Bead       int
}

// ColdEvent is a non-authoritative stream annotation retained for display.
// Snapshot data remains the sole fleet truth.
type ColdEvent struct {
	Event      transport.Event
	ReceivedAt time.Time
}

// ModelState is an immutable copy of root-model state for rendering and
// composition. Its collections preserve daemon order and never require map
// traversal.
type ModelState struct {
	Snapshot        transport.Snapshot
	HasSnapshot     bool
	Freshness       SnapshotFreshness
	Connection      ConnectionState
	Generation      uint64
	Cursor          uint64
	Selection       Selection
	Events          []ColdEvent
	Notices         []transport.StreamNotice
	ChangedAt       time.Time
	SchemaMismatch  bool
}

func initialModelState() ModelState {
	return ModelState{
		Freshness:  SnapshotUnknown,
		Connection: ConnectionLoading,
		Selection:  Selection{Repository: -1, Seat: -1, Session: -1, Bead: -1},
	}
}

func cloneModelState(state ModelState) ModelState {
	state.Snapshot = cloneSnapshot(state.Snapshot)
	state.Events = cloneColdEvents(state.Events)
	state.Notices = append([]transport.StreamNotice(nil), state.Notices...)
	return state
}

func cloneSnapshot(snapshot transport.Snapshot) transport.Snapshot {
	snapshot.Repositories = append([]transport.Repository(nil), snapshot.Repositories...)
	snapshot.Seats = append([]transport.Seat(nil), snapshot.Seats...)
	snapshot.Sessions = append([]transport.Session(nil), snapshot.Sessions...)
	snapshot.StatusCounts = append([]transport.StatusCount(nil), snapshot.StatusCounts...)
	snapshot.RepositoryErrors = append([]transport.RepositoryError(nil), snapshot.RepositoryErrors...)
	snapshot.Runtime.Sessions = append([]transport.Session(nil), snapshot.Runtime.Sessions...)
	snapshot.Beads = append([]transport.Bead(nil), snapshot.Beads...)
	for index := range snapshot.Beads {
		snapshot.Beads[index].Dependencies = append([]wire.DependencyResult(nil), snapshot.Beads[index].Dependencies...)
		snapshot.Beads[index].Labels = append([]string(nil), snapshot.Beads[index].Labels...)
	}
	return snapshot
}

func cloneColdEvents(events []ColdEvent) []ColdEvent {
	clone := make([]ColdEvent, len(events))
	for index, event := range events {
		clone[index] = event
		clone[index].Event.Fields = cloneEventFields(event.Event.Fields)
	}
	return clone
}

func cloneEventFields(fields map[string]string) map[string]string {
	if fields == nil {
		return nil
	}
	clone := make(map[string]string, len(fields))
	for key, value := range fields {
		clone[key] = value
	}
	return clone
}

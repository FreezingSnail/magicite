package tui

import (
	"reflect"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestModelSnapshotLifecycleAndGenerationRejection(t *testing.T) {
	clock := &modelClock{now: time.Date(2026, time.September, 7, 19, 0, 0, 0, time.UTC)}
	model := NewModel(ModelOptions{Now: clock.Now})
	model = updatedModel(t, model, SnapshotMsg{Snapshot: testSnapshot(3, 9, true, false)})
	state := model.State()
	if state.Connection != ConnectionOnline || state.Freshness != SnapshotFresh || state.Generation != 3 || state.Cursor != 9 || !state.HasSnapshot || !state.ChangedAt.Equal(clock.now) {
		t.Fatalf("fresh state = %#v", state)
	}

	clock.now = clock.now.Add(time.Minute)
	model = updatedModel(t, model, SnapshotMsg{Snapshot: testSnapshot(2, 11, false, true)})
	if got := model.State(); got.Generation != 3 || got.Cursor != 9 || !got.ChangedAt.Before(clock.now) {
		t.Fatalf("late snapshot changed state: %#v", got)
	}

	model = updatedModel(t, model, SnapshotMsg{Snapshot: testSnapshot(4, 12, false, true)})
	state = model.State()
	if state.Connection != ConnectionOnline || state.Freshness != SnapshotStale || state.Generation != 4 || state.Cursor != 12 || !state.ChangedAt.Equal(clock.now) {
		t.Fatalf("stale state = %#v", state)
	}
}

func TestModelOutagesRetainSnapshotAndSchemaMismatchTerminates(t *testing.T) {
	model := NewModel(ModelOptions{Now: func() time.Time { return time.Unix(1, 0) }})
	model = updatedModel(t, model, SnapshotMsg{Snapshot: testSnapshot(1, 1, true, false)})
	model = updatedModel(t, model, SnapshotMsg{Error: &transport.Error{Code: transport.ErrorUnavailable}})
	state := model.State()
	if state.Connection != ConnectionOffline || state.Freshness != SnapshotStale || !state.HasSnapshot || state.Generation != 1 || state.SchemaMismatch {
		t.Fatalf("outage state = %#v", state)
	}

	model = updatedModel(t, model, StreamNoticeMsg{Notice: transport.StreamNotice{Kind: transport.StreamNoticeSchemaMismatch, Cursor: 1}})
	state = model.State()
	if state.Connection != ConnectionSchemaMismatch || !state.SchemaMismatch || state.Generation != 1 {
		t.Fatalf("schema state = %#v", state)
	}
	model = updatedModel(t, model, SnapshotMsg{Snapshot: testSnapshot(2, 2, true, false)})
	if got := model.State(); got.Generation != 1 {
		t.Fatalf("terminal model accepted snapshot: %#v", got)
	}
}

func TestModelBoundsColdEventsAndCopiesState(t *testing.T) {
	clock := &modelClock{now: time.Unix(10, 0)}
	model := NewModel(ModelOptions{Now: clock.Now, ColdEventLimit: 2})
	model = updatedModel(t, model, SnapshotMsg{Snapshot: testSnapshot(1, 0, true, false)})
	for sequence := uint64(1); sequence <= 3; sequence++ {
		clock.now = clock.now.Add(time.Second)
		model = updatedModel(t, model, StreamEventMsg{Event: transport.Event{Seq: sequence, Fields: map[string]string{"sequence": string(rune('0' + sequence))}}})
	}
	state := model.State()
	if len(state.Events) != 2 || state.Events[0].Event.Seq != 2 || state.Events[1].Event.Seq != 3 || state.Cursor != 3 {
		t.Fatalf("bounded events = %#v", state)
	}
	state.Events[0].Event.Fields["sequence"] = "mutated"
	state.Snapshot.Seats[0].Name = "mutated"
	copy := model.State()
	if copy.Events[0].Event.Fields["sequence"] == "mutated" || copy.Snapshot.Seats[0].Name == "mutated" {
		t.Fatalf("State exposed model storage: %#v", copy)
	}
}

func TestModelSelectionClampsToContiguousCollections(t *testing.T) {
	model := NewModel()
	snapshot := testSnapshot(1, 0, true, false)
	snapshot.Seats = []transport.Seat{{Name: "one"}, {Name: "two"}}
	model = updatedModel(t, model, snapshot)
	model = updatedModel(t, model, SelectionMsg{Kind: SelectSeat, Index: 8})
	if got := model.State().Selection.Seat; got != 1 {
		t.Fatalf("seat selection = %d, want 1", got)
	}
	snapshot.Generation = 2
	snapshot.Seats = snapshot.Seats[:1]
	model = updatedModel(t, model, snapshot)
	if got := model.State().Selection.Seat; got != 0 {
		t.Fatalf("shrunk selection = %d, want 0", got)
	}
}

func TestModelViewDoesNotReadClockOrMutateState(t *testing.T) {
	calls := 0
	model := NewModel(ModelOptions{Now: func() time.Time { calls++; return time.Unix(1, 0) }})
	model = updatedModel(t, model, testSnapshot(1, 0, true, false))
	before := model.State()
	_ = model.View()
	_ = model.View()
	if calls != 1 || !reflect.DeepEqual(before, model.State()) {
		t.Fatalf("View read clock or mutated state: calls=%d", calls)
	}
}

func updatedModel(t *testing.T, model Model, message tea.Msg) Model {
	t.Helper()
	next, command := model.Update(message)
	if command != nil {
		t.Fatal("Update returned unexpected command")
	}
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("Update model = %T, want tui.Model", next)
	}
	return updated
}

func testSnapshot(generation, cursor uint64, fresh, stale bool) transport.Snapshot {
	return transport.Snapshot{
		ModelVersion: wire.Schema,
		Generation:   generation,
		Cursor:       cursor,
		Fresh:        fresh,
		Stale:        stale,
		Seats:        []transport.Seat{{Name: "ifrit"}},
		Sessions:     []transport.Session{},
		Beads:        []transport.Bead{},
	}
}

type modelClock struct{ now time.Time }

func (clock *modelClock) Now() time.Time { return clock.now }

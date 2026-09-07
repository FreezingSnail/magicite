package tui

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
	"github.com/FreezingSnail/magicite/internal/wire"
)

const defaultColdEventLimit = 64

// ModelOptions provides deterministic root-model seams. No coordinator is
// started here; callers deliver transport results through typed messages.
type ModelOptions struct {
	Now            func() time.Time
	ColdEventLimit int
}

// SnapshotMsg delivers one snapshot attempt. Error may be a transport.Error;
// schema errors terminate the model while all other failures preserve stale
// data for repair.
type SnapshotMsg struct {
	Snapshot transport.Snapshot
	Error    error
}

// StreamEventMsg delivers one ordered, non-authoritative stream event.
type StreamEventMsg struct{ Event transport.Event }

// StreamNoticeMsg delivers one typed stream lifecycle condition.
type StreamNoticeMsg struct{ Notice transport.StreamNotice }

// SelectionMsg changes one contiguous collection selection. Input handling is
// intentionally left to later composition.
type SelectionMsg struct {
	Kind  SelectionKind
	Index int
}

// SelectionKind identifies a snapshot collection selection.
type SelectionKind string

const (
	SelectRepository SelectionKind = "repository"
	SelectSeat       SelectionKind = "seat"
	SelectSession    SelectionKind = "session"
	SelectBead       SelectionKind = "bead"
)

// Model is the Bubble Tea root lifecycle model. Update is its only writer;
// View reads a precomputed string and performs no I/O, clock read, mutation,
// or map traversal.
type Model struct {
	state          ModelState
	now            func() time.Time
	coldEventLimit int
	view           string
}

// NewModel constructs a loading root model. The optional form keeps basic
// composition terse while permitting tests and runtimes to inject a clock.
func NewModel(options ...ModelOptions) Model {
	option := ModelOptions{}
	if len(options) != 0 {
		option = options[0]
	}
	if option.Now == nil {
		option.Now = time.Now
	}
	if option.ColdEventLimit <= 0 {
		option.ColdEventLimit = defaultColdEventLimit
	}
	return Model{state: initialModelState(), now: option.Now, coldEventLimit: option.ColdEventLimit}
}

// Init starts no commands. Refresh coordination is composed separately.
func (Model) Init() tea.Cmd { return nil }

// Update serializes all lifecycle transitions.
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if model.state.SchemaMismatch {
		return model, nil
	}
	switch message := message.(type) {
	case SnapshotMsg:
		model.applySnapshot(message)
	case transport.Snapshot:
		model.applySnapshot(SnapshotMsg{Snapshot: message})
	case StreamEventMsg:
		model.applyEvent(message.Event)
	case transport.Event:
		model.applyEvent(message)
	case StreamNoticeMsg:
		model.applyNotice(message.Notice)
	case transport.StreamNotice:
		model.applyNotice(message)
	case SelectionMsg:
		model.applySelection(message)
	}
	return model, nil
}

// View is deliberately inert until a renderer is composed around this model.
func (model Model) View() string { return model.view }

// State returns an independent, deterministic state copy.
func (model Model) State() ModelState { return cloneModelState(model.state) }

func (model *Model) applySnapshot(message SnapshotMsg) {
	if message.Error != nil {
		if isSchemaMismatch(message.Error) {
			model.terminal()
			return
		}
		model.stale(ConnectionOffline)
		return
	}
	snapshot := message.Snapshot
	if snapshot.ModelVersion != wire.Schema {
		model.terminal()
		return
	}
	if model.state.HasSnapshot && snapshot.Generation < model.state.Generation {
		return
	}
	model.state.Snapshot = cloneSnapshot(snapshot)
	model.state.HasSnapshot = true
	model.state.Generation = snapshot.Generation
	if snapshot.Cursor > model.state.Cursor {
		model.state.Cursor = snapshot.Cursor
	}
	model.state.Connection = ConnectionOnline
	if snapshot.Fresh && !snapshot.Stale {
		model.state.Freshness = SnapshotFresh
	} else {
		model.state.Freshness = SnapshotStale
	}
	model.normalizeSelection()
	model.changed()
}

func (model *Model) applyEvent(event transport.Event) {
	if event.Seq != 0 && event.Seq <= model.state.Cursor {
		return
	}
	model.state.Events = append(model.state.Events, ColdEvent{Event: cloneEvent(event), ReceivedAt: model.now()})
	if excess := len(model.state.Events) - model.coldEventLimit; excess > 0 {
		model.state.Events = append([]ColdEvent(nil), model.state.Events[excess:]...)
	}
	if event.Seq > model.state.Cursor {
		model.state.Cursor = event.Seq
	}
	model.changedAt(model.state.Events[len(model.state.Events)-1].ReceivedAt)
}

func (model *Model) applyNotice(notice transport.StreamNotice) {
	if notice.Cursor < model.state.Cursor {
		return
	}
	model.state.Notices = append(model.state.Notices, notice)
	if excess := len(model.state.Notices) - model.coldEventLimit; excess > 0 {
		model.state.Notices = append([]transport.StreamNotice(nil), model.state.Notices[excess:]...)
	}
	if notice.Cursor > model.state.Cursor {
		model.state.Cursor = notice.Cursor
	}
	switch notice.Kind {
	case transport.StreamNoticeSchemaMismatch:
		model.terminal()
	case transport.StreamNoticeUnavailable, transport.StreamNoticeEOF, transport.StreamNoticeGap, transport.StreamNoticeMiss:
		model.stale(ConnectionOffline)
	default:
		model.changed()
	}
}

func (model *Model) applySelection(message SelectionMsg) {
	index := message.Index
	switch message.Kind {
	case SelectRepository:
		model.state.Selection.Repository = index
	case SelectSeat:
		model.state.Selection.Seat = index
	case SelectSession:
		model.state.Selection.Session = index
	case SelectBead:
		model.state.Selection.Bead = index
	default:
		return
	}
	model.normalizeSelection()
	model.changed()
}

func (model *Model) stale(connection ConnectionState) {
	model.state.Connection = connection
	model.state.Freshness = SnapshotStale
	model.changed()
}

func (model *Model) terminal() {
	model.state.Connection = ConnectionSchemaMismatch
	model.state.Freshness = SnapshotStale
	model.state.SchemaMismatch = true
	model.changed()
}

func (model *Model) normalizeSelection() {
	model.state.Selection.Repository = boundedSelection(model.state.Selection.Repository, len(model.state.Snapshot.Repositories))
	model.state.Selection.Seat = boundedSelection(model.state.Selection.Seat, len(model.state.Snapshot.Seats))
	model.state.Selection.Session = boundedSelection(model.state.Selection.Session, len(model.state.Snapshot.Sessions))
	model.state.Selection.Bead = boundedSelection(model.state.Selection.Bead, len(model.state.Snapshot.Beads))
}

func boundedSelection(index, length int) int {
	if length == 0 {
		return -1
	}
	if index < 0 {
		return -1
	}
	if index >= length {
		return length - 1
	}
	return index
}

func (model *Model) changed() { model.changedAt(model.now()) }

func (model *Model) changedAt(at time.Time) { model.state.ChangedAt = at }

func isSchemaMismatch(err error) bool {
	var transportError *transport.Error
	return errors.As(err, &transportError) && transportError.Code == transport.ErrorSchemaMismatch
}

func cloneEvent(event transport.Event) transport.Event {
	event.Fields = cloneEventFields(event.Fields)
	return event
}

var _ tea.Model = Model{}

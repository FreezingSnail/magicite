package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/wire"
)

const seatPlaceholder = "—"

type seatRow struct {
	seat       wire.SeatResult
	session    wire.SessionResult
	hasSession bool
}

// SeatsView renders the daemon's configured seats and their matching sessions.
// Rows are joined and sorted during Snapshot so View only formats cached data.
type SeatsView struct {
	seats     []wire.SeatResult
	sessions  []wire.SessionResult
	rows      []seatRow
	worktree  bool
	selection Selection
}

// NewSeatsView constructs an empty Seats tab.
func NewSeatsView() *SeatsView {
	return &SeatsView{selection: Selection{Index: -1}}
}

func (*SeatsView) Title() string { return "Seats" }

// Update moves the stable seat selection with vim or arrow keys.
func (view *SeatsView) Update(message tea.Msg) (TabView, tea.Cmd) {
	key, ok := message.(tea.KeyMsg)
	if !ok {
		return view, nil
	}
	switch key.String() {
	case "j", "down":
		view.move(1)
	case "k", "up":
		view.move(-1)
	case "home":
		view.moveTo(0)
	case "end":
		view.moveTo(len(view.rows) - 1)
	}
	return view, nil
}

// Snapshot replaces cached daemon data and preserves selection by seat name.
func (view *SeatsView) Snapshot(snapshot wire.SnapshotResult) TabView {
	view.seats = append([]wire.SeatResult(nil), snapshot.Seats...)
	view.sessions = append([]wire.SessionResult(nil), snapshot.Sessions...)
	view.rows = joinSeatRows(view.seats, view.sessions)
	view.worktree = false
	for _, seat := range view.seats {
		if seat.Worktree != "" {
			view.worktree = true
			break
		}
	}
	view.selection = KeepSelection(seatRowKeys(view.rows), view.selection)
	return view
}

// Hints returns no action hints; seats are read-only.
func (*SeatsView) Hints() []Hint { return nil }

// View renders cached rows without I/O, mutation, or clock reads.
func (view *SeatsView) View(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if len(view.rows) == 0 {
		return Truncate(SanitizeText("No seats configured"), width)
	}

	rows := make([][]string, len(view.rows))
	for index, row := range view.rows {
		rows[index] = seatRowValues(row, view.worktree)
	}
	table := Table{
		Columns:  seatViewColumns(view.worktree),
		Rows:     rows,
		Selected: view.selection.Index,
	}
	return table.Render(width, height)
}

// SelectedSeat returns the current snapshot seat, if one is selected.
func (view *SeatsView) SelectedSeat() (wire.SeatResult, bool) {
	if view.selection.Index < 0 || view.selection.Index >= len(view.rows) {
		return wire.SeatResult{}, false
	}
	return view.rows[view.selection.Index].seat, true
}

func (view *SeatsView) move(delta int) {
	view.moveTo(view.selection.Index + delta)
}

func (view *SeatsView) moveTo(index int) {
	view.selection = KeepSelection(seatRowKeys(view.rows), Selection{Index: index})
}

func joinSeatRows(seats []wire.SeatResult, sessions []wire.SessionResult) []seatRow {
	rows := make([]seatRow, len(seats))
	for index, seat := range seats {
		rows[index].seat = seat
		for _, session := range sessions {
			if session.Seat == seat.Name {
				rows[index].session = session
				rows[index].hasSession = true
				break
			}
		}
	}
	sort.SliceStable(rows, func(left, right int) bool {
		if rows[left].seat.Role != rows[right].seat.Role {
			return rows[left].seat.Role < rows[right].seat.Role
		}
		return rows[left].seat.Name < rows[right].seat.Name
	})
	return rows
}

func seatRowKeys(rows []seatRow) []string {
	keys := make([]string, len(rows))
	for index := range rows {
		keys[index] = rows[index].seat.Name
	}
	return keys
}

func seatViewColumns(worktree bool) []Column {
	columns := []Column{
		{Title: "Seat", Width: 14, Min: 4},
		{Title: "Role", Width: 13, Min: 5},
		{Title: "Repository", Width: 16, Min: 4},
		{Title: "State", Width: 16, Min: 6},
		{Title: "Task", Width: 18, Min: 4},
		{Title: "Session", Width: 16, Min: 4},
		{Title: "Backend", Width: 10, Min: 4},
		{Title: "Model", Width: 18, Min: 4},
		{Title: "Uptime", Width: 10, Min: 5, Right: true},
	}
	if worktree {
		columns = append(columns, Column{Title: "Worktree", Width: 24, Min: 4, Weight: 1})
	}
	for index := range columns {
		columns[index].Title = SanitizeText(columns[index].Title)
	}
	return columns
}

func seatRowValues(row seatRow, worktree bool) []string {
	state, label := "idle", "available"
	if row.seat.Busy {
		state, label = "busy", "assigned"
	}
	values := []string{
		seatCell(row.seat.Name),
		seatCell(row.seat.Role),
		seatCell(row.seat.Repo),
		seatCell(state + " (" + label + ")"),
		seatCell(row.seat.Task),
	}
	if row.seat.Busy && row.hasSession {
		values = append(values,
			seatCell(row.session.Handle),
			seatCell(row.session.Backend),
			seatCell(row.session.Model),
			seatCell(dashboardUptime(row.session.UptimeSeconds)),
		)
	} else {
		values = append(values, seatPlaceholder, seatPlaceholder, seatPlaceholder, seatPlaceholder)
	}
	if worktree {
		values = append(values, seatCell(row.seat.Worktree))
	}
	return values
}

func seatCell(value string) string {
	if value == "" {
		return seatPlaceholder
	}
	return SanitizeText(value)
}

var _ TabView = (*SeatsView)(nil)

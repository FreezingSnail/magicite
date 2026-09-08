package tui

import (
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/wire"
)

const beadPlaceholder = "—"

type beadRow struct {
	bead wire.BeadResult
	key  string
}

// BeadsView renders every daemon-supplied bead. Snapshot and filter changes
// build the cached sorted and filtered indexes; View only formats those rows.
type BeadsView struct {
	beads     []wire.BeadResult
	rows      []beadRow
	filtered  []int
	query     string
	parsed    Query
	focused   bool
	selection Selection
	detail    *BeadDetail
}

// NewBeadsView constructs an empty Beads tab.
func NewBeadsView() *BeadsView {
	return &BeadsView{selection: Selection{Index: -1}}
}

func (*BeadsView) Title() string { return "Beads" }

// Update changes the filter or moves selection through Table.MoveBy.
func (view *BeadsView) Update(message tea.Msg) (TabView, tea.Cmd) {
	if view.detail != nil && view.detail.IsOpen() {
		return view, view.detail.Update(message)
	}
	key, ok := message.(tea.KeyMsg)
	if !ok {
		return view, nil
	}
	if view.focused {
		switch key.Type {
		case tea.KeyEsc:
			view.focused = false
			view.setQuery("")
		case tea.KeyBackspace, tea.KeyDelete:
			if len(view.query) > 0 {
				view.setQuery(view.query[:len(view.query)-1])
			}
		case tea.KeyEnter:
			view.focused = false
		default:
			if len(key.Runes) > 0 {
				view.setQuery(view.query + string(key.Runes))
			}
		}
		return view, nil
	}

	switch key.String() {
	case "/":
		view.focused = true
	case "enter":
		if bead, ok := view.SelectedBead(); ok {
			view.detail = NewBeadDetail(bead)
		}
	case "j", "down":
		view.move(1)
	case "k", "up":
		view.move(-1)
	case "home":
		view.moveTo(0)
	case "end":
		view.moveTo(len(view.filtered) - 1)
	}
	return view, nil
}

// Snapshot retains the immutable daemon slice and derives sorted presentation
// rows without altering daemon order.
func (view *BeadsView) Snapshot(snapshot wire.SnapshotResult) TabView {
	view.beads = snapshot.Beads
	view.rebuild()
	if view.detail != nil && view.detail.IsOpen() {
		for _, bead := range view.beads {
			if bead.ID == view.detail.beadID {
				view.detail.Refresh(bead)
				return view
			}
		}
		view.detail.Refresh(wire.BeadResult{})
	}
	return view
}

// Hints exposes only keys usable by the currently focused Beads pane.
func (view *BeadsView) Hints() []Hint {
	if view.detail != nil && view.detail.IsOpen() {
		return []Hint{{Key: "j/k", Label: "scroll", Enabled: true}, {Key: "Esc", Label: "close", Enabled: true}}
	}
	return []Hint{{Key: "Enter", Label: "detail", Enabled: len(view.filtered) > 0}, {Key: "/", Label: "filter", Enabled: true}, {Key: "j/k", Label: "select", Enabled: true}}
}

// View renders the open detail or cached table data without I/O.
func (view *BeadsView) View(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if view.detail != nil && view.detail.IsOpen() {
		return view.detail.View(width, height)
	}
	if len(view.rows) == 0 {
		return Truncate("No beads in snapshot", width)
	}

	lines := []string{Truncate("Beads: "+strconv.Itoa(len(view.filtered))+"/"+strconv.Itoa(len(view.rows)), width)}
	if view.focused || view.query != "" {
		lines = append(lines, Truncate("Filter: "+view.query, width))
	}
	if len(view.parsed.Unknown) > 0 {
		lines = append(lines, Truncate("Filter error: unknown field "+strings.Join(view.parsed.Unknown, ", "), width))
	}
	if len(view.filtered) == 0 {
		lines = append(lines, Truncate("No beads match filter", width))
		return strings.Join(lines[:min(len(lines), height)], "\n")
	}
	if len(lines) >= height {
		return strings.Join(lines[:height], "\n")
	}

	rows := make([][]string, len(view.filtered))
	for index, rowIndex := range view.filtered {
		rows[index] = beadRowValues(view.rows[rowIndex].bead)
	}
	table := Table{Columns: beadViewColumns(), Rows: rows, Selected: view.selection.Index}
	lines = append(lines, table.Render(width, height-len(lines)))
	return strings.Join(lines, "\n")
}

// SelectedBead returns an independent selected daemon bead, if the filtered row exists.
func (view *BeadsView) SelectedBead() (wire.BeadResult, bool) {
	if view.selection.Index < 0 || view.selection.Index >= len(view.filtered) {
		return wire.BeadResult{}, false
	}
	return cloneBead(view.rows[view.filtered[view.selection.Index]].bead), true
}

func cloneBead(bead wire.BeadResult) wire.BeadResult {
	bead.Description = cloneString(bead.Description)
	bead.Design = cloneString(bead.Design)
	bead.AcceptanceCriteria = cloneString(bead.AcceptanceCriteria)
	bead.Assignee = cloneString(bead.Assignee)
	bead.Owner = cloneString(bead.Owner)
	bead.Parent = cloneString(bead.Parent)
	bead.CreatedAt = cloneTime(bead.CreatedAt)
	bead.CreatedBy = cloneString(bead.CreatedBy)
	bead.UpdatedAt = cloneTime(bead.UpdatedAt)
	bead.StartedAt = cloneTime(bead.StartedAt)
	bead.ClosedAt = cloneTime(bead.ClosedAt)
	bead.DeferredUntil = cloneTime(bead.DeferredUntil)
	bead.CloseReason = cloneString(bead.CloseReason)
	bead.Dependencies = append([]wire.DependencyResult(nil), bead.Dependencies...)
	bead.Labels = append([]string(nil), bead.Labels...)
	bead.Dispatch.Reason = cloneString(bead.Dispatch.Reason)
	bead.Review.Reason = cloneString(bead.Review.Reason)
	return bead
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (view *BeadsView) setQuery(query string) {
	if view.query == query {
		return
	}
	view.query = query
	view.rebuildFilter()
}

func (view *BeadsView) rebuild() {
	view.rows = make([]beadRow, len(view.beads))
	for index, bead := range view.beads {
		view.rows[index] = beadRow{bead: bead, key: bead.ID}
	}
	sort.SliceStable(view.rows, func(left, right int) bool {
		if view.rows[left].bead.Repo != view.rows[right].bead.Repo {
			return view.rows[left].bead.Repo < view.rows[right].bead.Repo
		}
		return view.rows[left].bead.ID < view.rows[right].bead.ID
	})
	view.rebuildFilter()
}

func (view *BeadsView) rebuildFilter() {
	view.parsed = ParseQuery(view.query)
	view.filtered = view.filtered[:0]
	for index, row := range view.rows {
		if beadMatches(view.parsed, row.bead) {
			view.filtered = append(view.filtered, index)
		}
	}
	view.selection = KeepSelection(view.filteredKeys(), view.selection)
}

func (view *BeadsView) filteredKeys() []string {
	keys := make([]string, len(view.filtered))
	for index, rowIndex := range view.filtered {
		keys[index] = view.rows[rowIndex].key
	}
	return keys
}

func (view *BeadsView) move(delta int) {
	view.moveTo(view.selection.Index + delta)
}

func (view *BeadsView) moveTo(index int) {
	table := Table{Rows: make([][]string, len(view.filtered)), Selected: view.selection.Index}
	table.MoveBy(index - table.Selected)
	view.selection = KeepSelection(view.filteredKeys(), Selection{Index: table.Selected})
}

func beadMatches(query Query, bead wire.BeadResult) bool {
	if len(query.Unknown) > 0 {
		return false
	}
	for _, term := range query.Terms {
		switch term.Field {
		case "repo":
			if bead.Repo != term.Value {
				return false
			}
		case "state":
			if bead.Status != term.Value {
				return false
			}
		case "type":
			if bead.IssueType != term.Value {
				return false
			}
		case "staged":
			if strconv.FormatBool(bead.Staged) != term.Value {
				return false
			}
		case "label":
			matched := false
			for _, label := range bead.Labels {
				if label == term.Value {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		}
	}
	for _, free := range query.Free {
		free = strings.ToLower(free)
		if !strings.Contains(strings.ToLower(bead.ID), free) && !strings.Contains(strings.ToLower(bead.Title), free) {
			return false
		}
	}
	return true
}

func beadViewColumns() []Column {
	return []Column{
		{Title: "ID", Width: 18, Min: 3},
		{Title: "Type", Width: 12, Min: 4},
		{Title: "State", Width: 12, Min: 4},
		{Title: "Priority", Width: 8, Min: 4, Right: true},
		{Title: "Staged", Width: 8, Min: 4},
		{Title: "Title", Width: 36, Min: 6, Weight: 1},
		{Title: "Repository", Width: 18, Min: 4},
	}
}

func beadRowValues(bead wire.BeadResult) []string {
	return []string{
		beadCell(bead.ID),
		beadCell(bead.IssueType),
		beadCell(bead.Status),
		SanitizeText(strconv.Itoa(bead.Priority)),
		SanitizeText(strconv.FormatBool(bead.Staged)),
		beadCell(bead.Title),
		beadCell(bead.Repo),
	}
}

func beadCell(value string) string {
	if value == "" {
		return beadPlaceholder
	}
	return SanitizeText(value)
}

var _ TabView = (*BeadsView)(nil)

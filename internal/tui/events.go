package tui

import (
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/wire"
)

// EventMarkKind identifies a runtime stream notice retained in event history.
type EventMarkKind string

const (
	EventMarkReconnect   EventMarkKind = "reconnect"
	EventMarkStreamGap   EventMarkKind = "stream-gap"
	EventMarkMissedRange EventMarkKind = "missed-range"
)

// EventMark records an ordered stream notice. From and To are inclusive.
type EventMark struct {
	Kind   EventMarkKind
	From   uint64
	To     uint64
	Detail string
}

type eventRow struct {
	key    string
	event  wire.Event
	mark   *EventMark
	fields []eventField
	values []string
}

type eventField struct {
	key   string
	value string
}

// EventsView retains bounded, session-local event history. It never owns or
// changes snapshot-derived daemon state.
type EventsView struct {
	Capacity int
	Dropped  uint64
	Follow   bool
	Query    Query

	ring       []eventRow
	start      int
	count      int
	markNumber uint64
	selection  Selection
	offset     int
	filter     string
	filtering  bool
	expanded   bool
}

var eventColumns = []Column{
	{Title: "Time", Width: 20, Min: 4},
	{Title: "Level", Width: 7, Min: 3},
	{Title: "Kind", Width: 13, Min: 4},
	{Title: "Repository", Width: 16, Min: 4},
	{Title: "Task", Width: 16, Min: 4},
	{Title: "Seat", Width: 12, Min: 4},
	{Title: "Summary", Min: 6, Weight: 1},
}

// NewEventsView constructs an Events tab with exactly capacity history slots.
func NewEventsView(capacity int) *EventsView {
	if capacity < 0 {
		capacity = 0
	}
	return &EventsView{Capacity: capacity, Follow: true, ring: make([]eventRow, capacity)}
}

func (*EventsView) Title() string { return "Events" }

// Append retains event in arrival order, evicting the oldest retained row when
// full. Event fields are copied and sorted before later rendering.
func (view *EventsView) Append(event wire.Event) {
	view.append(eventRow{
		key:    "event:" + strconv.FormatUint(event.Seq, 10),
		event:  cloneEventsViewEvent(event),
		fields: sortedEventFields(event.Fields),
		values: eventValues(event),
	})
}

// Mark retains a reconnect, stream-gap, or missed-range notice in event order.
func (view *EventsView) Mark(mark EventMark) {
	view.markNumber++
	mark.Detail = SanitizeText(mark.Detail)
	view.append(eventRow{
		key:  "mark:" + string(mark.Kind) + ":" + strconv.FormatUint(mark.From, 10) + ":" + strconv.FormatUint(mark.To, 10) + ":" + strconv.FormatUint(view.markNumber, 10),
		mark: &mark,
		fields: []eventField{
			{key: "detail", value: mark.Detail},
			{key: "from", value: strconv.FormatUint(mark.From, 10)},
			{key: "notice", value: string(mark.Kind)},
			{key: "to", value: strconv.FormatUint(mark.To, 10)},
		},
		values: []string{"", "notice", string(mark.Kind), "", "", "", markSummary(mark)},
	})
}

func (view *EventsView) append(row eventRow) {
	if view.Capacity == 0 {
		view.Dropped++
		return
	}
	if view.count == view.Capacity {
		view.ring[view.start] = row
		view.start = (view.start + 1) % view.Capacity
		view.Dropped++
	} else {
		view.ring[(view.start+view.count)%view.Capacity] = row
		view.count++
	}
	view.resolveSelection()
}

// SetFilter replaces the event filter. Supported exact predicates are level,
// repo, task, and seat; free terms search kind and summary.
func (view *EventsView) SetFilter(raw string) {
	view.filter = raw
	view.Query = parseEventQuery(raw)
	view.resolveSelection()
}

// Update handles filtering, selection, follow, and deterministic field detail.
func (view *EventsView) Update(message tea.Msg) (TabView, tea.Cmd) {
	key, ok := message.(tea.KeyMsg)
	if !ok {
		return view, nil
	}
	if view.filtering {
		view.updateFilter(key)
		return view, nil
	}
	switch key.String() {
	case "/":
		view.filtering = true
	case "f":
		view.Follow = !view.Follow
		if view.Follow {
			view.selectNewest()
		}
	case "enter":
		if view.selection.Index >= 0 {
			view.expanded = !view.expanded
		}
	case "j", "down":
		view.move(1)
	case "k", "up":
		view.move(-1)
	case "home":
		view.moveTo(0)
	case "end":
		view.moveTo(len(view.visibleRows()) - 1)
	case "pgdown":
		view.move(10)
	case "pgup":
		view.move(-10)
	}
	return view, nil
}

func (view *EventsView) updateFilter(key tea.KeyMsg) {
	switch key.String() {
	case "esc":
		view.filtering = false
	case "enter":
		view.filtering = false
	case "backspace", "ctrl+h":
		if len(view.filter) > 0 {
			view.filter = string([]rune(view.filter)[:len([]rune(view.filter))-1])
			view.SetFilter(view.filter)
		}
	default:
		if key.Type == tea.KeyRunes {
			view.SetFilter(view.filter + string(key.Runes))
		}
	}
}

// Snapshot deliberately ignores daemon state: events are session-local history.
func (view *EventsView) Snapshot(wire.SnapshotResult) TabView { return view }

// Hints returns only commands implemented by this tab.
func (view *EventsView) Hints() []Hint {
	if view.filtering {
		return []Hint{{Key: "enter", Label: "apply", Enabled: true}, {Key: "esc", Label: "cancel", Enabled: true}}
	}
	return []Hint{{Key: "/", Label: "filter", Enabled: true}, {Key: "f", Label: "follow", Enabled: true}, {Key: "enter", Label: "fields", Enabled: view.selection.Index >= 0}}
}

// View renders only retained rows and data already captured from events.
func (view *EventsView) View(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	rows := view.visibleRows()
	view.selection = KeepSelection(rowKeys(rows), view.selection)
	header := "Events  follow:"
	if view.Follow {
		header += "on"
	} else {
		header += "off"
	}
	header += "  dropped-oldest:" + strconv.FormatUint(view.Dropped, 10)
	if view.filtering || view.filter != "" {
		header += "  filter:" + SanitizeText(view.filter)
	}
	lines := []string{Truncate(header, width)}
	if height == 1 {
		return strings.Join(lines, "\n")
	}

	detail := view.detail(rows)
	tableHeight := height - 1 - len(detail)
	if tableHeight > 0 {
		tableRows := make([][]string, len(rows))
		for index := range rows {
			tableRows[index] = rows[index].values
		}
		table := Table{Columns: eventColumns, Rows: tableRows, Selected: view.selection.Index, Offset: view.offset}
		lines = append(lines, table.Render(width, tableHeight))
		view.offset = table.Offset
	}
	for _, line := range detail {
		if len(lines) == height {
			break
		}
		lines = append(lines, Truncate(line, width))
	}
	return strings.Join(lines, "\n")
}

func (view *EventsView) detail(rows []eventRow) []string {
	if !view.expanded || view.selection.Index < 0 || view.selection.Index >= len(rows) {
		return nil
	}
	row := rows[view.selection.Index]
	if len(row.fields) == 0 {
		return []string{"Fields: none"}
	}
	lines := make([]string, 1, len(row.fields)+1)
	lines[0] = "Fields:"
	for _, field := range row.fields {
		lines = append(lines, SanitizeText(field.key)+": "+SanitizeText(field.value))
	}
	return lines
}

func (view *EventsView) move(delta int) {
	view.moveTo(view.selection.Index + delta)
}

func (view *EventsView) moveTo(index int) {
	rows := view.visibleRows()
	if len(rows) == 0 {
		return
	}
	view.Follow = false
	view.selection = KeepSelection(rowKeys(rows), Selection{Index: index})
}

func (view *EventsView) selectNewest() {
	rows := view.visibleRows()
	if len(rows) == 0 {
		view.selection = Selection{Index: -1}
		return
	}
	view.selection = Selection{Key: rows[len(rows)-1].key, Index: len(rows) - 1}
}

func (view *EventsView) resolveSelection() {
	if view.Follow {
		view.selectNewest()
		return
	}
	view.selection = KeepSelection(rowKeys(view.visibleRows()), view.selection)
}

func (view *EventsView) visibleRows() []eventRow {
	rows := make([]eventRow, 0, view.count)
	for index := 0; index < view.count; index++ {
		row := view.ring[(view.start+index)%view.Capacity]
		if row.mark != nil || eventQueryMatches(view.Query, row) {
			rows = append(rows, row)
		}
	}
	return rows
}

func rowKeys(rows []eventRow) []string {
	keys := make([]string, len(rows))
	for index := range rows {
		keys[index] = rows[index].key
	}
	return keys
}

func parseEventQuery(raw string) Query {
	var query Query
	for _, token := range strings.Fields(raw) {
		field, value, hasField := strings.Cut(token, ":")
		if !hasField {
			query.Free = append(query.Free, token)
			continue
		}
		switch field {
		case "level", "repo", "task", "seat":
			query.Terms = append(query.Terms, FieldTerm{Field: field, Value: value})
		default:
			query.Unknown = append(query.Unknown, field)
		}
	}
	return query
}

func eventValues(event wire.Event) []string {
	time := ""
	if !event.Time.IsZero() {
		time = event.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	return []string{time, SanitizeText(event.Level), SanitizeText(string(event.Kind)), SanitizeText(event.Repo), SanitizeText(event.Task), SanitizeText(event.Seat), SanitizeText(eventSummary(event))}
}

func eventQueryMatches(query Query, row eventRow) bool {
	if len(query.Unknown) != 0 {
		return false
	}
	fields := map[string]string{
		"level": row.event.Level,
		"repo":  row.event.Repo,
		"task":  row.event.Task,
		"seat":  row.event.Seat,
	}
	for _, term := range query.Terms {
		if fields[term.Field] != term.Value {
			return false
		}
	}
	for _, term := range query.Free {
		term = strings.ToLower(term)
		if !strings.Contains(strings.ToLower(string(row.event.Kind)), term) && !strings.Contains(strings.ToLower(eventSummary(row.event)), term) {
			return false
		}
	}
	return true
}

func eventSummary(event wire.Event) string {
	for _, key := range []string{"summary", "message", "detail"} {
		if value, ok := event.Fields[key]; ok {
			return value
		}
	}
	return ""
}

func markSummary(mark EventMark) string {
	summary := string(mark.Kind) + " " + strconv.FormatUint(mark.From, 10) + "-" + strconv.FormatUint(mark.To, 10)
	if mark.Detail != "" {
		summary += ": " + mark.Detail
	}
	return summary
}

func sortedEventFields(fields map[string]string) []eventField {
	result := make([]eventField, 0, len(fields))
	for key, value := range fields {
		result = append(result, eventField{key: key, value: SanitizeText(value)})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].key < result[right].key })
	return result
}

func cloneEventsViewEvent(event wire.Event) wire.Event {
	fields := event.Fields
	if fields == nil {
		return event
	}
	event.Fields = make(map[string]string, len(fields))
	for key, value := range fields {
		event.Fields[key] = value
	}
	return event
}

var _ TabView = (*EventsView)(nil)

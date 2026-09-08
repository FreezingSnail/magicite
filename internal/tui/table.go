package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Column describes one table column. Width is its preferred width, Weight
// receives remaining width, Min is its narrow-layout width, and Right aligns
// cell contents against the right edge.
type Column struct {
	Title  string
	Width  int
	Weight int
	Min    int
	Right  bool
}

// Table renders pre-sanitized rows in a deterministic, selectable viewport.
type Table struct {
	Columns  []Column
	Rows     [][]string
	Selected int
	Offset   int
}

const tableMarkerWidth = 2

// Layout returns widths for the leading columns that fit within width.
func (table Table) Layout(width int) []int {
	if width <= 0 || len(table.Columns) == 0 {
		return nil
	}

	count := 1
	used := columnMinimum(table.Columns[0])
	if used > width {
		used = width
	}
	for count < len(table.Columns) {
		minimum := columnMinimum(table.Columns[count])
		if used+1+minimum > width {
			break
		}
		used += 1 + minimum
		count++
	}

	widths := make([]int, count)
	for index := range widths {
		widths[index] = columnMinimum(table.Columns[index])
	}
	if widths[0] > width {
		widths[0] = width
	}
	remaining := width - used

	for index := range widths {
		preferred := table.Columns[index].Width
		if preferred <= widths[index] || remaining == 0 {
			continue
		}
		additional := min(preferred-widths[index], remaining)
		widths[index] += additional
		remaining -= additional
	}
	if remaining == 0 {
		return widths
	}

	totalWeight := 0
	for index := range widths {
		if table.Columns[index].Weight > 0 {
			totalWeight += table.Columns[index].Weight
		}
	}
	if totalWeight == 0 {
		widths[len(widths)-1] += remaining
		return widths
	}
	for remaining > 0 {
		for index := range widths {
			for units := 0; units < table.Columns[index].Weight && remaining > 0; units++ {
				widths[index]++
				remaining--
			}
		}
	}
	return widths
}

func columnMinimum(column Column) int {
	if column.Min > 0 {
		return column.Min
	}
	return 1
}

// MoveBy moves the selection by delta without leaving the row range.
func (table *Table) MoveBy(delta int) {
	if len(table.Rows) == 0 {
		table.Selected = 0
		table.Offset = 0
		return
	}
	table.Selected = clampSelection(table.Selected, len(table.Rows))
	if delta > 0 && delta > len(table.Rows)-1-table.Selected {
		table.Selected = len(table.Rows) - 1
	} else if delta < 0 && delta < -table.Selected {
		table.Selected = 0
	} else {
		table.Selected += delta
	}
}

// Render returns a fixed-width text table and keeps the selected row visible.
func (table *Table) Render(width, height int) string {
	if width <= 0 || height <= 0 || len(table.Columns) == 0 {
		return ""
	}

	columnWidth := max(1, width-tableMarkerWidth)
	layout := table.Layout(columnWidth)
	if len(layout) == 0 {
		return ""
	}
	lines := []string{table.renderLine("  ", nil, layout, width)}
	if height == 1 || len(table.Rows) == 0 {
		return strings.Join(lines, "\n")
	}

	table.Selected = clampSelection(table.Selected, len(table.Rows))
	visible := height - 1
	maxOffset := max(0, len(table.Rows)-visible)
	table.Offset = min(max(table.Offset, 0), maxOffset)
	if table.Selected < table.Offset {
		table.Offset = table.Selected
	}
	if table.Selected >= table.Offset+visible {
		table.Offset = table.Selected - visible + 1
	}

	end := min(len(table.Rows), table.Offset+visible)
	for index := table.Offset; index < end; index++ {
		marker := "  "
		if index == table.Selected {
			marker = "> "
		}
		lines = append(lines, table.renderLine(marker, table.Rows[index], layout, width))
	}
	return strings.Join(lines, "\n")
}

func (table Table) renderLine(marker string, row []string, widths []int, width int) string {
	cells := make([]string, len(widths))
	for index, cellWidth := range widths {
		value := table.Columns[index].Title
		if row != nil && index < len(row) {
			value = row[index]
		}
		value = Truncate(value, cellWidth)
		padding := max(0, cellWidth-lipgloss.Width(value))
		if table.Columns[index].Right {
			cells[index] = strings.Repeat(" ", padding) + value
		} else {
			cells[index] = value + strings.Repeat(" ", padding)
		}
	}
	return Truncate(marker+strings.Join(cells, " "), width)
}

func clampSelection(selection, rows int) int {
	if rows == 0 {
		return 0
	}
	return min(max(selection, 0), rows-1)
}

// FieldTerm is one exact field predicate in a Query.
type FieldTerm struct {
	Field string
	Value string
}

// Query is a parsed inventory filter.
type Query struct {
	Terms   []FieldTerm
	Free    []string
	Unknown []string
}

var queryFields = map[string]struct{}{
	"repo":   {},
	"state":  {},
	"type":   {},
	"label":  {},
	"staged": {},
}

// Parse turns raw filter text into exact field predicates and free-text terms.
func Parse(raw string) Query {
	var query Query
	for _, token := range strings.Fields(raw) {
		field, value, hasField := strings.Cut(token, ":")
		if !hasField {
			query.Free = append(query.Free, token)
			continue
		}
		if _, known := queryFields[field]; !known {
			query.Unknown = append(query.Unknown, field)
			continue
		}
		query.Terms = append(query.Terms, FieldTerm{Field: field, Value: value})
	}
	return query
}

// ParseQuery is an explicit alias for Parse.
func ParseQuery(raw string) Query {
	return Parse(raw)
}

// Match reports whether fields and any free-text fields satisfy every predicate.
func (query Query) Match(fields map[string]string, free ...string) bool {
	if len(query.Unknown) > 0 {
		return false
	}
	for _, term := range query.Terms {
		if fields[term.Field] != term.Value {
			return false
		}
	}
	for _, term := range query.Free {
		term = strings.ToLower(term)
		found := false
		for _, value := range free {
			if strings.Contains(strings.ToLower(value), term) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

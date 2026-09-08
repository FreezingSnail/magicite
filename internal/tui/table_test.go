package tui

import (
	"reflect"
	"strings"
	"testing"
)

func TestTableLayoutWeightsMinimumsAndDropping(t *testing.T) {
	table := Table{Columns: []Column{
		{Title: "ID", Width: 4, Min: 2},
		{Title: "State", Min: 3, Weight: 1},
		{Title: "Title", Min: 4, Weight: 2},
	}}
	if got, want := table.Layout(20), []int{4, 6, 8}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Layout(20) = %#v, want %#v", got, want)
	}
	if got, want := table.Layout(9), []int{4, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Layout(9) = %#v, want %#v", got, want)
	}
	if got, want := table.Layout(1), []int{1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Layout(1) = %#v, want %#v", got, want)
	}
}

func TestTableRenderScrollsAndUsesVisibleMarker(t *testing.T) {
	table := Table{
		Columns: []Column{{Title: "Name", Min: 4, Weight: 1}, {Title: "N", Width: 3, Right: true}},
		Rows:    [][]string{{"alpha", "1"}, {"beta", "20"}, {"gamma", "300"}},
		Selected: 2,
	}
	want := "  Name      N\n  beta     20\n> gamma   300"
	if got := table.Render(13, 3); got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
	if table.Offset != 1 {
		t.Fatalf("Offset = %d, want 1", table.Offset)
	}
}

func TestTableMoveByClampsAndRenderSanitizes(t *testing.T) {
	table := Table{Columns: []Column{{Title: "Value", Weight: 1}}, Rows: [][]string{{"\x1b[31mred"}, {"blue"}}}
	table.MoveBy(100)
	table.MoveBy(-100)
	if table.Selected != 0 {
		t.Fatalf("Selected = %d, want 0", table.Selected)
	}
	got := table.Render(8, 2)
	if strings.Contains(got, "\x1b") || !strings.Contains(got, "�") {
		t.Fatalf("Render() = %q, want sanitized control text", got)
	}
}

func TestQueryParsesAndMatchesExactFields(t *testing.T) {
	query := Parse("repo:magicite state:open title label:ui staged:true nope:value")
	if got, want := query.Unknown, []string{"nope"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Unknown = %#v, want %#v", got, want)
	}
	if query.Match(map[string]string{"repo": "magicite", "state": "open", "label": "ui", "staged": "true"}, "a title") {
		t.Fatal("unknown field matched")
	}

	query = Parse("repo:magicite state:open title")
	fields := map[string]string{"repo": "magicite", "state": "open"}
	if !query.Match(fields, "A TITLE here") {
		t.Fatal("matching query did not match")
	}
	if Parse("state:Open").Match(fields) {
		t.Fatal("field matching ignored case")
	}
	if query.Match(fields, "other") {
		t.Fatal("free text matched absent text")
	}
}

package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestRenderDashboardEventsBoundsAndPreservesOrder(t *testing.T) {
	input := DashboardEventsInput{
		Width: 80,
		Events: []DashboardEvent{
			{Kind: "one", Detail: "first"},
			{Kind: "two", Detail: "second"},
			{Kind: "three", Detail: "third"},
			{Kind: "four", Detail: "fourth"},
			{Kind: "five", Detail: "fifth"},
			{Kind: "six", Detail: "sixth"},
		},
		Notices: []DashboardNotice{
			{Kind: DashboardNoticeReconnect, Detail: "retrying"},
			{Kind: DashboardNoticeGap, Detail: "cursor 12 to 14"},
			{Kind: DashboardNoticeMiss, Detail: "events unavailable"},
			{Kind: DashboardNoticeError, Detail: "socket closed"},
		},
	}

	got := RenderDashboardEvents(input)
	want := strings.Join([]string{
		"Recent events",
		"• two: second",
		"• three: third",
		"• four: fourth",
		"• five: fifth",
		"• six: sixth",
		"! reconnect: retrying",
		"! gap: cursor 12 to 14",
		"! miss: events unavailable",
		"! error: socket closed",
	}, "\n")
	if got != want {
		t.Fatalf("RenderDashboardEvents() = %q, want %q", got, want)
	}
	if strings.Contains(got, "one") {
		t.Fatal("RenderDashboardEvents() retained event beyond cap")
	}
}

func TestRenderDashboardEventsBoundsDetailAndWidth(t *testing.T) {
	got := RenderDashboardEvents(DashboardEventsInput{
		Width: 12,
		Events: []DashboardEvent{{
			Kind:   "complete",
			Detail: "implementation has an intentionally long detail",
		}},
	})
	for _, line := range strings.Split(got, "\n") {
		if length := len([]rune(line)); length > 12 {
			t.Fatalf("line width = %d, want <= 12: %q", length, line)
		}
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("RenderDashboardEvents() = %q, want truncated detail", got)
	}
	if got := RenderDashboardEvents(DashboardEventsInput{}); got != "" {
		t.Fatalf("RenderDashboardEvents(zero width) = %q, want empty", got)
	}
}

func TestRenderDashboardEventsNoColorTextEquivalent(t *testing.T) {
	input := DashboardEventsInput{
		Width:   80,
		Events:  []DashboardEvent{{Kind: "complete", Detail: "magicite-123", Style: EventStyleSuccess}},
		Notices: []DashboardNotice{{Kind: DashboardNoticeError, Detail: "connection lost"}},
	}
	plain := RenderDashboardEvents(input)

	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)
	input.Color = true
	styled := ansiEscape.ReplaceAllString(RenderDashboardEvents(input), "")
	if styled != plain {
		t.Fatalf("unstyled styled output = %q, want %q", styled, plain)
	}
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

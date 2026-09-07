package tui

import "strings"

const maxDashboardEvents = 5

// DashboardNoticeKind identifies a transport condition independently of its
// diagnostic detail.
type DashboardNoticeKind string

const (
	DashboardNoticeReconnect DashboardNoticeKind = "reconnect"
	DashboardNoticeGap       DashboardNoticeKind = "gap"
	DashboardNoticeMiss      DashboardNoticeKind = "miss"
	DashboardNoticeError     DashboardNoticeKind = "error"
)

// DashboardEvent is one already-selected significant event. Events are
// rendered in their supplied order; callers retain ownership of the slice.
type DashboardEvent struct {
	Kind   string
	Detail string
	Style  EventStyle
}

// DashboardNotice is one ordered transport notice. Detail is optional.
type DashboardNotice struct {
	Kind   DashboardNoticeKind
	Detail string
}

// DashboardEventsInput contains only renderer input. It intentionally carries
// no snapshot or root-model state, so events cannot change snapshot truth.
type DashboardEventsInput struct {
	Events  []DashboardEvent
	Notices []DashboardNotice
	Width   int
	Color   bool
}

// RenderDashboardEvents renders the latest significant events and ordered
// transport notices. Lines are bounded by Width and semantic markers preserve
// their meaning when Color is false.
func RenderDashboardEvents(input DashboardEventsInput) string {
	if input.Width <= 0 {
		return ""
	}

	lines := []string{EventStyleMuted.render(fit("Recent events", input.Width), input.Color)}
	events := input.Events
	if len(events) > maxDashboardEvents {
		events = events[len(events)-maxDashboardEvents:]
	}
	for _, event := range events {
		line := eventLine(event)
		lines = append(lines, event.Style.render(fit(line, input.Width), input.Color))
	}
	if len(events) == 0 && len(input.Notices) == 0 {
		lines = append(lines, EventStyleMuted.render(fit("  no recent events", input.Width), input.Color))
	}
	for _, notice := range input.Notices {
		line := noticeLine(notice)
		lines = append(lines, noticeStyle(notice.Kind).render(fit(line, input.Width), input.Color))
	}
	return strings.Join(lines, "\n")
}

func eventLine(event DashboardEvent) string {
	if event.Kind == "" {
		return "• " + event.Detail
	}
	if event.Detail == "" {
		return "• " + event.Kind
	}
	return "• " + event.Kind + ": " + event.Detail
}

func noticeLine(notice DashboardNotice) string {
	line := "! " + string(notice.Kind)
	if notice.Detail != "" {
		line += ": " + notice.Detail
	}
	return line
}

func noticeStyle(kind DashboardNoticeKind) EventStyle {
	switch kind {
	case DashboardNoticeReconnect:
		return EventStyleMuted
	case DashboardNoticeGap, DashboardNoticeMiss:
		return EventStyleWarning
	case DashboardNoticeError:
		return EventStyleError
	default:
		return EventStyleWarning
	}
}

func fit(text string, width int) string {
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

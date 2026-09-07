package tui

import "github.com/charmbracelet/lipgloss"

// EventStyle identifies the semantic presentation of a dashboard event line.
// Its text marker remains meaningful when color is unavailable.
type EventStyle uint8

const (
	EventStyleNeutral EventStyle = iota
	EventStyleSuccess
	EventStyleWarning
	EventStyleError
	EventStyleMuted
)

func (style EventStyle) render(text string, color bool) string {
	if !color {
		return text
	}
	return style.lipgloss().Render(text)
}

func (style EventStyle) lipgloss() lipgloss.Style {
	switch style {
	case EventStyleSuccess:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	case EventStyleWarning:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)
	case EventStyleError:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("8")).Bold(true)
	case EventStyleMuted:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	}
}

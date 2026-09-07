package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
)

// DashboardSessionsInput is the active daemon session data shown on Dashboard.
type DashboardSessionsInput struct {
	Sessions []transport.Session
}

// RenderDashboardSessions renders active sessions without reading UI state.
func RenderDashboardSessions(input DashboardSessionsInput, width int) string {
	sessions := append([]transport.Session(nil), input.Sessions...)
	sort.SliceStable(sessions, func(left, right int) bool {
		if sessions[left].UptimeSeconds != sessions[right].UptimeSeconds {
			return sessions[left].UptimeSeconds > sessions[right].UptimeSeconds
		}
		return sessions[left].Handle < sessions[right].Handle
	})

	lines := make([]string, 0, len(sessions)+1)
	lines = append(lines, "Sessions")
	for _, session := range sessions {
		backend := dashboardValue(session.Backend, "unknown")
		model := dashboardValue(session.Model, "unknown")
		lines = append(lines, truncateDashboardLine(fmt.Sprintf("%s [%s] %s/%s %s", dashboardValue(session.Handle, "unnamed"), dashboardValue(session.Phase, "unknown"), backend, model, dashboardUptime(session.UptimeSeconds)), width))
	}
	return strings.Join(lines, "\n")
}

func dashboardUptime(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	seconds %= 60
	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

package tui

import (
	"fmt"
	"strings"

	"github.com/FreezingSnail/magicite/internal/wire"
)

// DashboardMetricsInput contains the metrics lifecycle and last-good value
// prepared by the root model. Rendering uses no state, clock, or map.
type DashboardMetricsInput struct {
	Metrics MetricsState
}

// RenderDashboardMetrics renders the optional metrics pane from explicit input.
// Unsupported metrics intentionally render nothing so older daemons retain a
// usable Dashboard.
func RenderDashboardMetrics(input DashboardMetricsInput, width int) string {
	if width <= 0 || input.Metrics.Unsupported {
		return ""
	}
	if input.Metrics.Loading {
		return fitDashboardLines([]string{"Metrics", "loading"}, width)
	}
	if input.Metrics.Unavailable || !input.Metrics.HasLastGood {
		return fitDashboardLines([]string{"Metrics", "unavailable"}, width)
	}
	metrics := input.Metrics.LastGood
	title := "Metrics"
	if input.Metrics.Stale {
		title += " (stale)"
	}
	lines := []string{
		title,
		fmt.Sprintf("uptime: %ds", nonNegative(metrics.UptimeSeconds)),
		fmt.Sprintf("sessions: active %d; peak %d; completed %d; failed %d", metrics.Sessions.Active, metrics.Sessions.Peak, metrics.Sessions.Completed, metrics.Sessions.Failed),
		"lifecycle: " + dashboardMetricsCounts(metrics.Lifecycle, lifecycleMetricOrder),
		"land: " + dashboardMetricsCounts(metrics.Land, landMetricOrder),
		"roles: " + dashboardMetricRoles(metrics.Roles),
		fmt.Sprintf("queue: total %d; %s", metrics.QueueTotal, dashboardMetricQueue(metrics.Queue)),
		fmt.Sprintf("bus: published %d; dropped %d", metrics.Bus.Published, metrics.Bus.Dropped),
	}
	return fitDashboardLines(lines, width)
}

var lifecycleMetricOrder = []string{"pickup", "complete", "land", "close", "review", "verdict", "recovery", "warn", "error"}
var landMetricOrder = []string{"ok", "conflict", "gate_failed", "failed"}
var roleMetricOrder = []string{"concierge", "designer", "implementer", "reviewer", "repairer"}

func dashboardMetricsCounts(counts []wire.MetricsCount, order []string) string {
	if len(counts) == 0 {
		return "0"
	}
	parts := make([]string, 0, len(counts))
	used := make([]bool, len(counts))
	for _, key := range order {
		for index, count := range counts {
			if !used[index] && count.Key == key {
				parts = append(parts, fmt.Sprintf("%s %d", dashboardValue(count.Key, "unknown"), count.Count))
				used[index] = true
			}
		}
	}
	for index, count := range counts {
		if !used[index] {
			parts = append(parts, fmt.Sprintf("%s %d", dashboardValue(count.Key, "unknown"), count.Count))
		}
	}
	return strings.Join(parts, "; ")
}

func dashboardMetricRoles(roles []wire.RoleDuration) string {
	if len(roles) == 0 {
		return "0"
	}
	parts := make([]string, 0, len(roles))
	used := make([]bool, len(roles))
	for _, roleName := range roleMetricOrder {
		for index, role := range roles {
			if !used[index] && role.Role == roleName {
				parts = append(parts, dashboardMetricRole(role))
				used[index] = true
			}
		}
	}
	for index, role := range roles {
		if !used[index] {
			parts = append(parts, dashboardMetricRole(role))
		}
	}
	return strings.Join(parts, "; ")
}

func dashboardMetricRole(role wire.RoleDuration) string {
	return fmt.Sprintf("%s %d/%ds", dashboardValue(role.Role, "unknown"), role.Sessions, nonNegative(role.TotalSeconds))
}

func dashboardMetricQueue(queue []wire.RepoQueueDepth) string {
	if len(queue) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(queue))
	for _, depth := range queue {
		parts = append(parts, fmt.Sprintf("%s %d", dashboardValue(depth.Repo, "unknown"), depth.Depth))
	}
	return strings.Join(parts, "; ")
}

func nonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

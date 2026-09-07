package tui

import (
	"fmt"
	"time"
)

// DashboardConnectionState identifies the daemon connection state supplied by
// the runtime. RenderDashboardHeader does not derive it from other input.
type DashboardConnectionState string

const (
	DashboardConnectionLoading   DashboardConnectionState = "loading"
	DashboardConnectionConnected DashboardConnectionState = "connected"
	DashboardConnectionOffline   DashboardConnectionState = "offline"
)

// DashboardFreshness identifies the freshness of the displayed snapshot.
type DashboardFreshness string

const (
	DashboardFreshnessLoading DashboardFreshness = "loading"
	DashboardFreshnessFresh   DashboardFreshness = "fresh"
	DashboardFreshnessStale   DashboardFreshness = "stale"
)

// DashboardRefreshState identifies the runtime's current refresh state.
type DashboardRefreshState string

const (
	DashboardRefreshIdle       DashboardRefreshState = "idle"
	DashboardRefreshRefreshing DashboardRefreshState = "refreshing"
	DashboardRefreshError      DashboardRefreshState = "error"
)

// DashboardHeaderInput contains explicit daemon and refresh state for the
// Dashboard heading. Age is already measured by the runtime; rendering never
// reads a clock or infers lifecycle state.
type DashboardHeaderInput struct {
	Connection  DashboardConnectionState
	Freshness   DashboardFreshness
	SnapshotAge time.Duration
	Generation  uint64
	Running     bool
	Draining    bool
	Refresh     DashboardRefreshState
	Error       string
}

// RenderDashboardHeader renders the Dashboard heading from explicit state.
// Every line, including the title, is deterministically bounded by width.
func RenderDashboardHeader(input DashboardHeaderInput, width int) string {
	if width <= 0 {
		return ""
	}

	age := input.SnapshotAge
	if age < 0 {
		age = 0
	}
	draining := "no"
	if input.Draining {
		draining = "yes"
	}
	running := "stopped"
	if input.Running {
		running = "running"
	}

	lines := []string{
		"Dashboard",
		"connection: " + dashboardHeaderValue(string(input.Connection), "unknown"),
		fmt.Sprintf("snapshot: %s generation: %d age: %s", dashboardHeaderValue(string(input.Freshness), "unknown"), input.Generation, age),
		fmt.Sprintf("daemon: %s; draining: %s", running, draining),
		"refresh: " + dashboardHeaderValue(string(input.Refresh), "unknown"),
	}
	if input.Error != "" {
		lines = append(lines, "error: "+input.Error)
	}
	return fitDashboardLines(lines, width)
}

func dashboardHeaderValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

package tui

import (
	"strings"
	"testing"
	"time"
)

func TestRenderDashboardHeaderShowsExplicitState(t *testing.T) {
	got := RenderDashboardHeader(DashboardHeaderInput{
		Connection:  DashboardConnectionConnected,
		Freshness:   DashboardFreshnessFresh,
		SnapshotAge: time.Minute + 2*time.Second,
		Generation:  7,
		Running:     true,
		Draining:    true,
		Refresh:     DashboardRefreshRefreshing,
	}, 80)
	want := strings.Join([]string{
		"Dashboard",
		"connection: connected",
		"snapshot: fresh generation: 7 age: 1m2s",
		"daemon: running; draining: yes",
		"refresh: refreshing",
	}, "\n")
	if got != want {
		t.Fatalf("RenderDashboardHeader() = %q, want %q", got, want)
	}
}

func TestRenderDashboardHeaderShowsOfflineStaleError(t *testing.T) {
	got := RenderDashboardHeader(DashboardHeaderInput{
		Connection:  DashboardConnectionOffline,
		Freshness:   DashboardFreshnessStale,
		SnapshotAge: 10 * time.Second,
		Generation:  4,
		Refresh:     DashboardRefreshError,
		Error:       "daemon unavailable",
	}, 80)
	want := strings.Join([]string{
		"Dashboard",
		"connection: offline",
		"snapshot: stale generation: 4 age: 10s",
		"daemon: stopped; draining: no",
		"refresh: error",
		"error: daemon unavailable",
	}, "\n")
	if got != want {
		t.Fatalf("RenderDashboardHeader() = %q, want %q", got, want)
	}
}

func TestRenderDashboardHeaderWidthIsBounded(t *testing.T) {
	got := RenderDashboardHeader(DashboardHeaderInput{
		Connection: DashboardConnectionLoading,
		Freshness:  DashboardFreshnessLoading,
		Refresh:    DashboardRefreshIdle,
	}, 8)
	want := "Dashboa…\nconnect…\nsnapsho…\ndaemon:…\nrefresh…"
	if got != want {
		t.Fatalf("RenderDashboardHeader() = %q, want %q", got, want)
	}
	for _, line := range strings.Split(got, "\n") {
		if length := len([]rune(line)); length > 8 {
			t.Fatalf("line width = %d, want <= 8: %q", length, line)
		}
	}
	if got := RenderDashboardHeader(DashboardHeaderInput{}, 0); got != "" {
		t.Fatalf("RenderDashboardHeader(zero width) = %q, want empty", got)
	}
}

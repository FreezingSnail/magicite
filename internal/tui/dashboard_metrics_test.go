package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestRenderDashboardMetricsOrdersExplicitDataWithoutMutation(t *testing.T) {
	input := DashboardMetricsInput{Metrics: MetricsState{HasLastGood: true, LastGood: transport.Metrics{
		UptimeSeconds: 60,
		Lifecycle:     []wire.MetricsCount{{Key: "error", Count: 1}, {Key: "pickup", Count: 2}, {Key: "custom", Count: 3}},
		Land:          []wire.MetricsCount{{Key: "failed", Count: 4}, {Key: "ok", Count: 5}},
		Sessions:      wire.SessionGauges{Active: 1, Peak: 2, Completed: 3, Failed: 4},
		Roles:         []wire.RoleDuration{{Role: "reviewer", Sessions: 5, TotalSeconds: 6}, {Role: "implementer", Sessions: 7, TotalSeconds: 8}},
		Queue:         []wire.RepoQueueDepth{{Repo: "zeta", Depth: 9}, {Repo: "alpha", Depth: 10}}, QueueTotal: 19,
		Bus: wire.BusMetrics{Published: 20, Dropped: 21},
	}}}
	original := input
	original.Metrics.LastGood.Lifecycle = append([]wire.MetricsCount(nil), input.Metrics.LastGood.Lifecycle...)
	original.Metrics.LastGood.Land = append([]wire.MetricsCount(nil), input.Metrics.LastGood.Land...)
	original.Metrics.LastGood.Roles = append([]wire.RoleDuration(nil), input.Metrics.LastGood.Roles...)
	original.Metrics.LastGood.Queue = append([]wire.RepoQueueDepth(nil), input.Metrics.LastGood.Queue...)
	want := "Metrics\n" +
		"uptime: 60s\n" +
		"sessions: active 1; peak 2; completed 3; failed 4\n" +
		"lifecycle: pickup 2; error 1; custom 3\n" +
		"land: ok 5; failed 4\n" +
		"roles: implementer 7/8s; reviewer 5/6s\n" +
		"queue: total 19; zeta 9; alpha 10\n" +
		"bus: published 20; dropped 21"
	if got := RenderDashboardMetrics(input, 80); got != want {
		t.Fatalf("RenderDashboardMetrics() = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(input, original) {
		t.Fatalf("RenderDashboardMetrics() mutated input: %#v", input)
	}
}

func TestRenderDashboardMetricsHandlesLifecycleAndWidth(t *testing.T) {
	for _, test := range []struct {
		name  string
		state MetricsState
		want  string
	}{
		{"loading", MetricsState{Loading: true}, "Metrics\nloading"},
		{"unavailable", MetricsState{Unavailable: true}, "Metrics\nunavailable"},
		{"unsupported", MetricsState{Unsupported: true}, ""},
		{"stale", MetricsState{HasLastGood: true, Stale: true, LastGood: transport.Metrics{Lifecycle: []wire.MetricsCount{}, Land: []wire.MetricsCount{}, Roles: []wire.RoleDuration{}, Queue: []wire.RepoQueueDepth{}}}, "Metrics (stale)\nuptime: 0s\nsessions: active 0; peak 0; completed 0; failed 0\nlifecycle: 0\nland: 0\nroles: 0\nqueue: total 0; none\nbus: published 0; dropped 0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := RenderDashboardMetrics(DashboardMetricsInput{Metrics: test.state}, 80); got != test.want {
				t.Fatalf("RenderDashboardMetrics() = %q, want %q", got, test.want)
			}
		})
	}
	got := RenderDashboardMetrics(DashboardMetricsInput{Metrics: MetricsState{HasLastGood: true, LastGood: transport.Metrics{Lifecycle: []wire.MetricsCount{}, Land: []wire.MetricsCount{}, Roles: []wire.RoleDuration{}, Queue: []wire.RepoQueueDepth{}}}}, 18)
	for _, line := range strings.Split(got, "\n") {
		if len([]rune(line)) > 18 {
			t.Fatalf("wide line %q", line)
		}
	}
}

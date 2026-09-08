package tui

import (
	"reflect"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestCloneModelStateCopiesMetricsCollections(t *testing.T) {
	sampledAt := time.Unix(30, 0)
	state := ModelState{Metrics: MetricsState{HasLastGood: true, LastGood: transport.Metrics{
		Lifecycle: []wire.MetricsCount{{Key: "pickup", Count: 1}}, Land: []wire.MetricsCount{{Key: "ok", Count: 2}},
		Roles: []wire.RoleDuration{{Role: "implementer", Sessions: 3}}, Queue: []wire.RepoQueueDepth{{Repo: "magicite", Depth: 4}}, QueueSampledAt: &sampledAt,
	}}}
	copy := cloneModelState(state)
	copy.Metrics.LastGood.Lifecycle[0].Count = 9
	copy.Metrics.LastGood.Land[0].Count = 9
	copy.Metrics.LastGood.Roles[0].Sessions = 9
	copy.Metrics.LastGood.Queue[0].Depth = 9
	*copy.Metrics.LastGood.QueueSampledAt = time.Unix(99, 0)
	if reflect.DeepEqual(state, copy) || state.Metrics.LastGood.Lifecycle[0].Count != 1 || !state.Metrics.LastGood.QueueSampledAt.Equal(sampledAt) {
		t.Fatalf("clone mutated source: %#v", state)
	}
}

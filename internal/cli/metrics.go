package cli

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/FreezingSnail/magicite/internal/wire"
)

func init() {
	Register(Command{Name: "metrics", Usage: "metrics", Summary: "show fleet counters", Run: metrics})
}

func metrics(ctx context.Context, e *Env, args []string) int {
	if len(args) != 0 {
		return commandUsage(e, "metrics")
	}

	var raw json.RawMessage
	if err := e.Client.Call(ctx, "metrics", nil, &raw); err != nil {
		return Fail(e, err)
	}
	if e.JSON {
		return emitPayload(e, "metrics", raw)
	}

	var result wire.MetricsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return Fail(e, err)
	}
	return renderMetrics(e, result)
}

func renderMetrics(e *Env, result wire.MetricsResult) int {
	if err := EmitLine(e.Out, "started: %s  uptime: %d  active: %d  peak: %d  completed: %d  failed: %d", result.StartedAt.Format(time.RFC3339), result.UptimeSeconds, result.Sessions.Active, result.Sessions.Peak, result.Sessions.Completed, result.Sessions.Failed); err != nil {
		return Fail(e, err)
	}

	lifecycle := append([]wire.MetricsCount(nil), result.Lifecycle...)
	sort.Slice(lifecycle, func(i, j int) bool { return lifecycle[i].Key < lifecycle[j].Key })
	if err := emitMetricsCounts(e, []string{"kind", "count"}, lifecycle); err != nil {
		return Fail(e, err)
	}

	land := append([]wire.MetricsCount(nil), result.Land...)
	sort.Slice(land, func(i, j int) bool { return land[i].Key < land[j].Key })
	if err := emitMetricsCounts(e, []string{"result", "count"}, land); err != nil {
		return Fail(e, err)
	}

	queue := append([]wire.RepoQueueDepth(nil), result.Queue...)
	sort.Slice(queue, func(i, j int) bool { return queue[i].Repo < queue[j].Repo })
	queueRows := make([][]string, len(queue))
	for i, depth := range queue {
		queueRows[i] = []string{depth.Repo, strconv.Itoa(depth.Depth)}
	}
	if err := EmitTable(e.Out, []string{"repo", "depth"}, queueRows); err != nil {
		return Fail(e, err)
	}
	if err := EmitLine(e.Out, "queue total: %d  sampled: %s", result.QueueTotal, metricsSampleTime(result.QueueSampledAt)); err != nil {
		return Fail(e, err)
	}

	roles := append([]wire.RoleDuration(nil), result.Roles...)
	sort.Slice(roles, func(i, j int) bool { return roles[i].Role < roles[j].Role })
	roleRows := make([][]string, len(roles))
	for i, role := range roles {
		roleRows[i] = []string{role.Role, strconv.FormatUint(role.Sessions, 10), strconv.FormatInt(role.TotalSeconds, 10)}
	}
	if err := EmitTable(e.Out, []string{"role", "sessions", "total_seconds"}, roleRows); err != nil {
		return Fail(e, err)
	}

	if err := EmitLine(e.Out, "bus: published: %d  dropped: %d", result.Bus.Published, result.Bus.Dropped); err != nil {
		return Fail(e, err)
	}
	return 0
}

func emitMetricsCounts(e *Env, headers []string, counts []wire.MetricsCount) error {
	rows := make([][]string, len(counts))
	for i, count := range counts {
		rows[i] = []string{count.Key, strconv.FormatUint(count.Count, 10)}
	}
	return EmitTable(e.Out, headers, rows)
}

func metricsSampleTime(sampledAt *time.Time) string {
	if sampledAt == nil {
		return "never"
	}
	return sampledAt.Format(time.RFC3339)
}

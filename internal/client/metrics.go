package client

import (
	"context"

	"github.com/FreezingSnail/magicite/internal/wire"
)

// Metrics returns one daemon-owned cumulative fleet metrics snapshot.
func (c *Client) Metrics(ctx context.Context) (wire.MetricsResult, error) {
	var result wire.MetricsResult
	if err := c.Call(ctx, "metrics", nil, &result); err != nil {
		return wire.MetricsResult{}, err
	}
	return cloneMetricsResult(result), nil
}

func cloneMetricsResult(result wire.MetricsResult) wire.MetricsResult {
	result.Lifecycle = cloneSnapshotSlice(result.Lifecycle)
	result.Land = cloneSnapshotSlice(result.Land)
	result.Roles = cloneSnapshotSlice(result.Roles)
	result.Queue = cloneSnapshotSlice(result.Queue)
	return result
}

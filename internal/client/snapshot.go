package client

import (
	"context"

	"github.com/FreezingSnail/magicite/internal/wire"
)

// Snapshot returns one coherent daemon-owned fleet snapshot.
func (c *Client) Snapshot(ctx context.Context) (wire.SnapshotResult, error) {
	var result wire.SnapshotResult
	if err := c.Call(ctx, "snapshot", nil, &result); err != nil {
		return wire.SnapshotResult{}, err
	}
	return cloneSnapshotResult(result), nil
}

func cloneSnapshotResult(result wire.SnapshotResult) wire.SnapshotResult {
	result.Repositories = cloneSnapshotSlice(result.Repositories)
	result.Seats = cloneSnapshotSlice(result.Seats)
	result.Sessions = cloneSnapshotSlice(result.Sessions)
	result.Beads = cloneSnapshotSlice(result.Beads)
	result.StatusCounts = cloneSnapshotSlice(result.StatusCounts)
	result.RepositoryErrors = cloneSnapshotSlice(result.RepositoryErrors)
	result.Runtime.Sessions = cloneSnapshotSlice(result.Runtime.Sessions)
	for index := range result.Beads {
		result.Beads[index].Dependencies = cloneSnapshotSlice(result.Beads[index].Dependencies)
		result.Beads[index].Labels = cloneSnapshotSlice(result.Beads[index].Labels)
	}
	return result
}

func cloneSnapshotSlice[T any](values []T) []T {
	return append([]T{}, values...)
}

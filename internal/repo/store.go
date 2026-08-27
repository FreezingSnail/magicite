package repo

import "context"

// Lookup resolves repository records for daemon consumers.
type Lookup interface {
	List(ctx context.Context) []Repo
	Refresh(ctx context.Context) []Repo
	Invalidate()
	Get(ctx context.Context, nameOrRoot string) (Repo, error)
	Current(ctx context.Context, dir string) (Repo, error)
	ForBead(ctx context.Context, id string) (Repo, error)
}

var _ Lookup = (*Registry)(nil)

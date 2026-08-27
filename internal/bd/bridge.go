package bd

import "context"

// Bridge is the synchronous bd surface used by higher-level packages.
type Bridge interface {
	Show(ctx context.Context, id string) (Bead, error)
	List(ctx context.Context, all bool) ([]Bead, error)
	Ready(ctx context.Context) ([]Bead, error)
	Query(ctx context.Context, q string, all bool) ([]Bead, error)
	Deps(ctx context.Context, id string) ([]Dependency, error)
	Labels(ctx context.Context, id string) ([]string, error)
	Create(ctx context.Context, req CreateRequest) (string, error)
	Update(ctx context.Context, id string, req UpdateRequest) error
	Claim(ctx context.Context, id string) error
	Release(ctx context.Context, id string) error
	Close(ctx context.Context, id, reason string) error
	Comment(ctx context.Context, id, text string) error
	LabelAdd(ctx context.Context, id, label string) error
	LabelRemove(ctx context.Context, id, label string) error
	Defer(ctx context.Context, id, until string) error
	Undefer(ctx context.Context, id string) error
	DepAdd(ctx context.Context, id, dependsOn string) error
}

var _ Bridge = (*Client)(nil)

package worktree

import (
	"context"

	"github.com/FreezingSnail/magicite/internal/config"
)

// Provisioner manages per-seat repository worktrees.
type Provisioner interface {
	Path(repo Repo, seat string) (string, error)
	Branch(repo Repo, seat string) (string, error)
	Ensure(ctx context.Context, repo Repo, seat string) (string, error)
	Sync(ctx context.Context, repo Repo, seat string) (SyncResult, error)
	Cleanup(ctx context.Context, repo Repo, seat string) error
}

var _ Provisioner = (*Manager)(nil)

// FromConfig builds a Manager from daemon configuration.
func FromConfig(cfg config.Config, runner Runner) (*Manager, error) {
	return New(Options{WorkspacePath: cfg.Workspaces.Path, Runner: runner})
}

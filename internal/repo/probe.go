package repo

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	magicexec "github.com/FreezingSnail/magicite/internal/exec"
	"github.com/FreezingSnail/magicite/internal/logging"
)

// Prober checks whether a directory is an admitted fleet repository.
type Prober struct {
	Git string
	Log func(level logging.Level, kind string, fields map[string]any)
}

// NewProber creates a repository prober backed by git.
func NewProber() *Prober {
	return &Prober{Git: "git"}
}

// GitRoot returns the normalized worktree root for dir.
func (p *Prober) GitRoot(ctx context.Context, dir string) (string, bool) {
	root, ok, _ := p.gitRoot(ctx, dir)
	return root, ok
}

// HasBeads reports whether root contains a .beads directory.
func (p *Prober) HasBeads(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".beads"))
	return err == nil && info.IsDir()
}

// Admit returns candidate when it is a worktree root containing .beads.
func (p *Prober) Admit(ctx context.Context, candidate string) (string, bool) {
	root, ok := Directory(candidate)
	if !ok {
		reason := "invalid-root"
		if ctx.Err() != nil {
			reason = "probe-failed"
		}
		p.skip(candidate, reason)
		return "", false
	}
	if ctx.Err() != nil {
		p.skip(root, "probe-failed")
		return "", false
	}

	gitRoot, ok, ran := p.gitRoot(ctx, root)
	if !ok {
		reason := "not-worktree-root"
		if !ran || ctx.Err() != nil {
			reason = "probe-failed"
		}
		p.skip(root, reason)
		return "", false
	}
	if ctx.Err() != nil {
		p.skip(root, "probe-failed")
		return "", false
	}
	if !SameRoot(root, gitRoot) {
		p.skip(root, "not-worktree-root")
		return "", false
	}
	if !p.HasBeads(root) {
		p.skip(root, "no-beads")
		return "", false
	}
	return root, true
}

func (p *Prober) gitRoot(ctx context.Context, dir string) (string, bool, bool) {
	stdout, _, exitCode, runErr := magicexec.Run(ctx, dir, p.Git, "-C", dir, "rev-parse", "--show-toplevel")
	if ctx.Err() != nil {
		return "", false, false
	}
	if runErr != nil {
		return "", false, exitCode >= 0
	}
	if exitCode != 0 {
		return "", false, true
	}

	trimmed := strings.TrimRight(string(stdout), " \t\r\n")
	if trimmed == "" {
		return "", false, true
	}
	root, ok := Directory(trimmed)
	return root, ok, true
}

func (p *Prober) skip(root, reason string) {
	fields := map[string]any{"root": root, "reason": reason}
	if p.Log != nil {
		p.Log(logging.Debug, "repo.skip", fields)
		return
	}
	logging.Event(logging.Debug, "repo.skip", fields)
}

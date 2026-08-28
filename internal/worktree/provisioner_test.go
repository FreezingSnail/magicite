package worktree

import (
	"errors"
	"testing"

	"github.com/connorfranc/magicite/internal/config"
)

func TestFromConfig(t *testing.T) {
	runner := newFakeRunner()
	manager, err := FromConfig(config.Config{Workspaces: config.WorkspaceConfig{Path: "custom/workspaces"}}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if manager.workspacePath != "custom/workspaces" {
		t.Errorf("workspace path = %q, want custom/workspaces", manager.workspacePath)
	}
	if manager.runner != runner {
		t.Error("runner was not passed through")
	}
}

func TestFromConfigUsesExecRunnerAndNewValidation(t *testing.T) {
	manager, err := FromConfig(config.Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.runner.(execRunner); !ok {
		t.Errorf("runner = %T, want execRunner", manager.runner)
	}

	_, err = FromConfig(config.Config{Workspaces: config.WorkspaceConfig{Path: "../escape"}}, nil)
	if !errors.Is(err, ErrInvalidOptions) {
		t.Errorf("FromConfig() error = %v, want ErrInvalidOptions", err)
	}
}

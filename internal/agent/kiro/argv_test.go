package kiro

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestValidAgent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "installed.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "directory.json"), 0o700); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		agentsDir string
		agent     string
		want      bool
	}{
		{name: "default", agentsDir: dir, want: true},
		{name: "installed", agentsDir: dir, agent: "installed", want: true},
		{name: "missing", agentsDir: dir, agent: "missing", want: false},
		{name: "directory", agentsDir: dir, agent: "directory", want: false},
		{name: "slash", agentsDir: dir, agent: "nested/agent", want: false},
		{name: "backslash", agentsDir: dir, agent: `nested\agent`, want: false},
		{name: "parent", agentsDir: dir, agent: "..", want: false},
		{name: "parent reference", agentsDir: dir, agent: "../agent", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidAgent(tt.agentsDir, tt.agent); got != tt.want {
				t.Errorf("ValidAgent(%q, %q) = %v, want %v", tt.agentsDir, tt.agent, got, tt.want)
			}
		})
	}
}

func TestValidModel(t *testing.T) {
	for _, tt := range []struct {
		model string
		want  bool
	}{
		{model: "", want: false},
		{model: "claude-sonnet", want: true},
		{model: "provider/model", want: false},
		{model: `provider\\model`, want: true},
	} {
		if got := ValidModel(tt.model); got != tt.want {
			t.Errorf("ValidModel(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestValidEffort(t *testing.T) {
	for _, tt := range []struct {
		effort string
		want   bool
	}{
		{effort: "", want: false},
		{effort: "low", want: true},
		{effort: "MEDIUM", want: true},
		{effort: "high", want: true},
		{effort: "xhigh", want: true},
		{effort: "max", want: true},
		{effort: "higher", want: false},
		{effort: "low effort", want: false},
		{effort: "low\n", want: false},
		{effort: "low/high", want: false},
	} {
		if got := ValidEffort(tt.effort); got != tt.want {
			t.Errorf("ValidEffort(%q) = %v, want %v", tt.effort, got, tt.want)
		}
	}
}

func TestRunArgs(t *testing.T) {
	tests := []struct {
		name   string
		exe    string
		model  string
		agent  string
		effort string
		plan   string
		want   []string
	}{
		{
			name:   "all options",
			exe:    "/bin/kiro-cli",
			model:  "claude-sonnet",
			agent:  "builder",
			effort: "high",
			plan:   "implement feature",
			want:   []string{"/bin/kiro-cli", "chat", "--no-interactive", "--agent", "builder", "--model", "claude-sonnet", "--effort", "high", "--trust-all-tools", "implement feature"},
		},
		{
			name:   "default agent and invalid effort",
			exe:    "kiro-cli",
			model:  "model",
			effort: "not-valid",
			plan:   "--looks-like-a-flag\nsecond line",
			want:   []string{"kiro-cli", "chat", "--no-interactive", "--model", "model", "--trust-all-tools", "--looks-like-a-flag\nsecond line"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RunArgs(tt.exe, tt.model, tt.agent, tt.effort, tt.plan)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RunArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

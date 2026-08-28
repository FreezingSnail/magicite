// Package testenv provides hermetic process environments for parity tests.
package testenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Env is an isolated filesystem and explicit environment for child processes.
type Env struct {
	Root      string
	HomeDir   string
	BinDir    string
	TracePath string

	t                 *testing.T
	installed         map[string]string
	fakeBDStore       string
	fakeAgentStore    string
	fakeAgentEvents   string
	fakeAgentPIDs     string
	fakeAgentScenario string
}

// New creates an isolated environment rooted in the test's temporary directory.
func New(t *testing.T) *Env {
	t.Helper()

	root := t.TempDir()
	env := &Env{
		Root:      root,
		HomeDir:   filepath.Join(root, "home"),
		BinDir:    filepath.Join(root, "bin"),
		TracePath: filepath.Join(root, "trace.tsv"),
		t:         t,
		installed: make(map[string]string),
	}
	for _, dir := range []string{
		env.HomeDir,
		env.BinDir,
		filepath.Join(root, "xdg", "config"),
		filepath.Join(root, "xdg", "cache"),
		filepath.Join(root, "xdg", "data"),
		filepath.Join(root, "cache", "go"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create test environment directory %q: %v", dir, err)
		}
	}
	return env
}

// Env returns the complete, non-inherited environment for a child process.
func (e *Env) Env() []string {
	env := []string{
		"PATH=" + e.BinDir,
		"HOME=" + e.HomeDir,
		"XDG_CONFIG_HOME=" + filepath.Join(e.Root, "xdg", "config"),
		"XDG_CACHE_HOME=" + filepath.Join(e.Root, "xdg", "cache"),
		"XDG_DATA_HOME=" + filepath.Join(e.Root, "xdg", "data"),
		"MAGICITE_TRACE=" + e.TracePath,
		"GIT_CONFIG_GLOBAL=" + filepath.Join(e.Root, "gitconfig"),
		"GIT_CONFIG_NOSYSTEM=1",
	}
	if e.fakeBDStore != "" {
		env = append(env, "MAGICITE_FAKE_BD_STORE="+e.fakeBDStore)
	}
	if e.fakeAgentStore != "" {
		env = append(env,
			"MAGICITE_FAKE_AGENT_STORE="+e.fakeAgentStore,
			"MAGICITE_FAKE_AGENT_EVENTS="+e.fakeAgentEvents,
			"MAGICITE_FAKE_AGENT_PIDS="+e.fakeAgentPIDs,
			"MAGICITE_FAKE_AGENT_SCENARIO="+e.fakeAgentScenario,
		)
	}
	return env
}

// Install builds srcPkg from the repository and installs it in BinDir as name.
func (e *Env) Install(name, srcPkg string) string {
	e.t.Helper()
	if name == "" || filepath.Base(name) != name || name == "." {
		e.t.Fatalf("invalid installed binary name %q", name)
	}

	root, err := repositoryRoot()
	if err != nil {
		e.t.Fatalf("locate repository root: %v", err)
	}
	path := filepath.Join(e.BinDir, name)
	cmd := exec.Command("go", "build", "-o", path, srcPkg)
	cmd.Dir = root
	cmd.Env = append(e.Env(), "GOCACHE="+filepath.Join(e.Root, "cache", "go"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("build %q as %q: %v\n%s", srcPkg, name, err, output)
	}
	e.installed[name] = path
	return path
}

// Bin returns the path of an installed binary.
func (e *Env) Bin(name string) string {
	e.t.Helper()
	path, ok := e.installed[name]
	if !ok {
		e.t.Fatalf("binary %q is not installed", name)
	}
	return path
}

func repositoryRoot() (string, error) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("testenv source path unavailable")
	}
	return filepath.Abs(filepath.Join(filepath.Dir(source), "..", ".."))
}

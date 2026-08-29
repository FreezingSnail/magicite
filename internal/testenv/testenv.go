// Package testenv provides hermetic process environments for parity tests.
package testenv

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

const fakeProcessEnv = "MAGICITE_TESTENV_FAKE"

// init dispatches a re-executed test binary to the fake it was published as.
// Install refuses to publish any name this switch cannot route, so a marked
// process carrying some other name is a test re-executing itself on purpose --
// the parity offline probe does exactly that -- and must run as a test.
func init() {
	if os.Getenv(fakeProcessEnv) != "1" {
		return
	}
	switch filepath.Base(os.Args[0]) {
	case "bd":
		RunFakeBD()
	case "kiro", "opencode", "kiro-cli-chat":
		RunFakeAgent()
	default:
		return
	}
	syscall.Exit(0)
}

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
		fakeProcessEnv + "=1",
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

// Install publishes a re-executable fake CLI in BinDir as name. The test binary
// is hardlinked rather than compiled, so a suite performs no builds at all.
func (e *Env) Install(name, srcPkg string) string {
	e.t.Helper()
	if name == "" || filepath.Base(name) != name || name == "." {
		e.t.Fatalf("invalid installed binary name %q", name)
	}
	if !fakePackage(srcPkg) {
		e.t.Fatalf("unsupported fake package %q", srcPkg)
	}
	if !dispatchableFake(name) {
		e.t.Fatalf("fake name %q has no dispatch in the re-executed test binary", name)
	}
	if path, ok := e.installed[name]; ok {
		return path
	}

	executable, err := os.Executable()
	if err != nil {
		e.t.Fatalf("locate test executable: %v", err)
	}
	path := filepath.Join(e.BinDir, name)
	temporary := path + ".tmp"
	if err := publishExecutable(executable, temporary); err != nil {
		_ = os.Remove(temporary)
		e.t.Fatalf("publish fake %q: %v", name, err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		e.t.Fatalf("publish fake %q: %v", name, err)
	}
	e.installed[name] = path
	return path
}

// publishExecutable links source to destination, copying only when the two
// cannot share an inode. A hardlink writes no bytes; the copy exists because
// BinDir and the test binary may sit on different filesystems.
func publishExecutable(source, destination string) error {
	if err := os.Link(source, destination); err == nil {
		return nil
	}
	reader, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open test executable: %w", err)
	}
	defer func() { _ = reader.Close() }()
	writer, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return fmt.Errorf("create fake: %w", err)
	}
	if _, err := io.Copy(writer, reader); err != nil {
		_ = writer.Close()
		return fmt.Errorf("copy test executable: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close fake: %w", err)
	}
	return nil
}

func fakePackage(srcPkg string) bool {
	return srcPkg == "./cmd/fake-bd" || srcPkg == "./cmd/fake-agent"
}

// dispatchableFake reports whether init routes a process published as name.
func dispatchableFake(name string) bool {
	switch name {
	case "bd", "kiro", "opencode", "kiro-cli-chat":
		return true
	}
	return false
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

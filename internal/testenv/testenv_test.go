package testenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCreatesHermeticEnvironment(t *testing.T) {
	t.Setenv("MAGICITE_TESTENV_LEAK", "ambient")
	env := New(t)

	for _, path := range []string{env.Root, env.HomeDir, env.BinDir, filepath.Dir(env.TracePath)} {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatalf("directory %q = %v, want directory", path, err)
		}
	}
	values := environmentMap(env.Env())
	if got, want := values["PATH"], env.BinDir; got != want {
		t.Errorf("PATH = %q, want %q", got, want)
	}
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "GIT_CONFIG_GLOBAL"} {
		if !inside(env.Root, values[name]) {
			t.Errorf("%s = %q, want path inside %q", name, values[name], env.Root)
		}
	}
	if got, want := values["MAGICITE_TRACE"], env.TracePath; got != want {
		t.Errorf("MAGICITE_TRACE = %q, want %q", got, want)
	}
	if got := values["MAGICITE_TESTENV_LEAK"]; got != "" {
		t.Errorf("inherited value = %q, want empty", got)
	}
}

func TestInstallReexecsTestBinaryWithoutCompiling(t *testing.T) {
	env := New(t)
	path := env.Install("bd", "./cmd/fake-bd")
	first, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := os.Stat(executable)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(first, compiled) {
		t.Errorf("installed fake = %q, want hardlink to test executable %q", path, executable)
	}
	if got := env.Install("bd", "./cmd/fake-bd"); got != path {
		t.Errorf("second Install() = %q, want %q", got, path)
	}
	second, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(first, second) || len(env.installed) != 1 {
		t.Errorf("repeated Install() republished fake: %v, installed = %#v", second, env.installed)
	}
}

// Every name Install accepts must have a dispatch in init. A published fake that
// init does not recognize would re-run the whole suite as a child process.
func TestInstallableNamesAllDispatch(t *testing.T) {
	for _, name := range []string{"bd", "kiro", "opencode", "kiro-cli-chat"} {
		if !dispatchableFake(name) {
			t.Errorf("dispatchableFake(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"", "bdx", "fake-bd", "magicite", "go"} {
		if dispatchableFake(name) {
			t.Errorf("dispatchableFake(%q) = true, want false", name)
		}
	}
}

// A hardlink writes no bytes, but BinDir and the test binary need not share a
// filesystem. Linking across devices fails, and the copy path carries that case.
func TestPublishExecutableCopiesAcrossFilesystems(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "fake")
	if err := publishExecutable(os.DevNull, destination); err != nil {
		t.Fatalf("publishExecutable(%q) error = %v", os.DevNull, err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("copied fake mode = %v, want 0755", info.Mode().Perm())
	}
	source, err := os.Stat(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(info, source) {
		t.Errorf("published fake shares an inode with %q, want a copy", os.DevNull)
	}
}

func environmentMap(env []string) map[string]string {
	values := make(map[string]string, len(env))
	for _, pair := range env {
		name, value, _ := strings.Cut(pair, "=")
		values[name] = value
	}
	return values
}

func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

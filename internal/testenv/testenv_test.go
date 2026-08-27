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

func TestInstallBuildsAndResolvesBinary(t *testing.T) {
	env := New(t)
	path := env.Install("fakebd", "./internal/bd/internal/fakebd")
	if got, want := env.Bin("fakebd"), path; got != want {
		t.Errorf("Bin(fakebd) = %q, want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("installed mode = %v, want executable", info.Mode())
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

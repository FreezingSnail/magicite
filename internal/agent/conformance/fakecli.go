// Package conformance supplies adapter conformance fixtures and assertions.
package conformance

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var fakeCLIBuild struct {
	sync.Once
	path string
	err  error
}

// FakeCLI builds and returns the fixture agent executable. The build runs once
// per test process so every conformance case uses the same argv-only program.
func FakeCLI(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("Go toolchain unavailable: fake agent CLI requires go build")
	}

	fakeCLIBuild.Do(func() {
		_, source, _, ok := runtime.Caller(0)
		if !ok {
			fakeCLIBuild.err = os.ErrNotExist
			return
		}
		dir, err := os.MkdirTemp("", "magicite-fakeagent-")
		if err != nil {
			fakeCLIBuild.err = err
			return
		}
		fakeCLIBuild.path = filepath.Join(dir, "fakeagent")
		cmd := exec.Command("go", "build", "-o", fakeCLIBuild.path, "./testdata/fakeagent")
		cmd.Dir = filepath.Dir(source)
		fakeCLIBuild.err = cmd.Run()
	})
	if fakeCLIBuild.err != nil {
		t.Fatalf("build fake agent CLI: %v", fakeCLIBuild.err)
	}
	return fakeCLIBuild.path
}

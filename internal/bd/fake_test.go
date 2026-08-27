package bd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

type fakeEntry struct {
	Match   []string `json:"match"`
	Stdout  string   `json:"stdout"`
	Stderr  string   `json:"stderr"`
	Exit    int      `json:"exit"`
	DelayMS int      `json:"delay_ms"`
}

var (
	fakeBuildOnce sync.Once
	fakeBinary    string
	fakeBuildErr  error
)

func buildFake(t *testing.T) string {
	t.Helper()
	fakeBuildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "magicite-fakebd-")
		if err != nil {
			fakeBuildErr = err
			return
		}
		fakeBinary = filepath.Join(dir, "bd")
		command := exec.Command("go", "build", "-o", fakeBinary, ".")
		command.Dir = filepath.Join("internal", "fakebd")
		if output, err := command.CombinedOutput(); err != nil {
			fakeBuildErr = &fakeBuildError{err: err, output: string(output)}
		}
	})
	if fakeBuildErr != nil {
		t.Fatal(fakeBuildErr)
	}
	return fakeBinary
}

type fakeBuildError struct {
	err    error
	output string
}

func (e *fakeBuildError) Error() string { return e.err.Error() + ": " + e.output }

func newFake(t *testing.T, entries ...fakeEntry) *Client {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bdfake", "calls"), 0o755); err != nil {
		t.Fatal(err)
	}
	script, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bdfake", "script.json"), script, 0o644); err != nil {
		t.Fatal(err)
	}
	client, err := New(buildFake(t), root)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func fakeCalls(t *testing.T, c *Client) [][]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(c.Root, "bdfake", "calls", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	calls := make([][]string, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var argv []string
		if err := json.Unmarshal(data, &argv); err != nil {
			t.Fatal(err)
		}
		calls = append(calls, argv)
	}
	return calls
}

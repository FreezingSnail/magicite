package repo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/bd"
)

type prefixRunner struct {
	result bd.Result
	err    error
	calls  [][]string
	onRun  func()
}

func (r *prefixRunner) Run(_ context.Context, args ...string) (bd.Result, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	if r.onRun != nil {
		r.onRun()
	}
	return r.result, r.err
}

func TestNewPrefixSourceUsesBDClient(t *testing.T) {
	root := t.TempDir()
	source := NewPrefixSource()
	if source.NewRunner == nil {
		t.Fatal("NewPrefixSource().NewRunner = nil")
	}

	runner, err := source.NewRunner(root)
	if err != nil {
		t.Fatal(err)
	}
	client, ok := runner.(*bd.Client)
	if !ok {
		t.Fatalf("NewRunner() = %T, want *bd.Client", runner)
	}
	if client.Program != "bd" || client.Root != root {
		t.Errorf("client = %#v, want Program bd and Root %q", client, root)
	}
}

func TestPrefixUsesRunnerBeforeFile(t *testing.T) {
	root := writePrefixConfig(t, "issue-prefix: file-prefix\n")
	runner := &prefixRunner{result: bd.Result{Stdout: []byte("ignored = value\n issue_prefix  =  runner-prefix \nissue_prefix = second\n")}}
	source := PrefixSource{NewRunner: func(gotRoot string) (Runner, error) {
		if gotRoot != root {
			t.Errorf("NewRunner root = %q, want %q", gotRoot, root)
		}
		return runner, nil
	}}

	prefix, ok := source.Prefix(context.Background(), root)
	if !ok || prefix != "runner-prefix" {
		t.Errorf("Prefix() = %q, %v, want runner-prefix, true", prefix, ok)
	}
	if want := [][]string{{"config", "list"}}; !reflect.DeepEqual(runner.calls, want) {
		t.Errorf("runner calls = %#v, want %#v", runner.calls, want)
	}
}

func TestPrefixFallsBackToFile(t *testing.T) {
	for _, test := range []struct {
		name   string
		result bd.Result
		err    error
		config string
		want   string
		ok     bool
	}{
		{
			name:   "runner error",
			err:    errors.New("bd unavailable"),
			config: "issue-prefix: file-prefix\n",
			want:   "file-prefix",
			ok:     true,
		},
		{
			name:   "nonzero exit",
			result: bd.Result{ExitCode: 1, Stdout: []byte("issue_prefix = runner-prefix\n")},
			config: "issue-prefix: file-prefix\n",
			want:   "file-prefix",
			ok:     true,
		},
		{
			name:   "invalid runner output",
			result: bd.Result{Stdout: []byte("issue_prefix = bad prefix\nissue_prefix = ***\n")},
			config: "issue-prefix: file-prefix\n",
			want:   "file-prefix",
			ok:     true,
		},
		{
			name:   "file accepts quoted first valid value",
			result: bd.Result{},
			config: "# issue-prefix: ignored\nissue-prefix: \"quoted-prefix\"\nissue-prefix: later\n",
			want:   "quoted-prefix",
			ok:     true,
		},
		{
			name:   "file rejects comments and empty values",
			result: bd.Result{},
			config: "  # issue-prefix: ignored\nissue-prefix: \nissue-prefix: \"\"\n",
			want:   "",
			ok:     false,
		},
		{
			name:   "file rejects invalid values",
			result: bd.Result{},
			config: "issue-prefix: invalid prefix\nissue-prefix: 'single-quotes'\n",
			want:   "",
			ok:     false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := writePrefixConfig(t, test.config)
			runner := &prefixRunner{result: test.result, err: test.err}
			source := PrefixSource{NewRunner: func(string) (Runner, error) { return runner, nil }}

			prefix, ok := source.Prefix(context.Background(), root)
			if prefix != test.want || ok != test.ok {
				t.Errorf("Prefix() = %q, %v, want %q, %v", prefix, ok, test.want, test.ok)
			}
		})
	}
}

func TestPrefixFileOnlyMisses(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(t *testing.T) string
	}{
		{
			name:  "missing file",
			setup: func(t *testing.T) string { return t.TempDir() },
		},
		{
			name: "unreadable file",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				if err := os.MkdirAll(filepath.Join(root, ".beads", "config.yaml"), 0o755); err != nil {
					t.Fatal(err)
				}
				return root
			},
		},
		{
			name: "bounded read",
			setup: func(t *testing.T) string {
				return writePrefixConfig(t, strings.Repeat(" ", maxConfigBytes)+"\nissue-prefix: beyond-limit\n")
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			prefix, ok := (PrefixSource{}).Prefix(context.Background(), test.setup(t))
			if prefix != "" || ok {
				t.Errorf("Prefix() = %q, %v, want empty false", prefix, ok)
			}
		})
	}
}

func TestPrefixCancelledContextSkipsSources(t *testing.T) {
	root := writePrefixConfig(t, "issue-prefix: file-prefix\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &prefixRunner{result: bd.Result{Stdout: []byte("issue_prefix = runner-prefix\n")}}
	source := PrefixSource{NewRunner: func(string) (Runner, error) { return runner, nil }}

	prefix, ok := source.Prefix(ctx, root)
	if prefix != "" || ok {
		t.Errorf("Prefix() = %q, %v, want empty false", prefix, ok)
	}
	if len(runner.calls) != 0 {
		t.Errorf("runner calls = %#v, want none", runner.calls)
	}

	ctx, cancel = context.WithCancel(context.Background())
	runner = &prefixRunner{
		result: bd.Result{Stdout: []byte("issue_prefix = runner-prefix\n")},
		onRun:  cancel,
	}
	source = PrefixSource{NewRunner: func(string) (Runner, error) { return runner, nil }}
	prefix, ok = source.Prefix(ctx, root)
	if prefix != "" || ok {
		t.Errorf("Prefix() after cancellation during Run = %q, %v, want empty false", prefix, ok)
	}
}

func TestValidPrefix(t *testing.T) {
	for _, test := range []struct {
		prefix string
		want   bool
	}{
		{prefix: "abc-DEF_123", want: true},
		{prefix: "", want: false},
		{prefix: "contains space", want: false},
		{prefix: "unicode-é", want: false},
		{prefix: "punctuation!", want: false},
	} {
		if got := ValidPrefix(test.prefix); got != test.want {
			t.Errorf("ValidPrefix(%q) = %v, want %v", test.prefix, got, test.want)
		}
	}
}

func writePrefixConfig(t *testing.T, config string) string {
	t.Helper()
	root := t.TempDir()
	directory := filepath.Join(root, ".beads")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

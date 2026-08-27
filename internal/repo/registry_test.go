package repo

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
)

type registryEvent struct {
	level  logging.Level
	kind   string
	fields map[string]any
}

func TestRegistryListCachesCopiesAndInvalidates(t *testing.T) {
	root := registryRoot(t)
	runner := &prefixRunner{result: prefixResult("fleet")}
	var events []registryEvent
	registry := NewWith(Options{
		Repos:  config.ReposConfig{Discover: "explicit", Roots: []string{root}},
		Probe:  NewProber(),
		Prefix: PrefixSource{NewRunner: func(string) (Runner, error) { return runner, nil }},
		Log: func(level logging.Level, kind string, fields map[string]any) {
			events = append(events, registryEvent{level, kind, fields})
		},
	})

	first := registry.List(context.Background())
	if len(first) != 1 || first[0].Prefix != "fleet" || !first[0].Valid() {
		t.Fatalf("List() = %#v, want one valid fleet record", first)
	}
	first[0].Name = "changed"
	second := registry.List(context.Background())
	if len(second) != 1 || second[0].Name == "changed" {
		t.Fatalf("second List() = %#v, want independent cached copy", second)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("prefix calls = %d, want 1", len(runner.calls))
	}

	registry.Invalidate()
	third := registry.List(context.Background())
	if len(third) != 1 || third[0].Prefix != "fleet" {
		t.Fatalf("List() after Invalidate() = %#v, want refreshed record", third)
	}
	if len(runner.calls) != 2 {
		t.Errorf("prefix calls after invalidation = %d, want 2", len(runner.calls))
	}
	if len(events) != 2 {
		t.Fatalf("events = %#v, want two refresh events", events)
	}
	for _, event := range events {
		if event.level != logging.Info || event.kind != "repo.refresh" || event.fields["count"] != 1 || !reflect.DeepEqual(event.fields["names"], []string{second[0].LogName()}) {
			t.Errorf("event = %#v, want info repo.refresh with count and names", event)
		}
	}
}

func TestRegistryRefreshFallsBackToAmbient(t *testing.T) {
	root := registryRoot(t)
	registry := NewWith(Options{
		Repos:  config.ReposConfig{},
		Dir:    root,
		Probe:  NewProber(),
		Prefix: PrefixSource{NewRunner: func(string) (Runner, error) { return &prefixRunner{result: prefixResult("ambient")}, nil }},
		Log:    func(logging.Level, string, map[string]any) {},
	})

	records := registry.Refresh(context.Background())
	if len(records) != 1 || !SameRoot(records[0].Root, root) || records[0].Prefix != "ambient" {
		t.Fatalf("Refresh() = %#v, want ambient root with derived prefix", records)
	}
}

func TestRegistryCancelledRefreshKeepsCache(t *testing.T) {
	root := registryRoot(t)
	registry := NewWith(Options{
		Repos:  config.ReposConfig{Discover: "explicit", Roots: []string{root}},
		Probe:  NewProber(),
		Prefix: PrefixSource{NewRunner: func(string) (Runner, error) { return &prefixRunner{result: prefixResult("before")}, nil }},
		Log:    func(logging.Level, string, map[string]any) {},
	})
	before := registry.List(context.Background())
	if len(before) != 1 {
		t.Fatal("initial List() returned no record")
	}

	ctx, cancel := context.WithCancel(context.Background())
	registry.options.Prefix = PrefixSource{NewRunner: func(string) (Runner, error) {
		return &prefixRunner{result: prefixResult("after"), onRun: cancel}, nil
	}}
	got := registry.Refresh(ctx)
	if !reflect.DeepEqual(got, before) {
		t.Errorf("cancelled Refresh() = %#v, want previous %#v", got, before)
	}
	if cached := registry.List(context.Background()); !reflect.DeepEqual(cached, before) {
		t.Errorf("List() after cancelled Refresh() = %#v, want %#v", cached, before)
	}
}

func TestRegistryWarnsWhenNothingIsAdmitted(t *testing.T) {
	var events []registryEvent
	registry := NewWith(Options{
		Repos: config.ReposConfig{Discover: "explicit", Roots: []string{filepath.Join(t.TempDir(), "missing")}},
		Log: func(level logging.Level, kind string, fields map[string]any) {
			events = append(events, registryEvent{level, kind, fields})
		},
	})

	if got := registry.Refresh(context.Background()); len(got) != 0 {
		t.Errorf("Refresh() = %#v, want empty", got)
	}
	want := []registryEvent{{logging.Warn, "repo.refresh", map[string]any{"count": 0, "reason": "none-admitted"}}}
	if !reflect.DeepEqual(events, want) {
		t.Errorf("events = %#v, want %#v", events, want)
	}
}

func TestNewWithDefaultsDependencies(t *testing.T) {
	registry := NewWith(Options{})
	if registry.options.Probe == nil || registry.options.Prefix.NewRunner == nil || registry.options.Log == nil {
		t.Errorf("NewWith(Options{}) dependencies = %#v, want defaults", registry.options)
	}
}

func registryRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "--quiet")
	if err := os.Mkdir(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func prefixResult(prefix string) bd.Result {
	return bd.Result{Stdout: []byte("issue_prefix = " + prefix + "\n")}
}

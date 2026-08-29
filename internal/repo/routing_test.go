package repo

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
)

func TestForBeadInRoutesByLongestPrefix(t *testing.T) {
	short := Repo{Name: "short", Prefix: "magicite"}
	long := Repo{Name: "long", Prefix: "magicite-2az"}
	repos := []Repo{short, long}

	for _, test := range []struct {
		id   string
		want Repo
	}{
		{id: "magicite-2az", want: short},
		{id: "magicite-2az-4", want: long},
		{id: "magicitex-1", want: Repo{}},
		{id: "magicite", want: Repo{}},
		{id: "magicite-", want: Repo{}},
	} {
		t.Run(test.id, func(t *testing.T) {
			got, err := ForBeadIn(repos, test.id)
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("ForBeadIn(%q) = %#v, %v; want %#v, nil", test.id, got, err, test.want)
			}
			if test.want == (Repo{}) && !IsNotFound(err) {
				t.Errorf("ForBeadIn(%q) error = %v, want not found", test.id, err)
			}
		})
	}
}

func TestForBeadInPreservesOrderForEqualPrefixes(t *testing.T) {
	first := Repo{Name: "first", Prefix: "shared"}
	second := Repo{Name: "second", Prefix: "shared"}
	repos := []Repo{first, second}

	got, err := ForBeadIn(repos, "shared-1")
	if err != nil || got != first {
		t.Fatalf("ForBeadIn() = %#v, %v; want first record, nil", got, err)
	}
}

func TestForBeadInNotFoundPreservesQueryAndRecords(t *testing.T) {
	repos := []Repo{{Name: "repo", Prefix: "prefix"}}
	before := append([]Repo(nil), repos...)
	for _, id := range []string{"", " \t\n", "missing-1"} {
		got, err := ForBeadIn(repos, id)
		if !reflect.DeepEqual(got, Repo{}) || !IsNotFound(err) {
			t.Errorf("ForBeadIn(%q) = %#v, %v; want zero and not found", id, got, err)
		}
		var notFound *NotFoundError
		if !errors.As(err, &notFound) || notFound.Query != id {
			t.Errorf("ForBeadIn(%q) error = %v, want query-bearing NotFoundError", id, err)
		}
	}
	if !reflect.DeepEqual(repos, before) {
		t.Errorf("repos mutated: %#v -> %#v", before, repos)
	}
}

func TestRegistryForBeadLoadsOnceAndDelegates(t *testing.T) {
	root := registryRoot(t)
	runner := &prefixRunner{result: prefixResult("fleet")}
	registry := NewWith(Options{
		Repos:  config.ReposConfig{Discover: "explicit", Roots: []string{root}},
		Probe:  NewProber(),
		Prefix: PrefixSource{NewRunner: func(string) (Runner, error) { return runner, nil }},
		Log:    func(logging.Level, string, map[string]any) {},
	})

	want := Repo{Root: root + string('/'), Name: pathBase(root), Prefix: "fleet", Branch: "main"}
	got, err := registry.ForBead(context.Background(), "fleet-2az")
	if err != nil || got != want {
		t.Fatalf("Registry.ForBead() = %#v, %v; want %#v, nil", got, err, want)
	}
	if _, err := registry.ForBead(context.Background(), "fleet-2az.4"); err != nil {
		t.Fatalf("warm Registry.ForBead() error = %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("prefix calls = %d, want one cold refresh", len(runner.calls))
	}
}

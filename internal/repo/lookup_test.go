package repo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
)

func TestGetInResolvesNameAndAbsoluteRoot(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	record := Repo{Root: root + string(filepath.Separator), Name: "alpha", Prefix: "a", Branch: "main"}

	if got, err := GetIn([]Repo{record}, "alpha"); err != nil || got != record {
		t.Errorf("GetIn by name = %#v, %v; want %#v, nil", got, err, record)
	}
	if got, err := GetIn([]Repo{record}, link); err != nil || got != record {
		t.Errorf("GetIn by root = %#v, %v; want %#v, nil", got, err, record)
	}
}

func TestGetInNotFoundIsTypedAndDoesNotMutate(t *testing.T) {
	records := []Repo{{Root: "/repo/", Name: "repo", Prefix: "r", Branch: "main"}}
	before := append([]Repo(nil), records...)
	if _, err := GetIn([]Repo{{Name: ""}}, ""); !IsNotFound(err) {
		t.Errorf("GetIn(empty) error = %v, want not found", err)
	}

	got, err := GetIn(records, "missing")
	if !reflect.DeepEqual(got, Repo{}) || !IsNotFound(err) {
		t.Fatalf("GetIn() = %#v, %v; want zero and not found", got, err)
	}
	var notFound *NotFoundError
	if !errors.As(err, &notFound) || notFound.Query != "missing" {
		t.Fatalf("error = %v, want query-bearing NotFoundError", err)
	}
	if err.Error() != "repository not found: missing" {
		t.Errorf("error = %q, want repository not found: missing", err)
	}
	if !reflect.DeepEqual(records, before) {
		t.Errorf("records mutated: %#v -> %#v", before, records)
	}
}

func TestCurrentInResolvesSymlinksAndLongestRoot(t *testing.T) {
	parent := t.TempDir()
	nested := filepath.Join(parent, "nested")
	child := filepath.Join(nested, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "parent-link")
	if err := os.Symlink(parent, link); err != nil {
		t.Fatal(err)
	}
	parentRecord := Repo{Root: parent + string(filepath.Separator), Name: "parent"}
	nestedRecord := Repo{Root: nested + string(filepath.Separator), Name: "nested"}

	got, err := CurrentIn([]Repo{parentRecord, nestedRecord}, filepath.Join(link, "nested", "child"))
	if err != nil || got != nestedRecord {
		t.Errorf("CurrentIn() = %#v, %v; want nested record, nil", got, err)
	}

	sibling := filepath.Join(parent, "nested-sibling")
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := CurrentIn([]Repo{nestedRecord}, sibling); !IsNotFound(err) {
		t.Errorf("CurrentIn(sibling) error = %v, want not found", err)
	}
}

func TestRegistryLookupLoadsAndDelegates(t *testing.T) {
	root := registryRoot(t)
	registry := NewWith(Options{
		Repos:  configRepos(root),
		Probe:  NewProber(),
		Prefix: PrefixSource{NewRunner: func(string) (Runner, error) { return &prefixRunner{result: prefixResult("p")}, nil }},
		Log:    func(logging.Level, string, map[string]any) {},
	})

	name := filepath.Base(root)
	got, err := registry.Get(context.Background(), name)
	if err != nil || got.Name != name {
		t.Errorf("Registry.Get() = %#v, %v; want %q, nil", got, err, name)
	}
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err = registry.Current(context.Background(), child)
	if err != nil || got.Name != name {
		t.Errorf("Registry.Current() = %#v, %v; want %q, nil", got, err, name)
	}
}

func configRepos(root string) config.ReposConfig {
	return config.ReposConfig{Discover: "explicit", Roots: []string{root}}
}

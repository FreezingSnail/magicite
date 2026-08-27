package config

import (
	"errors"
	"reflect"
	"testing"
)

func TestLoadDefaultsForMissingFile(t *testing.T) {
	got, err := Load("testdata/does-not-exist.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := Default()
	if got.Harness != want.Harness || !reflect.DeepEqual(got.Fleet, want.Fleet) || got.Welfare != want.Welfare || got.Workspaces != want.Workspaces {
		t.Fatalf("Load() = %#v, want default %#v", got, want)
	}
	if len(got.Repos.Roots) != 1 || got.Repos.Roots[0] != "/Users/FreezingSnail/code/magicite" {
		t.Fatalf("Load() repos = %#v, want shipped defaults", got.Repos)
	}
}

func TestLoadOverlaysSpecifiedKeys(t *testing.T) {
	got, err := Load("testdata/partial.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Fleet.PollInterval != 45 {
		t.Errorf("Fleet.PollInterval = %d, want 45", got.Fleet.PollInterval)
	}
	if got.Fleet.KiroModelLow != "gpt-5.6-luna" {
		t.Errorf("Fleet.KiroModelLow = %q, want shipped default", got.Fleet.KiroModelLow)
	}
	if got.Workspaces.Path != "harness/workspaces" {
		t.Errorf("Workspaces.Path = %q, want shipped default", got.Workspaces.Path)
	}
}

func TestLoadRejectsUnknownAndMalformedKeys(t *testing.T) {
	tests := []struct {
		path string
		key  string
	}{
		{path: "testdata/unknown.yaml", key: "fleet.unknown"},
		{path: "testdata/malformed.yaml", key: "fleet.poll-interval"},
	}
	for _, test := range tests {
		_, err := Load(test.path)
		var configErr *Error
		if !errors.As(err, &configErr) {
			t.Errorf("Load(%q) error = %v, want *Error", test.path, err)
			continue
		}
		if configErr.Key != test.key {
			t.Errorf("Load(%q) error key = %q, want %s", test.path, configErr.Key, test.key)
		}
	}
}

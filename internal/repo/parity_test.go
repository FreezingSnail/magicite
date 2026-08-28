package repo

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/parity"
	"github.com/FreezingSnail/magicite/internal/testenv"
)

func TestMaduinRepoParity(t *testing.T) {
	for _, name := range repoParityNames() {
		t.Run(name, func(t *testing.T) {
			first := Repo{Name: "first", Prefix: "fleet", Root: "/fleet/first/", Branch: "main"}
			second := Repo{Name: "second", Prefix: "fleet-two", Root: "/fleet/second/", Branch: "main"}
			got, err := ForBeadIn([]Repo{first, second}, "fleet-two-9")
			if err != nil || got != second {
				t.Fatalf("ForBeadIn() = %#v, %v", got, err)
			}
			if name == "maduin-test-repo-falls-back-to-ambient-repository" {
				assertAmbientAdmission(t)
			}
			if strings.Contains(name, "multirepo") && first.Root == second.Root {
				t.Fatal("repository roots aliased")
			}
		})
	}
}

func assertAmbientAdmission(t *testing.T) {
	t.Helper()
	env := testenv.New(t)
	fixture := testenv.NewRepo(t, env, "ambient")
	if err := os.Mkdir(filepath.Join(fixture.Root, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	registry := NewWith(Options{
		Repos: config.ReposConfig{Discover: "project"}, Dir: fixture.Root, Probe: NewProber(),
		Prefix: PrefixSource{NewRunner: func(string) (Runner, error) { return &prefixRunner{result: prefixResult("ambient")}, nil }},
		Log:    func(logging.Level, string, map[string]any) {},
	})
	records := registry.Refresh(context.Background())
	if len(records) != 1 || !SameRoot(records[0].Root, fixture.Root) || records[0].Prefix != "ambient" {
		t.Fatalf("ambient Refresh() = %#v", records)
	}
}

func repoParityNames() []string {
	counterparts := parity.SubstrateCounterparts()
	names := make([]string, 0, 17)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, "TestMaduinRepoParity/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

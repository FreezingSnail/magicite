package parity

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoadCatalogSnapshot(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(catalog.Entries), 437; got != want {
		t.Fatalf("catalog entries = %d, want %d", got, want)
	}

	for domain, want := range map[string]int{
		"cockpit": 109, "dispatch": 70, "config": 48, "session": 23,
		"pipeline": 18, "kiro": 17, "stamp": 14, "terminal": 12,
		"review": 12, "log": 12, "workspace": 11, "repo": 11,
		"main": 11, "designer": 11, "bd": 11, "backend": 8,
		"concierge": 7, "multirepo": 6, "state": 5, "handoff": 3,
		"decomp": 3, "backcompat": 3,
	} {
		if got := len(catalog.ByDomain[domain]); got != want {
			t.Errorf("%s count = %d, want %d", domain, got, want)
		}
	}
	invariant, ok := catalog.ByName["maduin-test-cockpit-show"]
	if !ok || invariant.Domain != "cockpit" || invariant.Line != 1479 {
		t.Fatalf("cockpit lookup = %#v, %t", invariant, ok)
	}
}

func TestParseCatalogRejectsInvalidData(t *testing.T) {
	for name, test := range map[string]struct {
		source string
		want   string
	}{
		"missing total":  {"one\talpha\t1\n", "missing total"},
		"malformed row":  {"# total: 1\none\talpha\n", "malformed row"},
		"invalid line":   {"# total: 1\none\talpha\tzero\n", "invalid source line"},
		"duplicate":      {"# total: 2\none\talpha\t1\none\tbeta\t2\n", "duplicate invariant"},
		"total mismatch": {"# total: 2\none\talpha\t1\n", "total mismatch"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseCatalog(strings.NewReader(test.source))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("parseCatalog() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseCatalogIndexesEntries(t *testing.T) {
	catalog, err := parseCatalog(strings.NewReader("# total: 2\none\talpha\t1\ntwo\talpha\t2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := catalog.ByName["two"], (Invariant{Name: "two", Domain: "alpha", Line: 2}); got != want {
		t.Fatalf("ByName[two] = %#v, want %#v", got, want)
	}
	if got, want := catalog.ByDomain["alpha"], []Invariant{{Name: "one", Domain: "alpha", Line: 1}, {Name: "two", Domain: "alpha", Line: 2}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ByDomain[alpha] = %#v, want %#v", got, want)
	}
}

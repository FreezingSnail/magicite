package parity

import (
	"reflect"
	"strings"
	"testing"
)

const catalogRevision = "49b476855c417fde6bc5e2d8797d2c787c93ae9d"

func TestLoadCatalogSnapshot(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := catalog.Revision, catalogRevision; got != want {
		t.Fatalf("catalog revision = %q, want %q", got, want)
	}
	if got, want := len(catalog.Entries), 444; got != want {
		t.Fatalf("catalog entries = %d, want %d", got, want)
	}

	for domain, want := range map[string]int{
		"cockpit": 109, "dispatch": 72, "config": 48, "session": 23,
		"pipeline": 18, "kiro": 17, "review": 16, "stamp": 14,
		"terminal": 12, "log": 12, "bd": 12, "workspace": 11,
		"repo": 11, "main": 11, "designer": 11, "backend": 8,
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
	for name, domain := range map[string]string{
		"maduin-test-bd-in-progress-query-includes-epics":              "bd",
		"maduin-test-dispatch-completed-decomposition-releases-epic":   "dispatch",
		"maduin-test-dispatch-recover-releases-orphaned-epic":          "dispatch",
		"maduin-test-review-error-exhaustion-closes-completed-epic":    "review",
		"maduin-test-review-disabled-gate-still-closes-completed-epic": "review",
		"maduin-test-review-drift-still-files-one-fix-and-holds-epic":  "review",
		"maduin-test-review-operator-close-requires-completed-epic":    "review",
	} {
		if got, ok := catalog.ByName[name]; !ok || got.Domain != domain {
			t.Errorf("new invariant %q = %#v, %t; want domain %q", name, got, ok, domain)
		}
	}
}

func TestParseCatalogRejectsInvalidData(t *testing.T) {
	revision := "# revision: 0123456789012345678901234567890123456789\n"
	for name, test := range map[string]struct {
		source string
		want   string
	}{
		"missing total":      {revision + "one\talpha\t1\n", "missing total"},
		"missing revision":   {"# total: 1\none\talpha\t1\n", "missing revision"},
		"invalid revision":   {"# revision: unknown\n# total: 1\none\talpha\t1\n", "invalid revision"},
		"duplicate revision": {revision + revision + "# total: 1\none\talpha\t1\n", "duplicate revision"},
		"malformed row":      {revision + "# total: 1\none\talpha\n", "malformed row"},
		"invalid line":       {revision + "# total: 1\none\talpha\tzero\n", "invalid source line"},
		"duplicate":          {revision + "# total: 2\none\talpha\t1\none\tbeta\t2\n", "duplicate invariant"},
		"total mismatch":     {revision + "# total: 2\none\talpha\t1\n", "total mismatch"},
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
	catalog, err := parseCatalog(strings.NewReader("# revision: 0123456789012345678901234567890123456789\n# total: 2\none\talpha\t1\ntwo\talpha\t2\n"))
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

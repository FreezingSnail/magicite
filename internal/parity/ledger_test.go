package parity

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoadLedgerJustifiesDomains(t *testing.T) {
	ledger, err := LoadLedger()
	if err != nil {
		t.Fatal(err)
	}
	if ok, reason := ledger.Justified("maduin-test-cockpit-show"); !ok || !strings.Contains(reason, "headless") {
		t.Fatalf("Justified(cockpit) = %t, %q", ok, reason)
	}
	if ok, reason := ledger.Justified("maduin-test-dispatch-queue-ready-entry"); ok || reason != "" {
		t.Fatalf("Justified(dispatch) = %t, %q", ok, reason)
	}
	want := []Divergence{
		{Target: "cockpit", Reason: "Magicite is headless and does not reproduce the Emacs cockpit UI.", Owner: "magicite-5o2.2"},
		{Target: "terminal", Reason: "Magicite is headless and does not reproduce the Emacs terminal UI.", Owner: "magicite-5o2.2"},
		{Target: goldenArgvTraceTarget, Reason: "Maduin is not executed because the default suite must remain offline and require neither Emacs nor a local maduin checkout; passing goldens establish magicite argv stability only, not argv equivalence with maduin.", Owner: "magicite-e85.5"},
	}
	if got := ledger.Reasons(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Reasons() = %#v, want %#v", got, want)
	}
}

func TestParseLedgerRejectsInvalidRows(t *testing.T) {
	catalog := testCatalog(t)
	for name, test := range map[string]struct {
		source string
		want   string
	}{
		"empty reason": {"one\t\tmagicite-1\n", "malformed row"},
		"orphan":       {"missing\treason\tmagicite-1\n", "unknown invariant, domain, or limitation"},
		"duplicate":    {"alpha\treason\tmagicite-1\nalpha\tagain\tmagicite-2\n", "duplicate target"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseLedger(strings.NewReader(test.source), catalog)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("parseLedger() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLedgerJustifiedPrefersExactTarget(t *testing.T) {
	catalog := testCatalog(t)
	ledger, err := parseLedger(strings.NewReader("alpha\tdomain reason\tmagicite-1\none\texact reason\tmagicite-2\n"), catalog)
	if err != nil {
		t.Fatal(err)
	}
	if ok, got := ledger.Justified("one"); !ok || got != "exact reason" {
		t.Fatalf("Justified(one) = %t, %q", ok, got)
	}
	if ok, got := ledger.Justified("two"); !ok || got != "domain reason" {
		t.Fatalf("Justified(two) = %t, %q", ok, got)
	}
}

func TestCoverage(t *testing.T) {
	catalog, err := parseCatalog(strings.NewReader("# total: 3\none\talpha\t1\ntwo\talpha\t2\nthree\tbeta\t3\n"))
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := parseLedger(strings.NewReader("alpha\tdomain reason\tmagicite-1\n"), catalog)
	if err != nil {
		t.Fatal(err)
	}
	report := Coverage(catalog, ledger, map[string]string{"one": "TestOne"})
	if want := []string{"one"}; !reflect.DeepEqual(report.Covered, want) {
		t.Fatalf("Covered = %#v, want %#v", report.Covered, want)
	}
	if want := []string{"two"}; !reflect.DeepEqual(report.Diverged, want) {
		t.Fatalf("Diverged = %#v, want %#v", report.Diverged, want)
	}
	if want := []string{"three"}; !reflect.DeepEqual(report.Missing, want) {
		t.Fatalf("Missing = %#v, want %#v", report.Missing, want)
	}
}

func testCatalog(t *testing.T) Catalog {
	t.Helper()
	catalog, err := parseCatalog(strings.NewReader("# total: 2\none\talpha\t1\ntwo\talpha\t2\n"))
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

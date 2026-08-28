package parity

import (
	"strings"
	"testing"
)

func TestBindingsRejectsUnboundInvariant(t *testing.T) {
	bindings := newBindings(t, "TestOwner", map[string]string{"one": "TestOwner/one"}, Ledger{})
	assertBindingProblems(t, bindings, "no invariants bound", `unbound invariant "one"`)
}

func TestBindingsRejectsSharedAssertion(t *testing.T) {
	bindings := newBindings(t, "TestOwner", map[string]string{"one": "TestOwner/one", "two": "TestOwner/two"}, Ledger{})
	assert := func(*testing.T) {}
	bindings.Bind("one", assert)
	bindings.Bind("two", assert)
	assertBindingProblems(t, bindings, "shared assertion one, two")
}

func TestBindingsRejectsUnknownAndDuplicateNames(t *testing.T) {
	bindings := newBindings(t, "TestOwner", map[string]string{"one": "TestOwner/one"}, Ledger{})
	bindings.Bind("missing", func(*testing.T) {})
	bindings.Bind("one", func(*testing.T) {})
	bindings.Bind("one", func(*testing.T) {})
	assertBindingProblems(t, bindings, `unknown invariant "missing"`, `duplicate binding "one"`)
}

func TestBindingsRejectsEmptyOwner(t *testing.T) {
	assertBindingProblems(t, newBindings(t, "TestOwner", nil, Ledger{}), "no invariants bound")
}

func TestBindingsSkipsLedgerShadowedInvariant(t *testing.T) {
	catalog := Catalog{ByName: map[string]Invariant{"one": {Name: "one", Domain: "domain"}}}
	ledger := Ledger{catalog: catalog, byTarget: map[string]Divergence{"one": {Target: "one"}}}
	bindings := newBindings(t, "TestOwner", map[string]string{"one": "TestOwner/one"}, ledger)
	if len(bindings.owned) != 0 {
		t.Fatalf("owned = %#v, want no ledger-shadowed invariant", bindings.owned)
	}
	assertBindingProblems(t, bindings, "no invariants bound")
}

func TestBindingsRunUsesNamedSubtest(t *testing.T) {
	bindings := newBindings(t, "TestOwner", map[string]string{"one": "TestOwner/one"}, Ledger{})
	ran := false
	bindings.Bind("one", func(t *testing.T) { ran = true })
	bindings.Run()
	if !ran {
		t.Fatal("bound assertion did not run")
	}
}

func TestPendingDomains(t *testing.T) {
	pending, err := PendingDomains()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("PendingDomains() = %#v, want no pending blanket rows", pending)
	}
}

func TestPendingDomainsFileName(t *testing.T) {
	if !strings.HasSuffix(pendingFile, ".tsv") {
		t.Fatalf("pending file = %q", pendingFile)
	}
}

func assertBindingProblems(t *testing.T, bindings *Bindings, wants ...string) {
	t.Helper()
	problems := strings.Join(bindings.problems(), "\n")
	for _, want := range wants {
		if !strings.Contains(problems, want) {
			t.Errorf("problems = %q, want %q", problems, want)
		}
	}
}

package decomp

import (
	"reflect"
	"strings"
	"testing"
)

func TestInterimPhrasesReturnsCopy(t *testing.T) {
	want := []string{"until then", "placeholder", "for now pass nil"}
	got := InterimPhrases()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InterimPhrases() = %#v, want %#v", got, want)
	}
	got[0] = "changed"
	if reflect.DeepEqual(InterimPhrases(), got) {
		t.Fatal("InterimPhrases() returned shared storage")
	}
}

func TestDifficultyLabelsPreservesOrder(t *testing.T) {
	c := Child{Labels: []string{"x", "difficulty:high", "difficulty:low", "y"}}
	want := []string{"difficulty:high", "difficulty:low"}
	if got := DifficultyLabels(c); !reflect.DeepEqual(got, want) {
		t.Fatalf("DifficultyLabels() = %#v, want %#v", got, want)
	}
}

func TestCheckChildRuleOrderAndDetails(t *testing.T) {
	description := strings.Replace(
		validRulesDescription(strings.Repeat("word ", MaxProseWords-9)),
		"Rules are deterministic.", "Rules are deterministic. until then", 1,
	)
	child := Child{
		ID:          "child-1",
		Description: description,
		Design:      strings.Repeat("word ", MaxDesignWords+1),
		Acceptance:  strings.Repeat("word ", MaxAcceptanceWords+1),
		Labels:      []string{"difficulty:high"},
	}

	got := CheckChild(child)
	wantRules := []Rule{RuleCapProse, RuleCapAcceptance, RuleCapDesign, RuleTierMismatch, RuleInterimValue}
	if len(got) != len(wantRules) {
		t.Fatalf("CheckChild() = %#v, want %d violations", got, len(wantRules))
	}
	for i, violation := range got {
		if violation.ID != child.ID || violation.Rule != wantRules[i] || violation.Detail == "" {
			t.Errorf("violation %d = %#v, want id %q rule %q and detail", i, violation, child.ID, wantRules[i])
		}
	}
	if got[0].Detail != "words=401 maximum=400" {
		t.Errorf("cap-prose detail = %q", got[0].Detail)
	}
	if got[1].Detail != "words=81 maximum=80" {
		t.Errorf("cap-acceptance detail = %q", got[1].Detail)
	}
	if got[2].Detail != "words=151 maximum=150" {
		t.Errorf("cap-design detail = %q", got[2].Detail)
	}
	if got[4].Detail != "matched=until then" {
		t.Errorf("interim detail = %q", got[4].Detail)
	}
}

func TestCheckChildSchemaAndAcceptance(t *testing.T) {
	child := Child{ID: "child-2", Description: "not a contract", Acceptance: "line\nwrapped"}
	got := CheckChild(child)
	if len(got) != 3 {
		t.Fatalf("CheckChild() = %#v, want schema, tier and acceptance violations", got)
	}
	if got[0].Rule != RuleSchemaSection || got[1].Rule != RuleCapAcceptance || got[2].Rule != RuleTierMismatch {
		t.Fatalf("CheckChild() rules = %#v, want schema, acceptance, tier", got)
	}
	if got[1].Detail != `not plain ASCII one line: "line\nwrapped"` {
		t.Errorf("acceptance detail = %q", got[1].Detail)
	}
}

func TestCheckChildClean(t *testing.T) {
	child := Child{
		ID:          "child-3",
		Description: validRulesDescription("short prose"),
		Design:      "small design",
		Acceptance:  "one line",
		Labels:      []string{"difficulty:low"},
	}
	if got := CheckChild(child); got != nil {
		t.Fatalf("CheckChild() = %#v, want nil", got)
	}
}

func validRulesDescription(scope string) string {
	return `# Scope
` + scope + `

# Files
internal/decomp/rules.go

# Contract
Rules are deterministic.

# Invariants
Pure checks only.

# Non-goals
No graph checks.

# MACHINE
provides:
consumes:
files: internal/decomp/rules.go, internal/decomp/rules_test.go
tier: low`
}

package decomp

import (
	"reflect"
	"testing"
)

const validDescription = `# Scope
Build parser.

# Files
internal/decomp/decomp.go

# Contract
Terms stay here.

# Invariants
Pure parser only.

# Non-goals
No repair.

# MACHINE
provides: decomp.Parse, decomp.Contract
consumes: bd.Child
files: internal/decomp/decomp.go, internal/decomp/decomp_test.go
tier: high (shared parser)`

func TestVocabularyReturnsCopies(t *testing.T) {
	headings := Headings()
	if want := []string{"Scope", "Files", "Contract", "Invariants", "Non-goals", "MACHINE"}; !reflect.DeepEqual(headings, want) {
		t.Fatalf("Headings() = %#v, want %#v", headings, want)
	}
	headings[0] = "changed"
	if Headings()[0] != "Scope" {
		t.Fatal("Headings() returned shared vocabulary")
	}

	keys := MachineKeys()
	if want := []string{"provides", "consumes", "files", "tier"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("MachineKeys() = %#v, want %#v", keys, want)
	}
	keys[0] = "changed"
	if MachineKeys()[0] != "provides" {
		t.Fatal("MachineKeys() returned shared vocabulary")
	}

	wantRules := []Rule{RuleCountBudget, RuleUnprovidedSymbol, RuleCapProse, RuleCapAcceptance, RuleCapDesign, RuleSchemaSection, RuleTierMismatch, RuleInterimValue, RuleGraphCycle, RuleGraphWidth}
	if got := Rules(); !reflect.DeepEqual(got, wantRules) {
		t.Fatalf("Rules() = %#v, want %#v", got, wantRules)
	}
}

func TestSectionsPreserveInnerNewlines(t *testing.T) {
	got := Sections("intro\n# Scope\nfirst\n\nsecond\n# Files\nfile\n")
	want := []Section{{Name: "Scope", Body: "first\n\nsecond"}, {Name: "Files", Body: "file"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Sections() = %#v, want %#v", got, want)
	}
}

func TestParseValidContract(t *testing.T) {
	got := Parse(validDescription)
	want := Contract{
		Scope: "Build parser.", Files: "internal/decomp/decomp.go", Terms: "Terms stay here.",
		Invariants: "Pure parser only.", NonGoals: "No repair.",
		Provides: []string{"decomp.Parse", "decomp.Contract"}, Consumes: []string{"bd.Child"},
		MachineFiles: []string{"internal/decomp/decomp.go", "internal/decomp/decomp_test.go"}, Tier: TierHigh,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestParseMalformedIsZero(t *testing.T) {
	for _, desc := range []string{"", "# Scope\nonly", validDescription + "\n# Extra\nx", "# Scope\n# Files\n# Contract\n# Invariants\n# Non-goals\n# MACHINE\nprovides:\nconsumes:\nfiles:\ntier: highest"} {
		if got := Parse(desc); !reflect.DeepEqual(got, Contract{}) {
			t.Fatalf("Parse(%q) = %#v, want zero Contract", desc, got)
		}
	}
}

func TestWordsAndProseWords(t *testing.T) {
	if got, want := Words("one_two 34! café"), 3; got != want {
		t.Fatalf("Words() = %d, want %d", got, want)
	}
	if got, want := ProseWords(validDescription), 11; got != want {
		t.Fatalf("ProseWords() = %d, want %d", got, want)
	}
}

func TestSchemaDetail(t *testing.T) {
	for name, test := range map[string]struct {
		desc  string
		parts []string
	}{
		"empty":    {"", []string{"got empty", "want non-empty"}},
		"headings": {"# Scope\n# Files\n# Contract\n# Invariants\n# MACHINE\nprovides:\nconsumes:\nfiles:\ntier: low", []string{"headings: got", "want Scope, Files, Contract, Invariants, Non-goals, MACHINE"}},
		"keys":     {"# Scope\n# Files\n# Contract\n# Invariants\n# Non-goals\n# MACHINE\nprovides:\nconsumes:\noutputs:\ntier: low", []string{"MACHINE keys: got", "want provides, consumes, files, tier"}},
		"tier":     {"# Scope\n# Files\n# Contract\n# Invariants\n# Non-goals\n# MACHINE\nprovides:\nconsumes:\nfiles:\ntier: medium", []string{"tier: got medium", "want low or high"}},
	} {
		t.Run(name, func(t *testing.T) {
			detail := SchemaDetail(test.desc)
			for _, part := range test.parts {
				if !contains(detail, part) {
					t.Fatalf("SchemaDetail() = %q, missing %q", detail, part)
				}
			}
		})
	}
	if got := SchemaDetail(validDescription); got != "" {
		t.Fatalf("SchemaDetail(valid) = %q, want empty", got)
	}
}

func TestImplementationFile(t *testing.T) {
	for path, want := range map[string]bool{
		"internal/decomp/decomp.go":      true,
		"internal/decomp/decomp_test.go": false,
		"testdata/child.md":              false,
		"internal/probes/check.go":       false,
		"README.md":                      false,
		"README":                         false,
		"Makefile":                       false,
		"harness/check.sh":               false,
	} {
		if got := ImplementationFile(path); got != want {
			t.Errorf("ImplementationFile(%q) = %t, want %t", path, got, want)
		}
	}
}

func TestDeriveTier(t *testing.T) {
	for name, test := range map[string]struct {
		contract Contract
		want     Tier
	}{
		"one implementation no provides": {Contract{MachineFiles: []string{"impl.go", "impl_test.go"}}, TierLow},
		"duplicates count once":          {Contract{MachineFiles: []string{"impl.go", "impl.go"}}, TierLow},
		"provided symbol":                {Contract{MachineFiles: []string{"impl.go"}, Provides: []string{"pkg.Symbol"}}, TierHigh},
		"multiple implementations":       {Contract{MachineFiles: []string{"one.go", "two.go"}}, TierHigh},
		"no implementation":              {Contract{MachineFiles: []string{"README.md"}}, TierHigh},
	} {
		t.Run(name, func(t *testing.T) {
			if got := DeriveTier(test.contract); got != test.want {
				t.Fatalf("DeriveTier(%#v) = %q, want %q", test.contract, got, test.want)
			}
		})
	}
}

func TestViolationString(t *testing.T) {
	v := Violation{ID: "magicite-1", Rule: RuleCapProse, Detail: "too many words"}
	if got, want := v.String(), "magicite-1 cap-prose too many words"; got != want {
		t.Fatalf("Violation.String() = %q, want %q", got, want)
	}
}

func contains(text, part string) bool {
	for i := 0; i+len(part) <= len(text); i++ {
		if text[i:i+len(part)] == part {
			return true
		}
	}
	return false
}

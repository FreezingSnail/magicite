package decomp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type checkFixture struct {
	Children []Child `json:"children"`
	Expect   []struct {
		ID   string `json:"id"`
		Rule Rule   `json:"rule"`
	} `json:"expect"`
}

func TestCheckFixtures(t *testing.T) {
	paths, err := filepath.Glob("testdata/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no decomposition fixtures")
	}
	sort.Strings(paths)

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var fixture checkFixture
			if err := json.Unmarshal(data, &fixture); err != nil {
				t.Fatal(err)
			}

			got := Check(fixture.Children)
			pairs := make([]struct {
				ID   string
				Rule Rule
			}, len(got))
			for i, violation := range got {
				pairs[i] = struct {
					ID   string
					Rule Rule
				}{violation.ID, violation.Rule}
			}
			want := make([]struct {
				ID   string
				Rule Rule
			}, len(fixture.Expect))
			for i, violation := range fixture.Expect {
				want[i] = struct {
					ID   string
					Rule Rule
				}{violation.ID, violation.Rule}
			}
			if !reflect.DeepEqual(pairs, want) {
				t.Fatalf("Check() pairs = %#v, want %#v", pairs, want)
			}
		})
	}
}

func TestCheckCopiesChildrenAndOrdersOutput(t *testing.T) {
	children := []Child{
		{ID: "z", Description: validRulesDescription("x"), Labels: []string{"difficulty:high"}},
		{ID: "a", Description: validRulesDescription("x"), Labels: []string{"difficulty:high"}},
	}
	original := append([]Child(nil), children...)
	original[0].Labels = append([]string(nil), children[0].Labels...)
	original[1].Labels = append([]string(nil), children[1].Labels...)

	got := Check(children)
	if !reflect.DeepEqual(children, original) {
		t.Fatalf("Check() mutated children: %#v, want %#v", children, original)
	}
	want := []Violation{
		{ID: "a", Rule: RuleCountBudget},
		{ID: "a", Rule: RuleGraphWidth},
		{ID: "a", Rule: RuleTierMismatch},
		{ID: "z", Rule: RuleTierMismatch},
	}
	if len(got) != len(want) {
		t.Fatalf("Check() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i].ID != want[i].ID || got[i].Rule != want[i].Rule {
			t.Errorf("violation %d = %#v, want ID %q rule %q", i, got[i], want[i].ID, want[i].Rule)
		}
	}
	if Check(nil) != nil || Check([]Child{}) != nil {
		t.Fatal("Check() empty input must return nil")
	}
}

func TestFormat(t *testing.T) {
	violations := []Violation{
		{ID: "a", Rule: RuleCapProse, Detail: "words=401 maximum=400"},
		{ID: "b", Rule: RuleGraphCycle, Detail: "child b is in a dependency cycle"},
	}
	if got, want := Format(violations), "a cap-prose words=401 maximum=400\nb graph-cycle child b is in a dependency cycle\n"; got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
	if got := Format(nil); got != "" {
		t.Fatalf("Format(nil) = %q, want empty", got)
	}
}

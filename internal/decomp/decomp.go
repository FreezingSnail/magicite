// Package decomp parses and validates decomposition child contracts.
package decomp

import (
	"path/filepath"
	"strings"
)

type Tier string

const (
	TierUnknown Tier = ""
	TierLow     Tier = "low"
	TierHigh    Tier = "high"
)

type Child struct {
	ID, Description, Design, Acceptance string
	Labels, Deps                        []string
	DecompVerdict                       bool
}

type Section struct {
	Name, Body string
}

type Contract struct {
	Scope, Files, Terms, Invariants, NonGoals string
	Provides, Consumes, MachineFiles          []string
	Tier                                      Tier
}

type Rule string

const (
	RuleCountBudget      Rule = "count-budget"
	RuleUnprovidedSymbol Rule = "unprovided-symbol"
	RuleCapProse         Rule = "cap-prose"
	RuleCapAcceptance    Rule = "cap-acceptance"
	RuleCapDesign        Rule = "cap-design"
	RuleSchemaSection    Rule = "schema-section"
	RuleTierMismatch     Rule = "tier-mismatch"
	RuleInterimValue     Rule = "interim-value"
	RuleGraphCycle       Rule = "graph-cycle"
	RuleGraphWidth       Rule = "graph-width"
)

type Violation struct {
	ID     string
	Rule   Rule
	Detail string
}

func (v Violation) String() string {
	return v.ID + " " + string(v.Rule) + " " + v.Detail
}

var headings = []string{"Scope", "Files", "Contract", "Invariants", "Non-goals", "MACHINE"}
var machineKeys = []string{"provides", "consumes", "files", "tier"}
var rules = []Rule{
	RuleCountBudget,
	RuleUnprovidedSymbol,
	RuleCapProse,
	RuleCapAcceptance,
	RuleCapDesign,
	RuleSchemaSection,
	RuleTierMismatch,
	RuleInterimValue,
	RuleGraphCycle,
	RuleGraphWidth,
}

func Headings() []string {
	return append([]string(nil), headings...)
}

func MachineKeys() []string {
	return append([]string(nil), machineKeys...)
}

func Rules() []Rule {
	return append([]Rule(nil), rules...)
}

// Sections returns every level-one section in desc, preserving body newlines.
func Sections(desc string) []Section {
	var sections []Section
	var name string
	var body []string

	flush := func() {
		if name == "" {
			return
		}
		sections = append(sections, Section{Name: name, Body: strings.TrimSpace(strings.Join(body, "\n"))})
	}

	for _, line := range strings.Split(desc, "\n") {
		heading := strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(heading, "# ") && len(heading) > len("# ") {
			flush()
			name = heading[len("# "):]
			body = nil
			continue
		}
		if name != "" {
			body = append(body, line)
		}
	}
	flush()
	return sections
}

func Parse(desc string) Contract {
	if SchemaDetail(desc) != "" {
		return Contract{}
	}

	sections := Sections(desc)
	c := Contract{
		Scope:      sections[0].Body,
		Files:      sections[1].Body,
		Terms:      sections[2].Body,
		Invariants: sections[3].Body,
		NonGoals:   sections[4].Body,
	}
	pairs := machinePairs(sections[5].Body)
	c.Provides = tokens(pairs["provides"])
	c.Consumes = tokens(pairs["consumes"])
	c.MachineFiles = tokens(pairs["files"])
	c.Tier = tier(pairs["tier"])
	return c
}

func Words(text string) int {
	count := 0
	inWord := false
	for i := 0; i < len(text); i++ {
		word := asciiWord(text[i])
		if word && !inWord {
			count++
		}
		inWord = word
	}
	return count
}

func ProseWords(desc string) int {
	c := Parse(desc)
	return Words(c.Scope) + Words(c.Files) + Words(c.Invariants) + Words(c.NonGoals)
}

func SchemaDetail(desc string) string {
	if strings.TrimSpace(desc) == "" {
		return "description: got empty, want non-empty description"
	}

	sections := Sections(desc)
	observedHeadings := make([]string, len(sections))
	for i, section := range sections {
		observedHeadings[i] = section.Name
	}
	if !equalStrings(observedHeadings, headings) {
		return "headings: got " + strings.Join(observedHeadings, ", ") + ", want " + strings.Join(headings, ", ")
	}

	pairs, observedKeys := machinePairsWithKeys(sections[5].Body)
	if !equalStrings(observedKeys, machineKeys) {
		return "MACHINE keys: got " + strings.Join(observedKeys, ", ") + ", want " + strings.Join(machineKeys, ", ")
	}
	if tier(pairs["tier"]) == TierUnknown {
		return "tier: got " + pairs["tier"] + ", want low or high"
	}
	return ""
}

func ImplementationFile(path string) bool {
	path = strings.ReplaceAll(path, "\\", "/")
	for _, segment := range strings.Split(path, "/") {
		if segment == "testdata" || segment == "probes" {
			return false
		}
	}

	base := filepath.Base(path)
	if strings.HasSuffix(base, "_test.go") || base == "Makefile" || base == "check.sh" {
		return false
	}
	return strings.TrimSuffix(base, filepath.Ext(base)) != "README"
}

func DeriveTier(c Contract) Tier {
	files := make(map[string]struct{})
	for _, path := range c.MachineFiles {
		if ImplementationFile(path) {
			files[path] = struct{}{}
		}
	}
	if len(files) == 1 && len(c.Provides) == 0 {
		return TierLow
	}
	return TierHigh
}

func machinePairs(body string) map[string]string {
	pairs, _ := machinePairsWithKeys(body)
	return pairs
}

func machinePairsWithKeys(body string) (map[string]string, []string) {
	pairs := make(map[string]string)
	var keys []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSuffix(line, "\r")
		key, value, found := strings.Cut(line, ":")
		if !found {
			if strings.TrimSpace(line) != "" {
				keys = append(keys, strings.TrimSpace(line))
			}
			continue
		}
		key = strings.TrimSpace(key)
		keys = append(keys, key)
		pairs[key] = strings.TrimSpace(value)
	}
	return pairs, keys
}

func tokens(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ','
	})
}

func tier(value string) Tier {
	word := leadingWord(value)
	switch word {
	case "low":
		return TierLow
	case "high":
		return TierHigh
	default:
		return TierUnknown
	}
}

func leadingWord(value string) string {
	value = strings.TrimSpace(value)
	end := 0
	for end < len(value) && asciiWord(value[end]) {
		end++
	}
	return value[:end]
}

func asciiWord(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_'
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

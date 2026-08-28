package parity

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const pendingFile = "binding-pending.tsv"

// Bindings owns the named parity assertions beneath one Go test.
type Bindings struct {
	t       *testing.T
	owner   string
	owned   map[string]struct{}
	bound   map[string]func(*testing.T)
	invalid []string
}

// NewBindings collects counterparts owned by OWNER which are not justified by
// the divergence ledger. Each owned invariant must be bound exactly once.
func NewBindings(t *testing.T, owner string) *Bindings {
	t.Helper()
	ledger, err := LoadLedger()
	if err != nil {
		t.Fatal(err)
	}
	counterparts := SubstrateCounterparts()
	for name, counterpart := range OrchestrationCounterparts() {
		if prior, duplicate := counterparts[name]; duplicate {
			t.Fatalf("duplicate counterpart for %q: %q and %q", name, prior, counterpart)
		}
		counterparts[name] = counterpart
	}
	return newBindings(t, owner, counterparts, ledger)
}

func newBindings(t *testing.T, owner string, counterparts map[string]string, ledger Ledger) *Bindings {
	t.Helper()
	bindings := &Bindings{t: t, owner: owner, owned: make(map[string]struct{}), bound: make(map[string]func(*testing.T))}
	prefix := owner + "/"
	for name, counterpart := range counterparts {
		if !strings.HasPrefix(counterpart, prefix) {
			continue
		}
		if justified, _ := ledger.Justified(name); !justified {
			bindings.owned[name] = struct{}{}
		}
	}
	return bindings
}

// Bind attaches ASSERT to one owned invariant. Invalid names and duplicates are
// retained as Run failures so a caller receives every binding error at once.
func (bindings *Bindings) Bind(name string, assert func(*testing.T)) {
	bindings.t.Helper()
	if _, owned := bindings.owned[name]; !owned {
		bindings.invalid = append(bindings.invalid, fmt.Sprintf("unknown invariant %q", name))
		return
	}
	if _, duplicate := bindings.bound[name]; duplicate {
		bindings.invalid = append(bindings.invalid, fmt.Sprintf("duplicate binding %q", name))
		return
	}
	if assert == nil {
		bindings.invalid = append(bindings.invalid, fmt.Sprintf("nil assertion %q", name))
		return
	}
	bindings.bound[name] = assert
}

// Run reports incomplete or shared assertions, then runs every bound invariant
// as its own named subtest.
func (bindings *Bindings) Run() {
	bindings.t.Helper()
	for _, problem := range bindings.problems() {
		bindings.t.Error(problem)
	}
	for _, name := range bindings.names() {
		if assert, bound := bindings.bound[name]; bound {
			bindings.t.Run(name, assert)
		}
	}
}

func (bindings *Bindings) problems() []string {
	problems := append([]string(nil), bindings.invalid...)
	if len(bindings.bound) == 0 {
		problems = append(problems, "no invariants bound")
	}
	for _, name := range bindings.names() {
		if _, bound := bindings.bound[name]; !bound {
			problems = append(problems, fmt.Sprintf("unbound invariant %q", name))
		}
	}
	byPointer := make(map[uintptr][]string)
	for name, assert := range bindings.bound {
		byPointer[reflect.ValueOf(assert).Pointer()] = append(byPointer[reflect.ValueOf(assert).Pointer()], name)
	}
	for _, duplicates := range byPointer {
		if len(duplicates) > 1 {
			sort.Strings(duplicates)
			problems = append(problems, fmt.Sprintf("shared assertion %s", strings.Join(duplicates, ", ")))
		}
	}
	sort.Strings(problems)
	return problems
}

func (bindings *Bindings) names() []string {
	names := make([]string, 0, len(bindings.owned))
	for name := range bindings.owned {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// PendingDomains reads domain-wide divergences still awaiting conversion.
func PendingDomains() (map[string]string, error) {
	file, err := os.Open(dataPath(pendingFile))
	if err != nil {
		return nil, fmt.Errorf("open binding pending list: %w", err)
	}
	defer file.Close()

	pending := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSuffix(scanner.Text(), "\r")
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) != 2 || fields[0] == "" || fields[1] == "" || strings.TrimSpace(fields[0]) != fields[0] || strings.TrimSpace(fields[1]) != fields[1] {
			return nil, fmt.Errorf("binding pending line %d: malformed row", line)
		}
		if _, duplicate := pending[fields[0]]; duplicate {
			return nil, fmt.Errorf("binding pending line %d: duplicate domain %q", line, fields[0])
		}
		pending[fields[0]] = fields[1]
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read binding pending list: %w", err)
	}
	return pending, nil
}

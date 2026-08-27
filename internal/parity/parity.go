// Package parity inventories maduin behavior and records intentional gaps.
package parity

import (
	"path/filepath"
	"runtime"
)

// dataPath resolves checked-in parity fixtures independently of the caller's directory.
func dataPath(name string) string {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("testdata", name)
	}
	return filepath.Join(filepath.Dir(source), "testdata", name)
}

// Invariant identifies one ERT behavior in the fixed maduin snapshot.
type Invariant struct {
	Name   string
	Domain string
	Line   int
}

// Catalog is the immutable-in-practice snapshot plus indexes for parity checks.
type Catalog struct {
	Entries  []Invariant
	ByName   map[string]Invariant
	ByDomain map[string][]Invariant
}

// Divergence records behavior magicite intentionally does not reproduce.
type Divergence struct {
	Target string
	Reason string
	Owner  string
}

// Ledger records exact-invariant and domain-wide intentional divergences.
type Ledger struct {
	entries  []Divergence
	byTarget map[string]Divergence
	catalog  Catalog
}

// Report partitions catalog invariant names by their current parity state.
type Report struct {
	Covered  []string
	Diverged []string
	Missing  []string
}

// Coverage reports each invariant as replayed, deliberately diverged, or missing.
func Coverage(catalog Catalog, ledger Ledger, counterparts map[string]string) Report {
	var report Report
	for _, invariant := range catalog.Entries {
		if _, ok := counterparts[invariant.Name]; ok {
			report.Covered = append(report.Covered, invariant.Name)
			continue
		}
		if ok, _ := ledger.Justified(invariant.Name); ok {
			report.Diverged = append(report.Diverged, invariant.Name)
			continue
		}
		report.Missing = append(report.Missing, invariant.Name)
	}
	return report
}

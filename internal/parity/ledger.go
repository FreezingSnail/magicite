package parity

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	ledgerFile            = "divergences.tsv"
	goldenArgvTraceTarget = "golden-argv-traces"
)

// LoadLedger reads deliberate divergences and the recorded golden-trace limitation.
func LoadLedger() (Ledger, error) {
	catalog, err := LoadCatalog()
	if err != nil {
		return Ledger{}, err
	}

	path := dataPath(ledgerFile)
	file, err := os.Open(path)
	if err != nil {
		return Ledger{}, fmt.Errorf("open divergence ledger %q: %w", path, err)
	}
	defer file.Close()

	return parseLedger(file, catalog)
}

func parseLedger(reader io.Reader, catalog Catalog) (Ledger, error) {
	ledger := Ledger{byTarget: make(map[string]Divergence), catalog: catalog}
	scanner := bufio.NewScanner(reader)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 || fields[0] == "" || fields[1] == "" || fields[2] == "" {
			return Ledger{}, fmt.Errorf("ledger line %d: malformed row", lineNumber)
		}
		for _, field := range fields {
			if strings.TrimSpace(field) != field {
				return Ledger{}, fmt.Errorf("ledger line %d: malformed row", lineNumber)
			}
		}
		if fields[0] != goldenArgvTraceTarget {
			if _, invariant := catalog.ByName[fields[0]]; !invariant {
				if _, domain := catalog.ByDomain[fields[0]]; !domain {
					return Ledger{}, fmt.Errorf("ledger line %d: unknown invariant, domain, or limitation %q", lineNumber, fields[0])
				}
			}
		}
		if _, duplicate := ledger.byTarget[fields[0]]; duplicate {
			return Ledger{}, fmt.Errorf("ledger line %d: duplicate target %q", lineNumber, fields[0])
		}
		divergence := Divergence{Target: fields[0], Reason: fields[1], Owner: fields[2]}
		ledger.entries = append(ledger.entries, divergence)
		ledger.byTarget[divergence.Target] = divergence
	}
	if err := scanner.Err(); err != nil {
		return Ledger{}, fmt.Errorf("read ledger: %w", err)
	}
	return ledger, nil
}

// Justified reports the exact or domain-wide divergence reason for an invariant.
func (ledger Ledger) Justified(name string) (bool, string) {
	if divergence, ok := ledger.byTarget[name]; ok {
		return true, divergence.Reason
	}
	invariant, ok := ledger.catalog.ByName[name]
	if !ok {
		return false, ""
	}
	divergence, ok := ledger.byTarget[invariant.Domain]
	if !ok {
		return false, ""
	}
	return true, divergence.Reason
}

// Reasons returns a copy of recorded divergences in ledger-file order.
func (ledger Ledger) Reasons() []Divergence {
	return append([]Divergence(nil), ledger.entries...)
}

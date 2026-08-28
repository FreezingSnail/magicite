package parity

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const catalogFile = "maduin-invariants.tsv"

// LoadCatalog reads the checked-in maduin invariant snapshot without invoking maduin.
func LoadCatalog() (Catalog, error) {
	path := dataPath(catalogFile)
	file, err := os.Open(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("open catalog %q: %w", path, err)
	}
	defer file.Close()

	return parseCatalog(file)
}

func parseCatalog(reader io.Reader) (Catalog, error) {
	catalog := Catalog{
		ByName:   make(map[string]Invariant),
		ByDomain: make(map[string][]Invariant),
	}
	var declaredTotal *int

	scanner := bufio.NewScanner(reader)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if strings.HasPrefix(line, "#") {
			switch {
			case strings.HasPrefix(line, "# revision:"):
				if catalog.Revision != "" {
					return Catalog{}, fmt.Errorf("catalog line %d: duplicate revision header", lineNumber)
				}
				revision := strings.TrimSpace(strings.TrimPrefix(line, "# revision:"))
				if !validRevision(revision) {
					return Catalog{}, fmt.Errorf("catalog line %d: invalid revision header", lineNumber)
				}
				catalog.Revision = revision
			case strings.HasPrefix(line, "# total:"):
				if declaredTotal != nil {
					return Catalog{}, fmt.Errorf("catalog line %d: duplicate total header", lineNumber)
				}
				total, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "# total:")))
				if err != nil || total < 0 {
					return Catalog{}, fmt.Errorf("catalog line %d: invalid total header", lineNumber)
				}
				declaredTotal = &total
			}
			continue
		}
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) != 3 || fields[0] == "" || fields[1] == "" || fields[2] == "" {
			return Catalog{}, fmt.Errorf("catalog line %d: malformed row", lineNumber)
		}
		if strings.TrimSpace(fields[0]) != fields[0] || strings.TrimSpace(fields[1]) != fields[1] {
			return Catalog{}, fmt.Errorf("catalog line %d: malformed row", lineNumber)
		}
		sourceLine, err := strconv.Atoi(fields[2])
		if err != nil || sourceLine < 1 {
			return Catalog{}, fmt.Errorf("catalog line %d: invalid source line", lineNumber)
		}

		invariant := Invariant{Name: fields[0], Domain: fields[1], Line: sourceLine}
		if _, exists := catalog.ByName[invariant.Name]; exists {
			return Catalog{}, fmt.Errorf("catalog line %d: duplicate invariant %q", lineNumber, invariant.Name)
		}
		catalog.Entries = append(catalog.Entries, invariant)
		catalog.ByName[invariant.Name] = invariant
		catalog.ByDomain[invariant.Domain] = append(catalog.ByDomain[invariant.Domain], invariant)
	}
	if err := scanner.Err(); err != nil {
		return Catalog{}, fmt.Errorf("read catalog: %w", err)
	}
	if declaredTotal == nil {
		return Catalog{}, fmt.Errorf("catalog: missing total header")
	}
	if catalog.Revision == "" {
		return Catalog{}, fmt.Errorf("catalog: missing revision header")
	}
	if len(catalog.Entries) != *declaredTotal {
		return Catalog{}, fmt.Errorf("catalog: total mismatch: header declares %d rows, found %d", *declaredTotal, len(catalog.Entries))
	}
	return catalog, nil
}

func validRevision(revision string) bool {
	if len(revision) != 40 {
		return false
	}
	for _, char := range revision {
		if char < '0' || char > '9' {
			if char < 'a' || char > 'f' {
				return false
			}
		}
	}
	return true
}

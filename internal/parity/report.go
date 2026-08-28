package parity

import (
	"fmt"
	"io"
	"sort"
)

// DomainCounts counts coverage states for one catalog domain.
type DomainCounts struct {
	Covered  int
	Diverged int
	Missing  int
}

// WriteReport renders report in a stable, line-oriented form.
func WriteReport(w io.Writer, report Report) error {
	if _, err := fmt.Fprintf(w, "covered: %d\ndiverged: %d\nmissing: %d\n", len(report.Covered), len(report.Diverged), len(report.Missing)); err != nil {
		return err
	}

	domains := make([]string, 0, len(report.Domains))
	for domain := range report.Domains {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	for _, domain := range domains {
		counts := report.Domains[domain]
		if _, err := fmt.Fprintf(w, "domain %s: covered=%d diverged=%d missing=%d\n", domain, counts.Covered, counts.Diverged, counts.Missing); err != nil {
			return err
		}
	}

	missing := append([]string(nil), report.Missing...)
	sort.Strings(missing)
	for _, invariant := range missing {
		if _, err := fmt.Fprintf(w, "missing %s\n", invariant); err != nil {
			return err
		}
	}
	return nil
}

package parity

import (
	"bytes"
	"testing"
)

func TestWriteReport(t *testing.T) {
	report := Report{
		Covered:  []string{"covered-one", "covered-two"},
		Diverged: []string{"diverged-one"},
		Missing:  []string{"missing-z", "missing-a"},
		Domains: map[string]DomainCounts{
			"zeta":  {Covered: 1, Missing: 1},
			"alpha": {Covered: 1, Diverged: 1, Missing: 1},
		},
	}

	var output bytes.Buffer
	if err := WriteReport(&output, report); err != nil {
		t.Fatal(err)
	}
	const want = "covered: 2\ndiverged: 1\nmissing: 2\ndomain alpha: covered=1 diverged=1 missing=1\ndomain zeta: covered=1 diverged=0 missing=1\nmissing missing-a\nmissing missing-z\n"
	if got := output.String(); got != want {
		t.Fatalf("WriteReport() = %q, want %q", got, want)
	}
}

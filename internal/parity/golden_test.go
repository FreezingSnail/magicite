package parity

import (
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/testenv"
)

func TestRenderTrace(t *testing.T) {
	entries := []testenv.Entry{
		{Dir: "work/tree", Argv: []string{"fakebd", "show", "one"}},
		{Dir: "work tree", Argv: []string{"fake", "arg with space", "quote's value"}},
	}
	want := "work/tree fakebd show one\n'work tree' fake 'arg with space' 'quote'\\''s value'\n"
	if got := Render(entries); got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
}

func TestNormalizeTrace(t *testing.T) {
	root := "/tmp/magicite-root"
	first := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	second := "0123456789abcdef0123456789abcdef01234567"
	input := strings.Join([]string{
		"/tmp/magicite-root/bin/magicite --commit=" + first,
		"--parent=" + second + " --again=" + first,
		"at 2026-08-27T17:55:07.577-04:00 after 1.25s",
		"keep /tmp/magicite-rooted " + second,
	}, "\n")
	want := strings.Join([]string{
		"{root}/bin/magicite --commit={sha1}",
		"--parent={sha2} --again={sha1}",
		"at {time} after {time}",
		"keep /tmp/magicite-rooted {sha2}",
	}, "\n")
	if got := Normalize(root, input); got != want {
		t.Fatalf("Normalize() = %q, want %q", got, want)
	}
}

func TestNormalizeLeavesUnrelatedTextByteIdentical(t *testing.T) {
	input := "flag=value\npath=/tmp/other\nversion=1.2.3\n"
	if got := Normalize("/tmp/root", input); got != input {
		t.Fatalf("Normalize() changed unrelated text: %q", got)
	}
}

func TestFirstDifferentLine(t *testing.T) {
	if got, want := firstDifferentLine("one\ntwo\n", "one\nthree\n"), 2; got != want {
		t.Fatalf("firstDifferentLine() = %d, want %d", got, want)
	}
	if got, want := firstDifferentLine("one\n", "one\ntwo\n"), 2; got != want {
		t.Fatalf("firstDifferentLine() length mismatch = %d, want %d", got, want)
	}
}

func TestGoldenTraceProvenance(t *testing.T) {
	want := "Self-recorded magicite regression goldens: they verify magicite argv stability, not argv equivalence with maduin."
	if GoldenTraceProvenance != want {
		t.Fatalf("GoldenTraceProvenance = %q, want %q", GoldenTraceProvenance, want)
	}

	ledger, err := LoadLedger()
	if err != nil {
		t.Fatal(err)
	}
	for _, divergence := range ledger.Reasons() {
		if divergence.Target != goldenArgvTraceTarget {
			continue
		}
		if !strings.Contains(divergence.Reason, "Maduin is not executed") || !strings.Contains(divergence.Reason, "not argv equivalence with maduin") {
			t.Fatalf("golden trace limitation = %q", divergence.Reason)
		}
		return
	}
	t.Fatalf("missing %q ledger entry", goldenArgvTraceTarget)
}

package bd

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestKindString(t *testing.T) {
	for kind, want := range map[Kind]string{
		KindUnknown: "unknown", KindNotFound: "not_found", KindUsage: "usage",
		KindUnavailable: "unavailable", KindFailed: "failed", Kind(99): "unknown",
	} {
		if got := kind.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", kind, got, want)
		}
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name   string
		res    Result
		kind   Kind
		detail string
	}{
		{"success", Result{}, -1, ""},
		{"not found envelope", Result{ExitCode: 1, Stdout: []byte(`{"error":"issue not found: x"}`)}, KindNotFound, "issue not found: x"},
		{"usage exit", Result{ExitCode: 2, Stderr: []byte("bad args")}, KindUsage, "bad args"},
		{"unknown command", Result{ExitCode: 1, Stderr: []byte("Error: unknown command")}, KindUsage, "Error: unknown command"},
		{"usage text", Result{ExitCode: 1, Stderr: []byte("try again\nUsage: bd show")}, KindUsage, "try again Usage: bd show"},
		{"unavailable exit", Result{ExitCode: 127}, KindUnavailable, ""},
		{"unavailable text", Result{ExitCode: 1, Stderr: []byte("bd: command not found")}, KindUnavailable, "bd: command not found"},
		{"failed", Result{ExitCode: 7, Stderr: []byte("  stderr\nsecond "), Stdout: []byte("stdout")}, KindFailed, "stderr second"},
		{"stdout fallback", Result{ExitCode: 7, Stdout: []byte("stdout")}, KindFailed, "stdout"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Classify("show", []string{"x", "--json"}, test.res)
			if test.kind < 0 {
				if err != nil {
					t.Fatalf("Classify() = %v, want nil", err)
				}
				return
			}
			var classified *Error
			if !errors.As(err, &classified) {
				t.Fatalf("Classify() = %T %v, want *Error", err, err)
			}
			if classified.Kind != test.kind || classified.Detail != test.detail {
				t.Fatalf("error = %+v, want kind=%v detail=%q", classified, test.kind, test.detail)
			}
			if got := classified.Error(); strings.ContainsAny(got, "\r\n") {
				t.Fatalf("Error() contains newline: %q", got)
			}
		})
	}
}

func TestClassifyBoundsAndPreservesInvocation(t *testing.T) {
	args := []string{"literal; $(not-a-command)", "--json"}
	long := strings.Repeat("界", 100)
	err := Classify("show", args, Result{ExitCode: 9, Stderr: []byte(long)})
	classified := err.(*Error)
	if len(classified.Detail) > 200 || !utf8Boundary(classified.Detail) {
		t.Fatalf("detail length/boundary = %d %q", len(classified.Detail), classified.Detail)
	}
	if got, want := classified.Error(), "show literal; $(not-a-command) --json: failed (exit 9): "+classified.Detail; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestPredicates(t *testing.T) {
	notFound := Classify("show", nil, Result{ExitCode: 1, Stdout: []byte(`{"error":"missing"}`)})
	wrapped := fmt.Errorf("context: %w", notFound)
	if !IsNotFound(wrapped) || Loggable(wrapped) {
		t.Errorf("predicates for wrapped not-found = %t, %t", IsNotFound(wrapped), Loggable(wrapped))
	}
	for _, err := range []error{nil, errors.New("plain"), &Error{Kind: KindUsage}} {
		if IsNotFound(err) || (err == nil && Loggable(err)) || (err != nil && !Loggable(err)) {
			t.Errorf("predicates for %v = not-found:%t loggable:%t", err, IsNotFound(err), Loggable(err))
		}
	}
}

func utf8Boundary(s string) bool {
	return utf8.ValidString(s)
}

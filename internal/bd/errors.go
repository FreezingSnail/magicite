package bd

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Kind identifies the outcome of a bd invocation.
type Kind int

const (
	KindUnknown Kind = iota
	KindNotFound
	KindUsage
	KindUnavailable
	KindFailed
)

func (k Kind) String() string {
	switch k {
	case KindNotFound:
		return "not_found"
	case KindUsage:
		return "usage"
	case KindUnavailable:
		return "unavailable"
	case KindFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Error is the classified result of a bd invocation.
type Error struct {
	Op       string
	Args     []string
	Kind     Kind
	ExitCode int
	Detail   string
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	command := strings.Join(append([]string{e.Op}, e.Args...), " ")
	return fmt.Sprintf("%s: %s (exit %d): %s", command, e.Kind, e.ExitCode, cleanDetail(e.Detail))
}

// Classify maps one bd result to the shared error taxonomy.
func Classify(op string, args []string, res Result) error {
	if res.ExitCode == 0 {
		return nil
	}

	kind := KindFailed
	detail := outputDetail(res.Stderr, res.Stdout)
	stderr := string(res.Stderr)
	firstLine := strings.TrimSpace(strings.SplitN(stderr, "\n", 2)[0])
	switch {
	case res.ExitCode == 127 || executableMissing(stderr):
		kind = KindUnavailable
	case res.ExitCode == 2 || strings.HasPrefix(firstLine, "Error: unknown") || strings.Contains(stderr, "Usage:"):
		kind = KindUsage
	case res.ExitCode == 1:
		if envelope, ok := DecodeEnvelope(res.Stdout); ok {
			kind = KindNotFound
			detail = cleanDetail(envelope.Error)
		}
	}

	return &Error{Op: op, Args: append([]string(nil), args...), Kind: kind, ExitCode: res.ExitCode, Detail: detail}
}

func executableMissing(stderr string) bool {
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "executable file not found") || strings.Contains(lower, "command not found")
}

func outputDetail(preferred, fallback []byte) string {
	if detail := cleanDetail(string(preferred)); detail != "" {
		return detail
	}
	return cleanDetail(string(fallback))
}

func cleanDetail(detail string) string {
	detail = strings.Join(strings.Fields(detail), " ")
	if len(detail) <= 200 {
		return detail
	}
	cut := 200
	for cut > 0 && !utf8.RuneStart(detail[cut]) {
		cut--
	}
	return detail[:cut]
}

// IsNotFound reports whether err contains a semantic missing-bead result.
func IsNotFound(err error) bool {
	var classified *Error
	return err != nil && errors.As(err, &classified) && classified != nil && classified.Kind == KindNotFound
}

// Loggable reports whether err should produce a log event.
func Loggable(err error) bool {
	return err != nil && !IsNotFound(err)
}

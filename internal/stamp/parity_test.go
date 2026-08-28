package stamp

import (
	"reflect"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/parity"
)

func TestMaduinStampParity(t *testing.T) {
	bindings := parity.NewBindings(t, "TestMaduinStampParity")
	bindings.Bind("maduin-test-stamp-trailers-order", func(t *testing.T) {
		if got := (Stamp{Model: "m", Task: "t"}).Trailers(); !reflect.DeepEqual(got, []Trailer{{Key: KeyModel, Value: "m"}, {Key: KeyTask, Value: "t"}}) {
			t.Fatalf("Trailers() = %#v", got)
		}
	})
	bindings.Bind("maduin-test-stamp-trailers-omits-empty", func(t *testing.T) {
		if got := (Stamp{}).Trailers(); got == nil || len(got) != 0 {
			t.Fatalf("Trailers() = %#v", got)
		}
	})
	bindings.Bind("maduin-test-stamp-trailers-sanitizes-injection", func(t *testing.T) {
		if got := (Stamp{Task: "a\nMagicite-Model: injected"}).Trailers()[0].Value; got != "a Magicite-Model injected" {
			t.Fatalf("sanitized task = %q", got)
		}
	})
	bindings.Bind("maduin-test-stamp-repo-sanitizes-injection", func(t *testing.T) {
		if got := (Stamp{Repo: "one:two\r\nthree"}).Trailers()[0].Value; got != "one two three" {
			t.Fatalf("sanitized repo = %q", got)
		}
	})
	bindings.Bind("maduin-test-stamp-trailers-accepts-symbols", func(t *testing.T) {
		if got := (Stamp{Agent: "agent_1/-ok"}).Trailers()[0].Value; got != "agent_1/-ok" {
			t.Fatalf("agent = %q", got)
		}
	})
	bindings.Bind("maduin-test-stamp-format-summary", func(t *testing.T) {
		if got := Apply("subject\n", (Stamp{Model: "m", Task: "t"}).Trailers()); !strings.Contains(got, "Magicite-Model: m\nMagicite-Task: t") {
			t.Fatalf("Apply() = %q", got)
		}
	})
	bindings.Bind("maduin-test-stamp-format-partial", func(t *testing.T) {
		if got := Apply("", (Stamp{Task: "t"}).Trailers()); got != "Magicite-Task: t\n" {
			t.Fatalf("Apply() = %q", got)
		}
	})
	bindings.Bind("maduin-test-stamp-exec-command-quotes-values", func(t *testing.T) {
		if got := Apply("subject", []Trailer{{Key: KeyTask, Value: "a b"}}); !strings.Contains(got, "Magicite-Task: a b") {
			t.Fatalf("Apply() = %q", got)
		}
	})
	bindings.Bind("maduin-test-stamp-exec-command-sets-ifexists", func(t *testing.T) {
		if got := Apply("Magicite-Task: old\n", []Trailer{{Key: KeyTask, Value: "new"}}); strings.Count(got, "Magicite-Task:") != 1 || !strings.Contains(got, "new") {
			t.Fatalf("Apply() = %q", got)
		}
	})
	bindings.Bind("maduin-test-stamp-exec-command-empty-nil", func(t *testing.T) {
		if got := Apply("Magicite-Task: old\n", nil); strings.Contains(got, "Magicite-Task") {
			t.Fatalf("Apply() = %q", got)
		}
	})
	bindings.Bind("maduin-test-stamp-parse-roundtrip", func(t *testing.T) {
		trailers := (Stamp{Model: "m", Repo: "r", Task: "t"}).Trailers()
		if got := Parse(Apply("subject", trailers)); !reflect.DeepEqual(got, trailers) {
			t.Fatalf("Parse(Apply()) = %#v, want %#v", got, trailers)
		}
	})
	bindings.Bind("maduin-test-stamp-parse-legacy-without-repo", func(t *testing.T) {
		if got := Parse("Magicite-Task: task\n"); !reflect.DeepEqual(got, []Trailer{{Key: KeyTask, Value: "task"}}) {
			t.Fatalf("Parse() = %#v", got)
		}
	})
	bindings.Bind("maduin-test-stamp-parse-ignores-foreign", func(t *testing.T) {
		if got := Parse("Other: value\nMagicite-Task: task\n"); !reflect.DeepEqual(got, []Trailer{{Key: KeyTask, Value: "task"}}) {
			t.Fatalf("Parse() = %#v", got)
		}
	})
	bindings.Bind("maduin-test-stamp-parse-empty", func(t *testing.T) {
		if got := Parse(""); got == nil || len(got) != 0 {
			t.Fatalf("Parse(empty) = %#v", got)
		}
	})
	bindings.Run()
}

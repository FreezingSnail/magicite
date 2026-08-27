package stamp

import (
	"reflect"
	"testing"
)

func TestKeys(t *testing.T) {
	want := []string{
		"Magicite-Model",
		"Magicite-Backend",
		"Magicite-Difficulty",
		"Magicite-Effort",
		"Magicite-Agent",
		"Magicite-Repo",
		"Magicite-Seat",
		"Magicite-Task",
		"Magicite-Harness",
		"Magicite-Harness-Rev",
	}
	got := Keys()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Keys() = %#v, want %#v", got, want)
	}
	got[0] = "changed"
	if Keys()[0] != want[0] {
		t.Errorf("Keys() exposes vocabulary backing storage")
	}
}

func TestSanitize(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{input: "  alpha\t beta\n gamma\r", want: "alpha beta gamma"},
		{input: "task: one\r\ntask: two", want: "task one task two"},
		{input: " \t\r\n ", want: ""},
		{input: "already clean", want: "already clean"},
	} {
		if got := Sanitize(test.input); got != test.want {
			t.Errorf("Sanitize(%q) = %q, want %q", test.input, got, test.want)
		}
		if got := Sanitize(Sanitize(test.input)); got != Sanitize(test.input) {
			t.Errorf("Sanitize is not idempotent for %q", test.input)
		}
	}
}

func TestTrailers(t *testing.T) {
	stamp := Stamp{
		Model:      " model ",
		Backend:    "backend\nname",
		Difficulty: "",
		Effort:     "effort",
		Agent:      "agent",
		Repo:       "repo",
		Seat:       "seat",
		Task:       "task:42",
		Harness:    "harness",
		HarnessRev: "rev",
	}
	want := []Trailer{
		{Key: KeyModel, Value: "model"},
		{Key: KeyBackend, Value: "backend name"},
		{Key: KeyEffort, Value: "effort"},
		{Key: KeyAgent, Value: "agent"},
		{Key: KeyRepo, Value: "repo"},
		{Key: KeySeat, Value: "seat"},
		{Key: KeyTask, Value: "task 42"},
		{Key: KeyHarness, Value: "harness"},
		{Key: KeyHarnessRev, Value: "rev"},
	}
	if got := stamp.Trailers(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Trailers() = %#v, want %#v", got, want)
	}
}

func TestEmptyStampTrailersIsNonNil(t *testing.T) {
	trailers := (Stamp{}).Trailers()
	if trailers == nil {
		t.Fatal("Trailers() = nil, want empty non-nil slice")
	}
	if len(trailers) != 0 {
		t.Fatalf("Trailers() = %#v, want empty", trailers)
	}
}

package stamp

import (
	"reflect"
	"testing"
)

func TestParseKnownTrailersInOrder(t *testing.T) {
	message := "Subject\n\nMagicite-Task: task  one  \nUnknown: ignored\nMagicite-Model: model\r\nMagicite-Task: second\n"
	want := []Trailer{
		{Key: KeyTask, Value: "task  one"},
		{Key: KeyModel, Value: "model"},
		{Key: KeyTask, Value: "second"},
	}
	if got := Parse(message); !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestApplyCanonicalizesAndPreservesBody(t *testing.T) {
	message := "Subject\r\n\r\nBody\r\n\r\nMagicite-Task: old\nSigned-off-by: Example <example@test>\n\nMagicite-Model: duplicate\n"
	trailers := []Trailer{
		{Key: KeyTask, Value: "new:value\nvalue"},
		{Key: KeyModel, Value: " model "},
	}
	want := "Subject\r\n\r\nBody\r\n\r\nSigned-off-by: Example <example@test>\n\nMagicite-Model: model\nMagicite-Task: new value value\n"
	got := Apply(message, trailers)
	if got != want {
		t.Fatalf("Apply() = %q, want %q", got, want)
	}
	if again := Apply(got, trailers); again != got {
		t.Fatalf("Apply is not idempotent: first %q, second %q", got, again)
	}
	wantParsed := []Trailer{
		{Key: KeyModel, Value: "model"},
		{Key: KeyTask, Value: "new value value"},
	}
	if gotParsed := Parse(got); !reflect.DeepEqual(gotParsed, wantParsed) {
		t.Fatalf("Parse(Apply()) = %#v, want %#v", gotParsed, wantParsed)
	}
}

func TestApplyEmptyTrailersOnlyStripsMagicite(t *testing.T) {
	message := "Subject\n\nMagicite-Task: old\nSigned-off-by: Example\n"
	want := "Subject\n\nSigned-off-by: Example\n"
	if got := Apply(message, nil); got != want {
		t.Fatalf("Apply() = %q, want %q", got, want)
	}
}

func TestApplyIgnoresUnknownAndEmptyTrailers(t *testing.T) {
	message := "Subject\n\nMagicite-Task: old\n"
	if got := Apply(message, []Trailer{{Key: "Magicite-Unknown", Value: "value"}, {Key: KeyTask, Value: " \n:"}}); got != "Subject\n\n" {
		t.Fatalf("Apply() = %q, want %q", got, "Subject\n\n")
	}
}

package bd

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDecodeBeads(t *testing.T) {
	beads, err := DecodeBeads(readFixture(t, "ready.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(beads) != 1 {
		t.Fatalf("beads = %#v", beads)
	}
	bead := beads[0]
	if bead.ID != "magicite-e84.2" || bead.Title != "typed decode" || bead.Priority != 1 || bead.IssueType != "task" {
		t.Fatalf("bead = %#v", bead)
	}
	if bead.AcceptanceCriteria != "decode JSON" || bead.CreatedAt != "2026-08-27T09:00:00Z" || bead.DependencyCount != 1 || bead.DependentCount != 2 || bead.CommentCount != 3 {
		t.Fatalf("bead = %#v", bead)
	}

	show, err := DecodeBeads(readFixture(t, "show.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(show) != 1 || len(show[0].Dependencies) != 1 {
		t.Fatalf("show = %#v", show)
	}
	dependency := show[0].Dependencies[0]
	if dependency.ID != "magicite-e84.1" || dependency.DependencyType != "blocks" || dependency.Status != "closed" {
		t.Fatalf("dependency = %#v", dependency)
	}
}

func TestDecodeBeadsEmptyAndMalformed(t *testing.T) {
	beads, err := DecodeBeads(readFixture(t, "empty.json"))
	if err != nil || beads == nil || len(beads) != 0 {
		t.Fatalf("DecodeBeads(empty) = %#v, %v", beads, err)
	}

	for _, stdout := range [][]byte{nil, readFixture(t, "truncated.json"), []byte(`{"id":"wrong-shape"}`)} {
		beads, err := DecodeBeads(stdout)
		if !errors.Is(err, ErrMalformedOutput) || beads != nil {
			t.Errorf("DecodeBeads(%q) = %#v, %v", stdout, beads, err)
		}
	}

	long := []byte(strings.Repeat("x", 201))
	_, err = DecodeBeads(long)
	if !errors.Is(err, ErrMalformedOutput) || strings.Contains(err.Error(), string(long)) {
		t.Errorf("bounded malformed error = %v", err)
	}
}

func TestDecodeDepsAndIDs(t *testing.T) {
	dependencies, err := DecodeDeps(readFixture(t, "deps.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(dependencies) != 2 || dependencies[0].DependencyType != "blocks" || dependencies[1].DependencyType != "parent-child" {
		t.Fatalf("dependencies = %#v", dependencies)
	}

	ids, err := DecodeIDs([]byte(`[{"id":"one"},{"id":""},{"title":"missing"},{"id":"two"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(ids, ","), "one,two"; got != want {
		t.Errorf("ids = %q, want %q", got, want)
	}

	if _, err := DecodeDeps([]byte(`{}`)); !errors.Is(err, ErrMalformedOutput) {
		t.Errorf("DecodeDeps() error = %v", err)
	}
	if _, err := DecodeIDs([]byte(`null`)); !errors.Is(err, ErrMalformedOutput) {
		t.Errorf("DecodeIDs() error = %v", err)
	}
}

func TestDecodeLabels(t *testing.T) {
	for _, fixture := range []string{"labels-strings.json", "labels-objects.json"} {
		labels, err := DecodeLabels(readFixture(t, fixture))
		if err != nil {
			t.Fatalf("DecodeLabels(%s) error = %v", fixture, err)
		}
		if got, want := strings.Join(labels, ","), "bug,staged"; got != want {
			t.Errorf("DecodeLabels(%s) = %q, want %q", fixture, got, want)
		}
	}

	labels, err := DecodeLabels(readFixture(t, "empty.json"))
	if err != nil || labels == nil || len(labels) != 0 {
		t.Fatalf("DecodeLabels(empty) = %#v, %v", labels, err)
	}
	if _, err := DecodeLabels([]byte(`[1]`)); !errors.Is(err, ErrMalformedOutput) {
		t.Errorf("DecodeLabels() error = %v", err)
	}
}

func TestDecodeEnvelope(t *testing.T) {
	envelope, ok := DecodeEnvelope(readFixture(t, "not-found.json"))
	if !ok || envelope.Error != "issue not found: magicite-missing" || envelope.SchemaVersion != 1 {
		t.Fatalf("DecodeEnvelope() = %#v, %t", envelope, ok)
	}
	for _, stdout := range [][]byte{readFixture(t, "empty.json"), []byte(`{"error":""}`), []byte(`[]`), []byte(`truncated`)} {
		if envelope, ok := DecodeEnvelope(stdout); ok || envelope != (Envelope{}) {
			t.Errorf("DecodeEnvelope(%q) = %#v, %t", stdout, envelope, ok)
		}
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	contents, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

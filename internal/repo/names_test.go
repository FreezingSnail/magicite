package repo

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestRecordsNormalizesDeduplicatesAndSortsByName(t *testing.T) {
	parent := t.TempDir()
	zeta := filepath.Join(parent, "zeta")
	alpha := filepath.Join(parent, "alpha")

	got := Records([]string{zeta, "", zeta + string(filepath.Separator), alpha})
	want := []Repo{
		mustRecord(t, alpha, "alpha"),
		mustRecord(t, zeta, "zeta"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Records() = %#v, want %#v", got, want)
	}
	for _, record := range got {
		if !record.Valid() {
			t.Errorf("Records() returned invalid record: %#v", record)
		}
	}
}

func TestRecordsQualifiesCollisionsAndAssignsDeterministicSuffixes(t *testing.T) {
	parent := t.TempDir()
	roots := []string{
		filepath.Join(parent, "b", "foo"),
		filepath.Join(parent, "a", "foo"),
		filepath.Join(parent, "a-foo"),
	}

	got := Records(roots)
	want := []Repo{
		mustRecord(t, filepath.Join(parent, "a-foo"), "a-foo"),
		mustRecord(t, filepath.Join(parent, "a", "foo"), "a-foo-2"),
		mustRecord(t, filepath.Join(parent, "b", "foo"), "b-foo"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Records() = %#v, want %#v", got, want)
	}

	reversed := append([]string(nil), roots...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	if reversedGot := Records(reversed); !reflect.DeepEqual(reversedGot, got) {
		t.Errorf("Records() changed with discovery order: %#v, want %#v", reversedGot, got)
	}
}

func TestRecordsQualifiesEveryRootSharingBase(t *testing.T) {
	parent := t.TempDir()
	got := Records([]string{
		filepath.Join(parent, "one", "project"),
		filepath.Join(parent, "two", "project"),
	})
	want := []Repo{
		mustRecord(t, filepath.Join(parent, "one", "project"), "one-project"),
		mustRecord(t, filepath.Join(parent, "two", "project"), "two-project"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Records() = %#v, want %#v", got, want)
	}
}

func mustRecord(t *testing.T, root, stem string) Repo {
	t.Helper()
	record, ok := Make(root, stem, stem, "")
	if !ok {
		t.Fatalf("Make(%q, %q) failed", root, stem)
	}
	return record
}

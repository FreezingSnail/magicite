package dispatch

import (
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/repo"
)

func readyRepo(t *testing.T, root, name string) repo.Repo {
	t.Helper()
	record, ok := repo.Make(root, name, name, "main")
	if !ok {
		t.Fatalf("repo.Make(%q) failed", root)
	}
	return record
}

func TestNormalizeReady(t *testing.T) {
	alpha := readyRepo(t, "/alpha", "alpha")
	beta := readyRepo(t, "/beta", "beta")

	tests := []struct {
		name  string
		repo  repo.Repo
		entry ReadyEntry
		want  ReadyEntry
		ok    bool
	}{
		{
			name:  "fills repository and missing priority",
			repo:  alpha,
			entry: ReadyEntry{Task: "one"},
			want:  ReadyEntry{Repo: alpha, Task: "one", Priority: "2"},
			ok:    true,
		},
		{
			name:  "keeps integer priority",
			repo:  alpha,
			entry: ReadyEntry{Repo: alpha, Task: "one", Priority: "07"},
			want:  ReadyEntry{Repo: alpha, Task: "one", Priority: "07"},
			ok:    true,
		},
		{
			name:  "defaults noninteger priority",
			repo:  alpha,
			entry: ReadyEntry{Task: "one", Priority: "high"},
			want:  ReadyEntry{Repo: alpha, Task: "one", Priority: "2"},
			ok:    true,
		},
		{name: "rejects empty task", repo: alpha, entry: ReadyEntry{Priority: "1"}},
		{name: "rejects whitespace task", repo: alpha, entry: ReadyEntry{Task: " \t"}},
		{name: "rejects mismatched repository", repo: alpha, entry: ReadyEntry{Repo: beta, Task: "one"}},
		{name: "rejects invalid repository", repo: repo.Repo{}, entry: ReadyEntry{Task: "one"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := NormalizeReady(test.repo, test.entry)
			if ok != test.ok {
				t.Fatalf("NormalizeReady() ok = %v, want %v", ok, test.ok)
			}
			if test.ok && got != test.want {
				t.Errorf("NormalizeReady() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestMergeReadyPriorityAndRoundRobin(t *testing.T) {
	alpha := readyRepo(t, "/alpha", "alpha")
	beta := readyRepo(t, "/beta", "beta")
	gamma := readyRepo(t, "/gamma", "gamma")
	groups := []RepoReady{
		{Repo: alpha, Entries: []ReadyEntry{
			{Task: "a1", Priority: "1"},
			{Task: "a2", Priority: "1"},
			{Task: "a3", Priority: "2"},
			{Task: "", Priority: "1"},
		}},
		{Repo: beta, Entries: []ReadyEntry{
			{Task: "b1", Priority: "1"},
			{Task: "b2", Priority: "2"},
			{Repo: gamma, Task: "wrong", Priority: "1"},
		}},
		{Repo: gamma, Entries: []ReadyEntry{
			{Task: "c1", Priority: "1"},
			{Task: "c2", Priority: "1"},
		}},
	}

	got := MergeReady(groups)
	want := []ReadyEntry{
		{Repo: alpha, Task: "a1", Priority: "1"},
		{Repo: beta, Task: "b1", Priority: "1"},
		{Repo: gamma, Task: "c1", Priority: "1"},
		{Repo: alpha, Task: "a2", Priority: "1"},
		{Repo: gamma, Task: "c2", Priority: "1"},
		{Repo: alpha, Task: "a3", Priority: "2"},
		{Repo: beta, Task: "b2", Priority: "2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeReady() = %#v, want %#v", got, want)
	}
}

func TestMergeReadyReturnsIndependentSlice(t *testing.T) {
	alpha := readyRepo(t, "/alpha", "alpha")
	groups := []RepoReady{{Repo: alpha, Entries: []ReadyEntry{{Task: "one", Priority: "1"}}}}
	got := MergeReady(groups)
	got[0].Task = "changed"
	if groups[0].Entries[0].Task != "one" {
		t.Fatal("MergeReady() result aliases input")
	}
}

func TestTakeReadyBoundAndCopy(t *testing.T) {
	entries := []ReadyEntry{{Task: "one"}, {Task: "two"}, {Task: "three"}}
	for _, test := range []struct {
		name string
		n    int
		want []ReadyEntry
	}{
		{name: "negative", n: -1, want: []ReadyEntry{}},
		{name: "zero", n: 0, want: []ReadyEntry{}},
		{name: "bounded", n: 2, want: entries[:2]},
		{name: "larger", n: 4, want: entries},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := TakeReady(entries, test.n)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("TakeReady(..., %d) = %#v, want %#v", test.n, got, test.want)
			}
			if len(got) > 0 {
				got[0].Task = "changed"
				if entries[0].Task != "one" {
					t.Fatal("TakeReady() result aliases input")
				}
			}
		})
	}
}

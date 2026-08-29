package gate

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/repo"
)

func diffGate(t *testing.T, beads *fakeBeads, git *fakeGit) *Gate {
	t.Helper()
	g, err := New(Deps{Beads: beads, Git: git, Repos: &fakeRepos{}})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestEpicDiffSelectsOnlyItsInterleavedChildCommits(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{children: map[string][]bd.Bead{
		"left":  {{ID: "left-a"}, {ID: "left-b"}},
		"right": {{ID: "right-a"}},
	}}
	git := &fakeGit{output: func(_ context.Context, _ repo.Repo, args ...string) (int, string, error) {
		switch args[0] {
		case "rev-parse":
			return 0, "base\n", nil
		case "log":
			return 0, "c1\x00left-a\nc2\x00right-a\nc3\x00left-b\n", nil
		case "show":
			return 0, "[" + args[3] + "]", nil
		}
		t.Fatalf("unexpected git argv %q", args)
		return 0, "", nil
	}}
	g := diffGate(t, beads, git)
	if !g.NoteEpicLand(context.Background(), r, "left") || !g.NoteEpicLand(context.Background(), r, "right") {
		t.Fatal("NoteEpicLand() failed")
	}
	left, err := g.EpicDiff(context.Background(), r, "left")
	if err != nil || left != "[c1][c3]" {
		t.Fatalf("left EpicDiff() = (%q, %v)", left, err)
	}
	right, err := g.EpicDiff(context.Background(), r, "right")
	if err != nil || right != "[c2]" {
		t.Fatalf("right EpicDiff() = (%q, %v)", right, err)
	}
	for _, argv := range git.Calls() {
		if argv[0] == "log" && !slices.Contains(argv, "base..main") {
			t.Fatalf("log argv = %q, want argv-only range", argv)
		}
	}
}

func TestEpicCommitsMatchesChildIDsExactly(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{children: map[string][]bd.Bead{"epic": {{ID: "magicite-cl"}}}}
	git := &fakeGit{output: func(_ context.Context, _ repo.Repo, args ...string) (int, string, error) {
		if args[0] == "log" {
			return 0, "wrong\x00magicite-cld\nright\x00magicite-cl\n", nil
		}
		return 0, "base\n", nil
	}}
	g := diffGate(t, beads, git)
	k, _ := g.key(r, "epic")
	g.recordStart(k, "base")
	got, err := g.EpicCommits(context.Background(), r, "epic")
	if err != nil || !slices.Equal(got, []string{"right"}) {
		t.Fatalf("EpicCommits() = (%q, %v)", got, err)
	}
}

func TestEpicCommitsUsesRecordedOlderStart(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{children: map[string][]bd.Bead{"epic": {{ID: "child"}}}}
	git := &fakeGit{output: func(_ context.Context, _ repo.Repo, args ...string) (int, string, error) {
		if args[0] != "log" || !slices.Contains(args, "old..main") {
			t.Fatalf("git argv = %q, want old range", args)
		}
		return 0, "old-commit\x00child\n", nil
	}}
	g := diffGate(t, beads, git)
	k, _ := g.key(r, "epic")
	g.recordStart(k, "old")
	got, err := g.EpicCommits(context.Background(), r, "epic")
	if err != nil || !slices.Equal(got, []string{"old-commit"}) {
		t.Fatalf("EpicCommits() = (%q, %v)", got, err)
	}
}

func TestEpicDiffFallsBackForUnstampedAndMissingStart(t *testing.T) {
	r := testRepo(t, "repo")
	for _, test := range []struct {
		name, start, wantRange string
	}{
		{"recorded", "base", "base"},
		{"missing", "", "main~1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			beads := &fakeBeads{children: map[string][]bd.Bead{"epic": {{ID: "child"}}}}
			git := &fakeGit{output: func(_ context.Context, _ repo.Repo, args ...string) (int, string, error) {
				switch args[0] {
				case "log":
					return 0, "unstamped\x00other\n", nil
				case "diff":
					if !slices.Equal(args, []string{"diff", test.wantRange, "main"}) {
						t.Fatalf("diff argv = %q", args)
					}
					return 0, "fallback", nil
				}
				t.Fatalf("unexpected git argv %q", args)
				return 0, "", nil
			}}
			g := diffGate(t, beads, git)
			if test.start != "" {
				k, _ := g.key(r, "epic")
				g.recordStart(k, test.start)
			}
			got, err := g.EpicDiff(context.Background(), r, "epic")
			if err != nil || got != "fallback" {
				t.Fatalf("EpicDiff() = (%q, %v)", got, err)
			}
		})
	}
}

func TestEpicDiffReturnsNoPartialOutputAfterGitFailure(t *testing.T) {
	r := testRepo(t, "repo")
	beads := &fakeBeads{children: map[string][]bd.Bead{"epic": {{ID: "child"}}}}
	git := &fakeGit{output: func(_ context.Context, _ repo.Repo, args ...string) (int, string, error) {
		switch args[0] {
		case "log":
			return 0, "first\x00child\nsecond\x00child\n", nil
		case "show":
			if args[3] == "first" {
				return 0, "first diff", nil
			}
			return 1, "show failed", errors.New("git failed")
		}
		t.Fatalf("unexpected git argv %q", args)
		return 0, "", nil
	}}
	g := diffGate(t, beads, git)
	k, _ := g.key(r, "epic")
	g.recordStart(k, "base")
	got, err := g.EpicDiff(context.Background(), r, "epic")
	if err == nil || got != "" {
		t.Fatalf("EpicDiff() = (%q, %v), want empty output and error", got, err)
	}
}

func TestNoteEpicLandRetainsFirstStartAndRefusesFailure(t *testing.T) {
	r := testRepo(t, "repo")
	calls := 0
	git := &fakeGit{output: func(_ context.Context, _ repo.Repo, args ...string) (int, string, error) {
		calls++
		if !slices.Equal(args, []string{"rev-parse", "main~1"}) {
			t.Fatalf("rev-parse argv = %q", args)
		}
		return 0, "first\n", nil
	}}
	g := diffGate(t, &fakeBeads{}, git)
	if !g.NoteEpicLand(context.Background(), r, "epic") || !g.NoteEpicLand(context.Background(), r, "epic") || calls != 1 {
		t.Fatal("NoteEpicLand() did not retain first start")
	}
	failed := diffGate(t, &fakeBeads{}, &fakeGit{output: func(context.Context, repo.Repo, ...string) (int, string, error) {
		return 1, "bad", nil
	}})
	if failed.NoteEpicLand(context.Background(), r, "epic") {
		t.Fatal("NoteEpicLand() accepted failed git")
	}
}

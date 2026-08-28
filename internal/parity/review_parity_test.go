package parity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/connorfranc/magicite/internal/bd"
	"github.com/connorfranc/magicite/internal/gate"
	"github.com/connorfranc/magicite/internal/repo"
	"github.com/connorfranc/magicite/internal/testenv"
)

func TestReviewEpicParity(t *testing.T) {
	for _, test := range []struct {
		name, verdict string
		closed        bool
	}{
		{name: "approve", verdict: "REVIEW: APPROVED", closed: true},
		{name: "drift", verdict: "REVIEW: DRIFT: repair child output", closed: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := testenv.New(t)
			repository := testenv.NewRepo(t, env, "repo")
			record, ok := repo.Make(repository.Root, "repo", "repo", "main")
			if !ok {
				t.Fatal("make fixture repository record")
			}
			repository.Commit("baseline", map[string]string{"baseline.txt": "base\n"})
			beads := &parityBeads{env: env, beads: map[string]bd.Bead{
				"epic-1":  {ID: "epic-1", IssueType: "epic", Status: "open"},
				"child-1": {ID: "child-1", Parent: "epic-1", Status: "closed"},
				"other-1": {ID: "other-1", Parent: "other-epic", Status: "closed"},
			}}
			g, err := gate.New(gate.Deps{Beads: beads, Git: reviewGit{tracedGit{t: t, env: env}}, Repos: reviewRepos{record.Name: record}})
			if err != nil {
				t.Fatal(err)
			}
			if !g.NoteEpicLand(context.Background(), record, "epic-1") {
				t.Fatal("record epic start")
			}
			repository.Commit("child one\n\nMagicite-Task: child-1\n", map[string]string{"child.txt": "one\n"})
			repository.Commit("other epic\n\nMagicite-Task: other-1\n", map[string]string{"other.txt": "other\n"})
			resetTrace(t, env)

			diff, err := g.EpicDiff(context.Background(), record, "epic-1")
			if err != nil || diff == "" {
				t.Fatalf("EpicDiff() = (%q, %v)", diff, err)
			}
			if err := testenv.Record(env.TracePath, []string{"kiro", "review", "epic-1"}, repository.Root); err != nil {
				t.Fatal(err)
			}
			g.NoteSession("review-1", record, "epic-1")
			if err := g.CompleteReview(context.Background(), "review-1", test.verdict); err != nil {
				t.Fatal(err)
			}
			epic := beads.beads["epic-1"]
			if (epic.Status == "closed") != test.closed {
				t.Fatalf("epic status = %q, want closed=%t", epic.Status, test.closed)
			}
			if test.closed {
				AssertTrace(t, "review-epic-approve", readTrace(t, env))
				return
			}
			fixes := 0
			for _, bead := range beads.beads {
				if bead.Parent == "epic-1" && bead.ID != "child-1" {
					fixes++
				}
			}
			if fixes != 1 {
				t.Fatalf("drift fixes = %d, want one", fixes)
			}
			AssertTrace(t, "review-epic-drift", readTrace(t, env))
		})
	}
}

type reviewGit struct{ tracedGit }

func (g reviewGit) Output(ctx context.Context, r repo.Repo, args ...string) (int, string, error) {
	return g.Git(ctx, r.Root, args...)
}

type reviewRepos map[string]repo.Repo

func (r reviewRepos) Get(name string) (repo.Repo, bool) { value, ok := r[name]; return value, ok }

func (b *parityBeads) Show(_ context.Context, _ repo.Repo, id string) (bd.Bead, error) {
	value, ok := b.beads[id]
	if !ok {
		return bd.Bead{}, fmt.Errorf("unknown bead %q", id)
	}
	return value, nil
}

func (b *parityBeads) Labels(context.Context, repo.Repo, string) ([]string, error) { return nil, nil }

func (b *parityBeads) EpicChildren(_ context.Context, _ repo.Repo, epic string) ([]bd.Bead, error) {
	children := make([]bd.Bead, 0)
	for _, bead := range b.beads {
		if bead.Parent == epic {
			children = append(children, bead)
		}
	}
	return children, nil
}

func (b *parityBeads) Query(context.Context, repo.Repo, string) ([]bd.Bead, error) { return nil, nil }

func (b *parityBeads) Create(_ context.Context, r repo.Repo, request bd.CreateRequest) (string, error) {
	path := filepath.Join(b.env.Root, "drift-body.md")
	if err := writeFileError(path, request.Body); err != nil {
		return "", err
	}
	id := "drift-1"
	if err := b.record(r.Root, "create", request.Title, "--type", request.Type, "--silent", "--parent", request.Parent, "--body-file", path, "--labels", "drift-fix", "--priority", request.Priority); err != nil {
		return "", err
	}
	b.beads[id] = bd.Bead{ID: id, Title: request.Title, Description: request.Body, Parent: request.Parent, Status: "open"}
	return id, nil
}

func (b *parityBeads) Comment(_ context.Context, r repo.Repo, id, text string) error {
	path := filepath.Join(b.env.Root, "review-comment.md")
	if err := writeFileError(path, text); err != nil {
		return err
	}
	return b.record(r.Root, "comment", id, "--file", path)
}

func (b *parityBeads) Close(_ context.Context, r repo.Repo, id, reason string) error {
	path := filepath.Join(b.env.Root, "close-reason.md")
	if err := writeFileError(path, reason); err != nil {
		return err
	}
	if err := b.record(r.Root, "close", id, "--reason-file", path); err != nil {
		return err
	}
	item, ok := b.beads[id]
	if !ok {
		return fmt.Errorf("unknown bead %q", id)
	}
	item.Status = "closed"
	b.beads[id] = item
	return nil
}

func writeFileError(path, text string) error {
	return os.WriteFile(path, []byte(text), 0o600)
}

package gate

import (
	"context"
	"fmt"
	"strings"

	"github.com/connorfranc/magicite/internal/logging"
	"github.com/connorfranc/magicite/internal/repo"
	"github.com/connorfranc/magicite/internal/stamp"
)

const epicLogFormat = "--format=%H%x00%(trailers:key=" + stamp.KeyTask + ",valueonly)"

// NoteEpicLand records the integration commit preceding an epic's first land.
func (g *Gate) NoteEpicLand(ctx context.Context, r repo.Repo, epic string) bool {
	k, ok := g.key(r, epic)
	if !ok {
		return false
	}
	if _, exists := g.start(k); exists {
		return true
	}

	exit, output, err := g.git.Output(ctx, r, "rev-parse", r.Branch+"~1")
	if err != nil || exit != 0 {
		g.gitWarn("epic-land", r, epic, exit, output, err)
		return false
	}
	g.recordStart(k, strings.TrimSpace(output))
	return true
}

// EpicCommits returns oldest-first integration commits stamped for epic children.
func (g *Gate) EpicCommits(ctx context.Context, r repo.Repo, epic string) ([]string, error) {
	k, ok := g.key(r, epic)
	if !ok {
		return nil, fmt.Errorf("gate: invalid epic %q", epic)
	}
	start, exists := g.start(k)
	if !exists {
		start = r.Branch + "~1"
	}
	return g.epicCommits(ctx, r, epic, start)
}

// EpicDiff returns patches for commits stamped for epic children.
func (g *Gate) EpicDiff(ctx context.Context, r repo.Repo, epic string) (string, error) {
	k, ok := g.key(r, epic)
	if !ok {
		return "", fmt.Errorf("gate: invalid epic %q", epic)
	}
	start, exists := g.start(k)
	if !exists {
		start = r.Branch + "~1"
		g.log.Event(logging.Warn, "epic-diff", map[string]any{
			"reason": "missing-start", "repo": r.LogName(), "epic": epic, "start": start,
		})
	}

	commits, err := g.epicCommits(ctx, r, epic, start)
	if err != nil {
		return "", err
	}
	if len(commits) == 0 {
		g.log.Event(logging.Warn, "epic-diff", map[string]any{
			"reason": "unstamped-fallback", "repo": r.LogName(), "epic": epic,
		})
		exit, output, err := g.git.Output(ctx, r, "diff", start, r.Branch)
		if err != nil || exit != 0 {
			return "", g.gitError("diff", exit, output, err)
		}
		return output, nil
	}

	var diff strings.Builder
	for _, hash := range commits {
		exit, output, err := g.git.Output(ctx, r, "show", "--format=", "--patch", hash)
		if err != nil || exit != 0 {
			return "", g.gitError("show", exit, output, err)
		}
		diff.WriteString(output)
	}
	return diff.String(), nil
}

func (g *Gate) epicCommits(ctx context.Context, r repo.Repo, epic, start string) ([]string, error) {
	children, err := g.beads.EpicChildren(ctx, r, epic)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]struct{}, len(children))
	for _, child := range children {
		ids[child.ID] = struct{}{}
	}

	exit, output, err := g.git.Output(ctx, r, "log", "--reverse", start+".."+r.Branch, epicLogFormat)
	if err != nil || exit != 0 {
		return nil, g.gitError("log", exit, output, err)
	}
	return stampedCommits(output, ids), nil
}

func stampedCommits(output string, ids map[string]struct{}) []string {
	fields := strings.Split(output, "\x00")
	seen := make(map[string]struct{})
	commits := make([]string, 0)
	for index := 0; index+1 < len(fields); index++ {
		hash := strings.TrimSpace(fields[index])
		trailers, nextHash, found := strings.Cut(fields[index+1], "\n")
		if !found {
			trailers = fields[index+1]
			nextHash = ""
		} else {
			for rest, hashFound := nextHash, true; hashFound; {
				var line string
				line, rest, hashFound = strings.Cut(rest, "\n")
				if hashFound {
					trailers += "\n" + line
					nextHash = rest
				}
			}
		}
		fields[index+1] = nextHash
		if hash == "" || !hasChildTrailer(trailers, ids) {
			continue
		}
		if _, duplicate := seen[hash]; duplicate {
			continue
		}
		seen[hash] = struct{}{}
		commits = append(commits, hash)
	}
	return commits
}

func hasChildTrailer(trailers string, ids map[string]struct{}) bool {
	for _, value := range strings.Split(trailers, "\n") {
		if _, ok := ids[value]; ok {
			return true
		}
	}
	return false
}

func (g *Gate) gitWarn(operation string, r repo.Repo, epic string, exit int, output string, err error) {
	fields := map[string]any{
		"operation": operation, "repo": r.LogName(), "epic": epic, "exit": exit, "output": output,
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	g.log.Event(logging.Warn, "epic-diff", fields)
}

func (g *Gate) gitError(operation string, exit int, output string, err error) error {
	failure := fmt.Errorf("gate: git %s failed (exit %d, output %q)", operation, exit, output)
	if err != nil {
		return fmt.Errorf("%w: %w", failure, err)
	}
	return failure
}

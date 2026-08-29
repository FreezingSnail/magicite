package gate

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/connorfranc/magicite/internal/bd"
	"github.com/connorfranc/magicite/internal/logging"
	"github.com/connorfranc/magicite/internal/repo"
)

const (
	driftFixPrefix       = "drift-fix: "
	approvedReviewReason = "review gate approved goal"
	unparseableComment   = "review ended without a verdict marker."
)

// NoteSession records one review session and consumes its attempt.
func (g *Gate) NoteSession(handle string, r repo.Repo, epic string) {
	if handle == "" {
		return
	}
	k, ok := g.key(r, epic)
	if !ok {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.tracked[handle]; exists {
		return
	}
	g.tracked[handle] = k
	g.attempted[k]++
}

// CompleteReview dispatches the parsed verdict from a tracked review session.
func (g *Gate) CompleteReview(ctx context.Context, handle, transcript string) error {
	k, ok := g.drop(handle)
	if !ok {
		g.warn("untracked-review", handle)
		return nil
	}
	r, ok := g.repos.Get(k.repo)
	if !ok {
		g.warn("vanished-repo", k.repo)
		return nil
	}
	return g.DispatchVerdict(ctx, r, k.epic, ParseVerdict(transcript))
}

// AbortReview records an aborted review and leaves its epic open.
func (g *Gate) AbortReview(ctx context.Context, handle, reason string) error {
	k, ok := g.drop(handle)
	if !ok {
		return nil
	}
	r, found := g.repos.Get(k.repo)
	fields := map[string]any{"handle": handle, "reason": reason}
	if found {
		fields["repo"] = r.LogName()
		fields["epic"] = k.epic
		_ = g.beads.Comment(ctx, r, k.epic, reason)
	} else {
		g.warn("vanished-repo", k.repo)
	}
	g.log.Event(logging.Warn, "review-abort", fields)
	return nil
}

// DispatchVerdict applies one review outcome to an epic.
func (g *Gate) DispatchVerdict(ctx context.Context, r repo.Repo, epic string, v Verdict) error {
	k, ok := g.key(r, epic)
	if !ok {
		return nil
	}
	fields := map[string]any{
		"epic":    epic,
		"repo":    r.LogName(),
		"verdict": v.Kind.String(),
		"attempt": g.attempts(k),
	}

	switch v.Kind {
	case VerdictApproved:
		if err := g.beads.Close(ctx, r, epic, approvedReviewReason); err != nil {
			return err
		}
		g.clear(k)
		g.log.Event(logging.Info, logging.KindVerdict, fields)
	case VerdictDrift:
		id, err := g.CreateDriftFix(ctx, r, epic, v.Feedback)
		if id != "" {
			_ = g.beads.Comment(ctx, r, id, v.Feedback)
		}
		_ = g.beads.Comment(ctx, r, epic, v.Feedback)
		fields["drift_fix"] = id
		fields["feedback"] = v.Feedback
		g.log.Event(logging.Warn, logging.KindVerdict, fields)
		return err
	default:
		_ = g.beads.Comment(ctx, r, epic, unparseableComment)
		g.log.Event(logging.Error, logging.KindVerdict, fields)
	}
	return nil
}

// CreateDriftFix files a highest-priority child task containing reviewer feedback.
func (g *Gate) CreateDriftFix(ctx context.Context, r repo.Repo, epic, feedback string) (string, error) {
	title := driftFixPrefix + truncateRunes(strings.Join(strings.Fields(feedback), " "), 40)
	id, err := g.beads.Create(ctx, r, bd.CreateRequest{
		Title:    title,
		Type:     "task",
		Parent:   epic,
		Priority: "P1",
		Labels:   []string{"drift-fix"},
		Body:     feedback,
	})
	if err != nil {
		g.log.Event(logging.Error, "review-drift-fix", map[string]any{
			"epic": epic, "repo": r.LogName(), "feedback": feedback, "error": err.Error(),
		})
		return "", err
	}
	return id, nil
}

func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit])
}

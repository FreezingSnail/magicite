package gate

import (
	"context"

	"github.com/connorfranc/magicite/internal/bd"
	"github.com/connorfranc/magicite/internal/decomp"
	"github.com/connorfranc/magicite/internal/logging"
	"github.com/connorfranc/magicite/internal/repo"
)

const (
	decompFixPrefix   = "decomp-fix: "
	decompPassComment = "decomposition rules pass."
)

// DecompChildren reads every epic child and its labels as decomposition input.
func (g *Gate) DecompChildren(ctx context.Context, r repo.Repo, epic string) ([]decomp.Child, error) {
	beads, err := g.beads.EpicChildren(ctx, r, epic)
	if err != nil {
		return nil, err
	}

	children := make([]decomp.Child, 0, len(beads))
	for _, bead := range beads {
		labels, err := g.beads.Labels(ctx, r, bead.ID)
		if err != nil {
			return nil, err
		}
		deps := make([]string, 0, len(bead.Dependencies))
		for _, dependency := range bead.Dependencies {
			deps = append(deps, dependency.ID)
		}
		children = append(children, decomp.Child{
			ID:          bead.ID,
			Description: bead.Description,
			Design:      bead.Design,
			Acceptance:  bead.AcceptanceCriteria,
			Labels:      labels,
			Deps:        deps,
		})
	}
	return children, nil
}

// DecompositionVerdict files one rendered remediation task when rules fail.
func (g *Gate) DecompositionVerdict(ctx context.Context, r repo.Repo, epic string) ([]decomp.Violation, error) {
	if !g.Enabled() {
		return nil, nil
	}
	if _, ok := g.key(r, epic); !ok {
		return nil, nil
	}

	children, err := g.DecompChildren(ctx, r, epic)
	if err != nil {
		return nil, err
	}
	violations := decomp.Check(children)
	fields := map[string]any{"epic": epic, "repo": r.LogName(), "child_count": len(children)}
	if len(violations) == 0 {
		err := g.beads.Comment(ctx, r, epic, decompPassComment)
		g.log.Event(logging.Info, "decomp-verdict", fields)
		return nil, err
	}

	id, err := g.CreateDecompFix(ctx, r, epic, violations)
	_ = g.beads.Comment(ctx, r, epic, decomp.Format(violations))
	fields["decomp_fix"] = id
	fields["violation_count"] = len(violations)
	g.log.Event(logging.Warn, "decomp-verdict", fields)
	return violations, err
}

// CreateDecompFix files one highest-priority task containing every violation.
func (g *Gate) CreateDecompFix(ctx context.Context, r repo.Repo, epic string, violations []decomp.Violation) (string, error) {
	return g.beads.Create(ctx, r, bd.CreateRequest{
		Title:    decompFixPrefix + epic,
		Type:     "task",
		Parent:   epic,
		Priority: "P1",
		Labels:   []string{"decomp-fix"},
		Body:     decomp.Format(violations),
	})
}

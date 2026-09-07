package daemon

import (
	"time"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/repo"
	"github.com/FreezingSnail/magicite/internal/wire"
)

const (
	unknownEligibilityReason    = "workflow facts unavailable"
	ineligibleEligibilityReason = "workflow operation unavailable"
)

// workflowFact is one externally gathered workflow decision. Conversion never
// derives a decision from bead fields: an unavailable fact remains unavailable.
type workflowFact struct {
	Known    bool
	Eligible bool
	Reason   string
}

// workflowFacts contains the dispatch and review decisions for one bead.
type workflowFacts struct {
	Dispatch workflowFact
	Review   workflowFact
}

// beadView converts one complete bd bead and already-known workflow facts into
// an independently owned snapshot value. It does not inspect repository state.
func beadView(repository repo.Repo, bead bd.Bead, facts workflowFacts) wire.BeadResult {
	dependencies := make([]wire.DependencyResult, len(bead.Dependencies))
	for index, dependency := range bead.Dependencies {
		dependencies[index] = wire.DependencyResult{
			ID:     dependencyID(dependency),
			Title:  dependency.Title,
			Status: dependency.Status,
			Type:   dependencyType(dependency),
		}
	}

	return wire.BeadResult{
		ID:                 bead.ID,
		Repo:               repository.Name,
		Title:              bead.Title,
		Description:        nullableString(bead.Description),
		Design:             nullableString(bead.Design),
		AcceptanceCriteria: nullableString(bead.AcceptanceCriteria),
		Status:             bead.Status,
		Priority:           bead.Priority,
		IssueType:          bead.IssueType,
		Assignee:           nullableString(bead.Assignee),
		Owner:              nullableString(bead.Owner),
		Parent:             nullableString(bead.Parent),
		CreatedAt:          nullableTime(bead.CreatedAt),
		CreatedBy:          nullableString(bead.CreatedBy),
		UpdatedAt:          nullableTime(bead.UpdatedAt),
		StartedAt:          nullableTime(bead.StartedAt),
		ClosedAt:           nullableTime(bead.ClosedAt),
		DeferredUntil:      nullableTime(bead.DeferredUntil),
		CloseReason:        nullableString(bead.CloseReason),
		DependencyCount:    bead.DependencyCount,
		DependentCount:     bead.DependentCount,
		CommentCount:       bead.CommentCount,
		Dependencies:       dependencies,
		Labels:             append([]string{}, bead.Labels...),
		Staged:             hasExactLabel(bead.Labels, "staged"),
		Dispatch:           eligibility(facts.Dispatch),
		Review:             eligibility(facts.Review),
	}
}

// eligibility converts an externally supplied workflow fact without inferring
// readiness. Unknown facts are always unavailable and carry an explanation.
func eligibility(fact workflowFact) wire.Eligibility {
	if !fact.Known {
		return wire.Eligibility{Reason: reasonPointer(fact.Reason, unknownEligibilityReason)}
	}
	if fact.Eligible {
		return wire.Eligibility{Eligible: true, Reason: nullableString(fact.Reason)}
	}
	return wire.Eligibility{Reason: reasonPointer(fact.Reason, ineligibleEligibilityReason)}
}

func dependencyID(dependency bd.Dependency) string {
	if dependency.ID != "" {
		return dependency.ID
	}
	return dependency.DependsOnID
}

func dependencyType(dependency bd.Dependency) string {
	if dependency.DependencyType != "" {
		return dependency.DependencyType
	}
	return dependency.Type
}

func hasExactLabel(labels []string, want string) bool {
	for _, label := range labels {
		if label == want {
			return true
		}
	}
	return false
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func nullableTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return &parsed
}

func reasonPointer(reason, fallback string) *string {
	if value := nullableString(reason); value != nil {
		return value
	}
	return nullableString(fallback)
}

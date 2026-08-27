package bd

import (
	"strings"
)

// CreateArgs contains optional fields for creating a bead.
type CreateArgs struct {
	Title, Type, Parent, BodyFile, DesignFile, Acceptance, Priority string
	Labels                                                          []string
}

// UpdateArgs contains optional fields for updating a bead.
type UpdateArgs struct {
	Status, Assignee, BodyFile, DesignFile, Acceptance, Defer string
	AddLabels, RemoveLabels                                   []string
	Claim                                                     bool
}

// ArgsReady builds arguments for ready, excluding epics and human beads.
func ArgsReady() []string {
	return []string{"ready", "--exclude-type", "epic", "--exclude-label", HumanLabel, "--json"}
}

// ArgsList builds arguments for listing beads.
func ArgsList(all bool) []string {
	args := []string{"list", "--json"}
	if all {
		args = append(args, "--all")
	}
	return args
}

// ArgsShow builds arguments for showing one bead.
func ArgsShow(id string) []string {
	return []string{"show", id, "--json"}
}

// ArgsQuery builds arguments for querying beads.
func ArgsQuery(q string, all bool) []string {
	args := []string{"query", q, "--json"}
	if all {
		args = append(args, "--all")
	}
	return args
}

// ArgsDepList builds arguments for listing a bead's dependencies.
func ArgsDepList(id string) []string {
	return []string{"dep", "list", id, "--json"}
}

// ArgsDepAdd builds arguments for adding a dependency.
func ArgsDepAdd(id, dependsOn string) []string {
	return []string{"dep", "add", id, dependsOn}
}

// ArgsLabelList builds arguments for listing a bead's labels.
func ArgsLabelList(id string) []string {
	return []string{"label", "list", id, "--json"}
}

// ArgsLabelAdd builds arguments for adding a label.
func ArgsLabelAdd(id, label string) []string {
	return []string{"label", "add", id, label}
}

// ArgsLabelRemove builds arguments for removing a label.
func ArgsLabelRemove(id, label string) []string {
	return []string{"label", "remove", id, label}
}

// ArgsCreate builds arguments for creating a bead.
func ArgsCreate(a CreateArgs) []string {
	typ := a.Type
	if typ == "" {
		typ = "task"
	}
	args := []string{"create", a.Title, "--type", typ, "--silent"}
	args = appendValue(args, "--parent", a.Parent)
	args = appendValue(args, "--body-file", a.BodyFile)
	args = appendValue(args, "--design-file", a.DesignFile)
	args = appendValue(args, "--acceptance", a.Acceptance)
	if len(a.Labels) > 0 {
		args = append(args, "--labels", strings.Join(a.Labels, ","))
	}
	args = appendValue(args, "--priority", a.Priority)
	return args
}

// ArgsUpdate builds arguments for updating a bead.
func ArgsUpdate(id string, a UpdateArgs) []string {
	args := []string{"update", id}
	args = appendValue(args, "--status", a.Status)
	args = appendValue(args, "--assignee", a.Assignee)
	if a.Claim {
		args = append(args, "--claim")
	}
	args = appendValue(args, "--body-file", a.BodyFile)
	args = appendValue(args, "--design-file", a.DesignFile)
	args = appendValue(args, "--acceptance", a.Acceptance)
	for _, label := range a.AddLabels {
		args = append(args, "--add-label", label)
	}
	for _, label := range a.RemoveLabels {
		args = append(args, "--remove-label", label)
	}
	args = appendValue(args, "--defer", a.Defer)
	return args
}

// ArgsClose builds arguments for closing a bead with a file-backed reason.
func ArgsClose(id, reasonFile string) []string {
	return []string{"close", id, "--reason-file", reasonFile}
}

// ArgsComment builds arguments for adding a file-backed comment.
func ArgsComment(id, file string) []string {
	return []string{"comment", id, "--file", file}
}

// ArgsDefer builds arguments for deferring a bead.
func ArgsDefer(id, until string) []string {
	return []string{"update", id, "--defer", until}
}

// ArgsUndefer builds arguments for clearing a bead's defer date.
func ArgsUndefer(id string) []string {
	return []string{"update", id, "--defer", ""}
}

func appendValue(args []string, flag, value string) []string {
	if value == "" {
		return args
	}
	return append(args, flag, value)
}

// HumanLabel identifies beads reserved for human implementation.
const HumanLabel = "human"

// NotHumanQuery adds the human-bead exclusion to q.
func NotHumanQuery(q string) string {
	return q + " AND NOT label=" + HumanLabel
}

// InProgressQuery selects non-human in-progress tasks.
func InProgressQuery() string {
	return NotHumanQuery("status=in_progress AND type=task")
}

// DriftFixQuery selects non-human drift-fix beads.
func DriftFixQuery() string {
	return NotHumanQuery("label=drift-fix")
}

// OpenEpicsQuery selects open epics.
func OpenEpicsQuery() string {
	return "status=open AND type=epic"
}

// EpicChildrenQuery selects direct children of epic.
func EpicChildrenQuery(epic string) string {
	return "parent=" + epic
}

// EpicOpenChildrenQuery selects open direct children of epic.
func EpicOpenChildrenQuery(epic string) string {
	return "parent=" + epic + " AND status=open"
}

// IsHuman reports whether labels contain the human label.
func IsHuman(labels []string) bool {
	for _, label := range labels {
		if strings.EqualFold(strings.TrimSpace(label), HumanLabel) {
			return true
		}
	}
	return false
}

// DifficultyFromLabels returns high or low difficulty, preferring high.
func DifficultyFromLabels(labels []string) string {
	low := false
	for _, label := range labels {
		switch strings.ToLower(strings.TrimSpace(label)) {
		case "difficulty:high":
			return "high"
		case "difficulty:low":
			low = true
		}
	}
	if low {
		return "low"
	}
	return ""
}

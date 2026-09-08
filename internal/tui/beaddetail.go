package tui

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"

	"github.com/FreezingSnail/magicite/internal/wire"
)

// BeadDetail presents one daemon snapshot bead without performing lookups.
type BeadDetail struct {
	viewport viewport.Model
	bead     wire.BeadResult
	beadID   string
	content  string
	open     bool
	removed  bool
	width    int
	height   int
}

// NewBeadDetail opens a scrollable detail pane for bead.
func NewBeadDetail(bead wire.BeadResult) *BeadDetail {
	detail := &BeadDetail{viewport: viewport.New(80, 24), beadID: bead.ID, open: true, width: 80, height: 24}
	detail.Refresh(bead)
	return detail
}

// IsOpen reports whether the detail pane owns Beads-tab keyboard focus.
func (detail *BeadDetail) IsOpen() bool { return detail != nil && detail.open }

// Refresh replaces snapshot content while retaining the current scroll offset.
// An empty bead ID represents a bead removed from the latest snapshot.
func (detail *BeadDetail) Refresh(bead wire.BeadResult) {
	if detail == nil {
		return
	}
	offset := detail.viewport.YOffset
	detail.removed = bead.ID == ""
	if detail.removed {
		detail.content = "Bead removed from snapshot"
	} else {
		detail.bead = cloneBead(bead)
		detail.beadID = bead.ID
		detail.content = beadDetailContent(detail.bead, detail.contentWidth())
	}
	detail.viewport.SetContent(detail.content)
	detail.viewport.SetYOffset(offset)
}

// Update handles viewport movement and closes on Esc.
func (detail *BeadDetail) Update(message tea.Msg) tea.Cmd {
	if detail == nil || !detail.open {
		return nil
	}
	if key, ok := message.(tea.KeyMsg); ok && key.Type == tea.KeyEsc {
		detail.open = false
		return nil
	}
	var command tea.Cmd
	detail.viewport, command = detail.viewport.Update(message)
	return command
}

// View renders the current content at width and height without color.
func (detail *BeadDetail) View(width, height int) string {
	if detail == nil || !detail.open || width <= 0 || height <= 0 {
		return ""
	}
	if width != detail.width || height != detail.height {
		offset := detail.viewport.YOffset
		detail.width, detail.height = width, height
		detail.viewport.Width, detail.viewport.Height = width, height
		if detail.removed {
			detail.viewport.SetContent("Bead removed from snapshot")
		} else {
			detail.content = beadDetailContent(detail.bead, width)
			detail.viewport.SetContent(detail.content)
		}
		detail.viewport.SetYOffset(offset)
	}
	return detail.viewport.View()
}

func (detail *BeadDetail) contentWidth() int {
	if detail.width > 0 {
		return detail.width
	}
	return 80
}

func beadDetailContent(bead wire.BeadResult, width int) string {
	sections := make([]string, 0, 14)
	identity := []string{
		detailField("ID", bead.ID, width),
		detailField("Type", bead.IssueType, width),
		detailField("State", bead.Status, width),
		detailField("Priority", strconv.Itoa(bead.Priority), width),
		detailField("Staged", strconv.FormatBool(bead.Staged), width),
		detailField("Repository", bead.Repo, width),
	}
	sections = append(sections, detailSection("Identity", identity))
	sections = appendOptionalDetailSection(sections, "Title", bead.Title, width, false)
	sections = appendOptionalPointerSection(sections, "Body", bead.Description, width, true)
	sections = appendOptionalPointerSection(sections, "Design", bead.Design, width, true)
	sections = appendOptionalPointerSection(sections, "Acceptance criteria", bead.AcceptanceCriteria, width, true)
	if len(bead.Labels) > 0 {
		labels := append([]string(nil), bead.Labels...)
		sort.Strings(labels)
		sections = append(sections, detailSection("Labels", []string{detailField("Labels", strings.Join(labels, ", "), width)}))
	}
	assignment := make([]string, 0, 2)
	if bead.Assignee != nil {
		assignment = append(assignment, detailField("Assignee", *bead.Assignee, width))
	}
	if bead.Owner != nil {
		assignment = append(assignment, detailField("Owner", *bead.Owner, width))
	}
	if len(assignment) > 0 {
		sections = append(sections, detailSection("Assignment", assignment))
	}
	if bead.Parent != nil {
		sections = append(sections, detailSection("Parent", []string{detailField("Parent", *bead.Parent, width)}))
	}
	timestamps := make([]string, 0, 4)
	for _, value := range []struct {
		label string
		at    *time.Time
	}{
		{"Created", bead.CreatedAt},
		{"Updated", bead.UpdatedAt},
		{"Started", bead.StartedAt},
		{"Closed", bead.ClosedAt},
	} {
		if value.at != nil {
			timestamps = append(timestamps, detailField(value.label, value.at.Format(time.RFC3339), width))
		}
	}
	if len(timestamps) > 0 {
		sections = append(sections, detailSection("Timestamps", timestamps))
	}
	if bead.DeferredUntil != nil {
		sections = append(sections, detailSection("Deferred until", []string{detailField("Deferred until", bead.DeferredUntil.Format(time.RFC3339), width)}))
	}
	dependencies := append([]wire.DependencyResult(nil), bead.Dependencies...)
	sort.SliceStable(dependencies, func(left, right int) bool {
		if dependencies[left].Type != dependencies[right].Type {
			return dependencies[left].Type < dependencies[right].Type
		}
		return dependencies[left].ID < dependencies[right].ID
	})
	dependsOn, blocks := make([]string, 0, len(dependencies)), make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		line := dependencyDetailLine(dependency, width)
		if dependency.Type == "blocks" {
			dependsOn = append(dependsOn, line)
		} else {
			blocks = append(blocks, line)
		}
	}
	if len(dependsOn) > 0 {
		sections = append(sections, detailSection("Depends on", dependsOn))
	}
	if len(blocks) > 0 {
		sections = append(sections, detailSection("Blocks", blocks))
	}
	sections = append(sections, detailSection("Counts", []string{
		detailField("Dependencies", strconv.Itoa(bead.DependencyCount), width),
		detailField("Children", strconv.Itoa(bead.DependentCount), width),
		detailField("Comments", strconv.Itoa(bead.CommentCount), width),
	}))
	metadata := make([]string, 0, 2)
	if bead.CreatedBy != nil {
		metadata = append(metadata, detailField("Created by", *bead.CreatedBy, width))
	}
	if bead.CloseReason != nil {
		metadata = append(metadata, detailField("Close reason", *bead.CloseReason, width))
	}
	if len(metadata) > 0 {
		sections = append(sections, detailSection("Close and comment metadata", metadata))
	}
	sections = append(sections, detailSection("Eligibility", []string{
		detailEligibility("Dispatch", bead.Dispatch, width),
		detailEligibility("Review", bead.Review, width),
	}))
	return strings.Join(sections, "\n\n")
}

func appendOptionalDetailSection(sections []string, title, value string, width int, wrap bool) []string {
	if value == "" {
		return sections
	}
	if wrap {
		return append(sections, detailSection(title, detailWrapped(value, width)))
	}
	return append(sections, detailSection(title, []string{Truncate(SanitizeText(value), width)}))
}

func appendOptionalPointerSection(sections []string, title string, value *string, width int, wrap bool) []string {
	if value == nil {
		return sections
	}
	return appendOptionalDetailSection(sections, title, *value, width, wrap)
}

func detailSection(title string, lines []string) string {
	return SanitizeText(title) + "\n" + strings.Join(lines, "\n")
}

func detailField(label, value string, width int) string {
	return Truncate(SanitizeText(label)+": "+SanitizeText(value), width)
}

func dependencyDetailLine(dependency wire.DependencyResult, width int) string {
	line := SanitizeText(dependency.ID)
	if dependency.Status != "" {
		line += " [" + SanitizeText(dependency.Status) + "]"
	}
	if dependency.Title != "" {
		line += " " + SanitizeText(dependency.Title)
	}
	return Truncate(line, width)
}

func detailEligibility(label string, eligibility wire.Eligibility, width int) string {
	state := "unknown"
	if eligibility.Eligible {
		state = "eligible"
	} else if eligibility.Reason != nil {
		state = "ineligible"
	}
	line := SanitizeText(label) + ": " + state
	if eligibility.Reason != nil {
		line += " — " + SanitizeText(*eligibility.Reason)
	}
	return Truncate(line, width)
}

func detailWrapped(value string, width int) []string {
	if width <= 0 {
		return nil
	}
	paragraphs := strings.Split(value, "\n")
	lines := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		paragraph = SanitizeText(paragraph)
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		line := ""
		for _, word := range strings.Fields(paragraph) {
			if line == "" {
				line = word
			} else if lipgloss.Width(line)+1+lipgloss.Width(word) <= width {
				line += " " + word
			} else {
				lines = append(lines, line)
				line = word
			}
			for lipgloss.Width(line) > width {
				part, rest := splitDetailWidth(line, width)
				lines = append(lines, part)
				line = rest
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitDetailWidth(value string, width int) (string, string) {
	graphemes := uniseg.NewGraphemes(value)
	used, boundary := 0, 0
	for graphemes.Next() {
		grapheme := graphemes.Str()
		graphemeWidth := lipgloss.Width(grapheme)
		if used+graphemeWidth > width && boundary > 0 {
			break
		}
		used += graphemeWidth
		boundary += len(grapheme)
		if used >= width {
			break
		}
	}
	if boundary == 0 {
		return "", value
	}
	return value[:boundary], value[boundary:]
}

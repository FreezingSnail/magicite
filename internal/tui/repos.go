package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/wire"
)

const repoPlaceholder = "—"

type repoRow struct {
	repo   wire.RepoResult
	health string
	detail string
}

// ReposView renders daemon-provided repository identity and snapshot health.
// Repository rows are prepared during Snapshot; View performs no I/O or local
// repository inspection.
type ReposView struct {
	repositories     []wire.RepoResult
	repositoryErrors []wire.RepositoryError
	rows             []repoRow
	selection        Selection
}

// NewReposView constructs an empty Repositories tab.
func NewReposView() *ReposView {
	return &ReposView{selection: Selection{Index: -1}}
}

func (*ReposView) Title() string { return "Repositories" }

// Update moves the stable repository selection with vim or arrow keys.
func (view *ReposView) Update(message tea.Msg) (TabView, tea.Cmd) {
	key, ok := message.(tea.KeyMsg)
	if !ok {
		return view, nil
	}
	switch key.String() {
	case "j", "down":
		view.move(1)
	case "k", "up":
		view.move(-1)
	case "home":
		view.moveTo(0)
	case "end":
		view.moveTo(len(view.rows) - 1)
	}
	return view, nil
}

// Snapshot replaces cached daemon data and preserves selection by repository
// name. Error lookup is indexed once, before rows are built.
func (view *ReposView) Snapshot(snapshot wire.SnapshotResult) TabView {
	view.repositories = append([]wire.RepoResult(nil), snapshot.Repositories...)
	view.repositoryErrors = append([]wire.RepositoryError(nil), snapshot.RepositoryErrors...)

	errorsByRepository := make(map[string]string, len(view.repositoryErrors))
	for _, repositoryError := range view.repositoryErrors {
		errorsByRepository[repositoryError.Repository] = SanitizeText(repositoryError.Error)
	}
	view.rows = make([]repoRow, len(view.repositories))
	for index, repository := range view.repositories {
		row := repoRow{repo: repository, health: "ok (healthy)"}
		if detail, failed := errorsByRepository[repository.Name]; failed {
			row.health = "error (read failed)"
			row.detail = detail
		}
		view.rows[index] = row
	}
	sort.SliceStable(view.rows, func(left, right int) bool {
		return view.rows[left].repo.Name < view.rows[right].repo.Name
	})
	view.selection = KeepSelection(repoRowKeys(view.rows), view.selection)
	return view
}

// Hints returns no action hints; repositories are read-only.
func (*ReposView) Hints() []Hint { return nil }

// View renders cached rows without I/O, mutation, or clock reads.
func (view *ReposView) View(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if len(view.rows) == 0 {
		return Truncate(SanitizeText("No repositories configured"), width)
	}

	rows := make([][]string, len(view.rows))
	for index, row := range view.rows {
		rows[index] = repoRowValues(row)
	}
	table := Table{
		Columns:  repoViewColumns(),
		Rows:     rows,
		Selected: view.selection.Index,
	}
	selectedDetail := ""
	if view.selection.Index >= 0 && view.selection.Index < len(view.rows) {
		selectedDetail = view.rows[view.selection.Index].detail
	}
	tableHeight := height
	if selectedDetail != "" && tableHeight > 1 {
		tableHeight--
	}
	output := table.Render(width, tableHeight)
	if selectedDetail == "" || tableHeight == height {
		return output
	}
	return output + "\nDetail: " + selectedDetail
}

// SelectedRepo returns the current snapshot repository, if one is selected.
func (view *ReposView) SelectedRepo() (wire.RepoResult, bool) {
	if view.selection.Index < 0 || view.selection.Index >= len(view.rows) {
		return wire.RepoResult{}, false
	}
	return view.rows[view.selection.Index].repo, true
}

func (view *ReposView) move(delta int) {
	view.moveTo(view.selection.Index + delta)
}

func (view *ReposView) moveTo(index int) {
	view.selection = KeepSelection(repoRowKeys(view.rows), Selection{Index: index})
}

func repoRowKeys(rows []repoRow) []string {
	keys := make([]string, len(rows))
	for index := range rows {
		keys[index] = rows[index].repo.Name
	}
	return keys
}

func repoViewColumns() []Column {
	return []Column{
		{Title: "Repository", Width: 18, Min: 4},
		{Title: "Prefix", Width: 14, Min: 4},
		{Title: "Branch", Width: 16, Min: 4},
		{Title: "Beads", Width: 8, Min: 5, Right: true},
		{Title: "Health", Width: 20, Min: 6},
		{Title: "Detail", Width: 32, Min: 6, Weight: 1},
	}
}

func repoRowValues(row repoRow) []string {
	return []string{
		repoCell(row.repo.Name),
		repoCell(row.repo.Prefix),
		repoCell(row.repo.Branch),
		repoPlaceholder,
		repoCell(row.health),
		repoCell(row.detail),
	}
}

func repoCell(value string) string {
	if value == "" {
		return repoPlaceholder
	}
	return SanitizeText(value)
}

var _ TabView = (*ReposView)(nil)

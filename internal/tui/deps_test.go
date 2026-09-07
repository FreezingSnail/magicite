package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	_ tea.Model = charmCompileModel{}
	_           = spinner.New
	_           = lipgloss.NewStyle
)

type charmCompileModel struct{}

func (charmCompileModel) Init() tea.Cmd {
	return nil
}

func (charmCompileModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return charmCompileModel{}, nil
}

func (charmCompileModel) View() string {
	return ""
}

func TestCharmDependenciesCompile(t *testing.T) {
	t.Parallel()
}

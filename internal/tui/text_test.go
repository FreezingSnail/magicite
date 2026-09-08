package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestSanitizeTextControlsTabsAndInvalidUTF8(t *testing.T) {
	input := "a\t\x00\x1b[31m\x7f\x9f\n\xffz"
	want := "a    ��[31m����z"
	if got := SanitizeText(input); got != want {
		t.Fatalf("SanitizeText() = %q, want %q", got, want)
	}
}

func TestTruncateRespectsDisplayWidthAndGraphemes(t *testing.T) {
	tests := []struct {
		text  string
		width int
		want  string
	}{
		{"plain", 0, ""},
		{"plain", 5, "plain"},
		{"plain", 4, "pla…"},
		{"界界", 3, "界…"},
		{"e\u0301x", 2, "e\u0301x"},
		{"e\u0301xy", 2, "e\u0301…"},
		{"\x1b[31mred", 5, "�[31…"},
	}
	for _, test := range tests {
		got := Truncate(test.text, test.width)
		if got != test.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", test.text, test.width, got, test.want)
		}
		if lipgloss.Width(got) > test.width {
			t.Errorf("Truncate(%q, %d) width = %d", test.text, test.width, lipgloss.Width(got))
		}
	}
}

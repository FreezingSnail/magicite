package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"
)

const (
	textPlaceholder = "�"
	tabSpaces       = "    "
	ellipsis        = "…"
)

// SanitizeText replaces terminal control characters with visible text.
func SanitizeText(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))
	for len(text) > 0 {
		runeValue, size := utf8.DecodeRuneInString(text)
		if runeValue == utf8.RuneError && size == 1 {
			builder.WriteString(textPlaceholder)
			text = text[1:]
			continue
		}
		switch {
		case runeValue == '\t':
			builder.WriteString(tabSpaces)
		case runeValue <= 0x1f || (runeValue >= 0x7f && runeValue <= 0x9f):
			builder.WriteString(textPlaceholder)
		default:
			builder.WriteRune(runeValue)
		}
		text = text[size:]
	}
	return builder.String()
}

// Truncate returns text no wider than width terminal cells without splitting a grapheme.
func Truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}

	text = SanitizeText(text)
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return ellipsis
	}

	limit := width - lipgloss.Width(ellipsis)
	var builder strings.Builder
	graphemes := uniseg.NewGraphemes(text)
	used := 0
	for graphemes.Next() {
		grapheme := graphemes.Str()
		graphemeWidth := lipgloss.Width(grapheme)
		if used+graphemeWidth > limit {
			break
		}
		builder.WriteString(grapheme)
		used += graphemeWidth
	}
	builder.WriteString(ellipsis)
	return builder.String()
}

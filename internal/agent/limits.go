package agent

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TailWindow is the maximum number of output bytes inspected by tail predicates.
const TailWindow = 2048

var limitPhrases = []string{
	"usage limit",
	"usage exceeded",
	"rate limit",
	"rate-limit",
	"too many requests",
	"429",
	"quota",
	"insufficient",
	"credit limit",
	"credits exceeded",
}

var authenticationPhrases = []string{
	"auth",
	"authentication",
	"authorization",
	"unauthorized",
	"not authenticated",
	"invalid api key",
}

// LimitedLine reports whether an error-carrying NDJSON line describes a usage limit.
func LimitedLine(line string) bool {
	var value any
	if json.Unmarshal([]byte(line), &value) != nil || !errorCarrying(value) {
		return false
	}
	return containsAny(strings.ToLower(line), limitPhrases)
}

// LimitedTail reports whether the output tail describes a usage limit.
func LimitedTail(output string) bool {
	return containsAny(tail(output), limitPhrases)
}

// FailureTail reports whether the output tail describes a limit or authentication failure.
func FailureTail(output string) bool {
	tail := tail(output)
	return containsAny(tail, limitPhrases) || containsAny(tail, authenticationPhrases)
}

func tail(output string) string {
	if len(output) > TailWindow {
		start := len(output) - TailWindow
		for start < len(output) && !utf8.RuneStart(output[start]) {
			start++
		}
		output = output[start:]
	}
	return strings.ToLower(output)
}

func errorCarrying(value any) bool {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if key == "error" || (key == "type" && child == "session.error") {
				return true
			}
			if errorCarrying(child) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if errorCarrying(child) {
				return true
			}
		}
	}
	return false
}

func containsAny(text string, phrases []string) bool {
	for _, phrase := range phrases {
		if containsPhrase(text, phrase) {
			return true
		}
	}
	return false
}

func containsPhrase(text, phrase string) bool {
	words := strings.Fields(phrase)
	for start := 0; start < len(text); {
		if !wordBoundaryBefore(text, start) {
			_, size := utf8.DecodeRuneInString(text[start:])
			start += size
			continue
		}

		position := start
		matched := true
		for index, word := range words {
			if !strings.HasPrefix(text[position:], word) {
				matched = false
				break
			}
			position += len(word)
			if index+1 < len(words) {
				separatorStart := position
				for position < len(text) {
					runeValue, size := utf8.DecodeRuneInString(text[position:])
					if runeValue != '-' && !unicode.IsSpace(runeValue) {
						break
					}
					position += size
				}
				if position == separatorStart {
					matched = false
					break
				}
			}
		}
		if matched && wordBoundaryAfter(text, position) {
			return true
		}

		_, size := utf8.DecodeRuneInString(text[start:])
		start += size
	}
	return false
}

func wordBoundaryBefore(text string, position int) bool {
	if position == 0 {
		return true
	}
	runeValue, _ := utf8.DecodeLastRuneInString(text[:position])
	return !wordRune(runeValue)
}

func wordBoundaryAfter(text string, position int) bool {
	if position == len(text) {
		return true
	}
	runeValue, _ := utf8.DecodeRuneInString(text[position:])
	return !wordRune(runeValue)
}

func wordRune(runeValue rune) bool {
	return runeValue == '_' || unicode.IsLetter(runeValue) || unicode.IsDigit(runeValue)
}

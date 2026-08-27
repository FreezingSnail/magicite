package stamp

import "strings"

var keySet = func() map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, key := range keys {
		set[key] = true
	}
	return set
}()

// Parse returns known Magicite trailers in message order.
func Parse(message string) []Trailer {
	trailers := make([]Trailer, 0)
	for _, line := range messageLines(message) {
		if key, value, ok := trailerLine(line); ok && keySet[key] {
			trailers = append(trailers, Trailer{Key: key, Value: strings.TrimSpace(value)})
		}
	}
	return trailers
}

// Apply removes existing Magicite trailers and appends one canonical block.
func Apply(message string, trailers []Trailer) string {
	body := make([]string, 0)
	for _, line := range messageLines(message) {
		if key, _, ok := trailerLine(line); ok && keySet[key] {
			continue
		}
		body = append(body, line)
	}
	cleaned := strings.Join(body, "")
	if len(trailers) == 0 {
		return cleaned
	}

	values := make(map[string]string, len(trailers))
	for _, trailer := range trailers {
		if keySet[trailer.Key] {
			if value := Sanitize(trailer.Value); value != "" {
				values[trailer.Key] = value
			}
		}
	}
	if len(values) == 0 {
		return cleaned
	}

	bodyText := trimTrailingBlankLines(cleaned)
	block := make([]string, 0, len(values))
	for _, key := range keys {
		if value := values[key]; value != "" {
			block = append(block, key+": "+value)
		}
	}
	if bodyText == "" {
		return strings.Join(block, "\n") + "\n"
	}
	return bodyText + "\n\n" + strings.Join(block, "\n") + "\n"
}

func trailerLine(line string) (key, value string, ok bool) {
	content := strings.TrimSuffix(line, "\n")
	content = strings.TrimSuffix(content, "\r")
	key, value, ok = strings.Cut(content, ": ")
	return key, value, ok
}

func messageLines(message string) []string {
	if message == "" {
		return nil
	}
	lines := make([]string, 0, strings.Count(message, "\n")+1)
	start := 0
	for index := 0; index < len(message); index++ {
		if message[index] == '\n' {
			lines = append(lines, message[start:index+1])
			start = index + 1
		}
	}
	if start < len(message) {
		lines = append(lines, message[start:])
	}
	return lines
}

func trimTrailingBlankLines(message string) string {
	lines := messageLines(message)
	last := len(lines) - 1
	for last >= 0 {
		content := strings.TrimSuffix(lines[last], "\n")
		content = strings.TrimSuffix(content, "\r")
		if strings.TrimSpace(content) != "" {
			break
		}
		last--
	}
	if last < 0 {
		return ""
	}
	return strings.Join(lines[:last], "") + strings.TrimSuffix(strings.TrimSuffix(lines[last], "\n"), "\r")
}

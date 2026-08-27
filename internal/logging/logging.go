// Package logging records daemon lifecycle events as structured log lines.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Level controls which events are recorded.
type Level uint8

const (
	// Debug records diagnostic events.
	Debug Level = iota
	// Info records normal lifecycle events.
	Info
	// Warn records degraded but recoverable events.
	Warn
	// Error records failed lifecycle events.
	Error
)

// Named aliases keep level declarations readable at call sites that prefer a
// Level prefix.
const (
	LevelDebug = Debug
	LevelInfo  = Info
	LevelWarn  = Warn
	LevelError = Error
)

func (level Level) String() string {
	switch level {
	case Debug:
		return "debug"
	case Info:
		return "info"
	case Warn:
		return "warn"
	case Error:
		return "error"
	default:
		return "unknown"
	}
}

// Format selects the output representation.
type Format uint8

const (
	// Text writes a readable key-value line.
	Text Format = iota
	// JSON writes one JSON object per line.
	JSON
)

// Named aliases keep format declarations readable at call sites that prefer a
// Format prefix.
const (
	FormatText = Text
	FormatJSON = JSON
)

// Config controls the package logger. A nil Writer discards records.
type Config struct {
	Level  Level
	Format Format
	Writer io.Writer
}

type logger struct {
	mu     sync.Mutex
	config Config
	next   uint64
}

var defaultLogger = logger{
	config: Config{Level: Info, Format: Text, Writer: os.Stderr},
}

// Configure replaces the package logger configuration. It is safe to call
// while events are being recorded.
func Configure(config Config) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	defaultLogger.config = normalizeConfig(config)
}

// SetLevel changes the minimum level recorded by Event.
func SetLevel(level Level) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	defaultLogger.config.Level = normalizeLevel(level)
}

// SetFormat changes Event's output representation.
func SetFormat(format Format) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	defaultLogger.config.Format = normalizeFormat(format)
}

// SetWriter changes Event's output destination. A nil writer discards records.
func SetWriter(writer io.Writer) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	defaultLogger.config.Writer = writer
}

// Event records one lifecycle event. Logging failures are ignored so logging
// cannot interrupt a caller's lifecycle path.
func Event(level Level, kind string, fields map[string]any) {
	defer func() { _ = recover() }()

	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	if normalizeLevel(level) < defaultLogger.config.Level {
		return
	}

	defaultLogger.next++
	record := eventRecord{
		Sequence: defaultLogger.next,
		Level:    normalizeLevel(level).String(),
		Kind:     kind,
		Fields:   renderFields(fields),
	}
	line := renderRecord(record, defaultLogger.config.Format)
	if defaultLogger.config.Writer != nil {
		_, _ = defaultLogger.config.Writer.Write(append(line, '\n'))
	}
}

type eventRecord struct {
	Sequence uint64                     `json:"sequence"`
	Level    string                     `json:"level"`
	Kind     string                     `json:"kind"`
	Fields   map[string]json.RawMessage `json:"fields"`
}

func normalizeConfig(config Config) Config {
	config.Level = normalizeLevel(config.Level)
	config.Format = normalizeFormat(config.Format)
	return config
}

func normalizeLevel(level Level) Level {
	if level > Error {
		return Error
	}
	return level
}

func normalizeFormat(format Format) Format {
	if format != JSON {
		return Text
	}
	return format
}

func renderFields(fields map[string]any) map[string]json.RawMessage {
	rendered := make(map[string]json.RawMessage, len(fields))
	for key, value := range fields {
		rendered[key] = renderValue(value)
	}
	return rendered
}

func renderValue(value any) (rendered json.RawMessage) {
	defer func() {
		if recover() != nil {
			rendered = quoted(renderFallback(value))
		}
	}()

	encoded, err := json.Marshal(value)
	if err != nil {
		return quoted(renderFallback(value))
	}
	return encoded
}

func renderFallback(value any) (rendered string) {
	defer func() {
		if recover() != nil {
			rendered = fmt.Sprintf("<unrenderable %T>", value)
		}
	}()
	return fmt.Sprint(value)
}

func quoted(value string) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`"<unrenderable>"`)
	}
	return encoded
}

func renderRecord(record eventRecord, format Format) []byte {
	encoded, err := json.Marshal(record)
	if err != nil {
		return []byte(`{"sequence":0,"level":"error","kind":"logging.render","fields":{"error":"record encoding failed"}}`)
	}
	if format == JSON {
		return encoded
	}
	return []byte(fmt.Sprintf("sequence=%d level=%s kind=%q fields=%s", record.Sequence, record.Level, record.Kind, encodedFields(record.Fields)))
}

func encodedFields(fields map[string]json.RawMessage) []byte {
	encoded, err := json.Marshal(fields)
	if err != nil {
		return []byte(`{"error":"fields encoding failed"}`)
	}
	return encoded
}

// Package opencode parses OpenCode session event streams.
package opencode

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/connorfranc/magicite/internal/agent"
)

// Event is one recognized OpenCode stream event.
type Event struct {
	Type      string
	SessionID string
	Terminal  agent.Status
}

type wireEvent struct {
	Type       string `json:"type"`
	SessionID  string `json:"sessionID"`
	Properties struct {
		SessionID string `json:"sessionID"`
		Part      part   `json:"part"`
	} `json:"properties"`
	Part part `json:"part"`
}

type part struct {
	Reason string `json:"reason"`
	State  struct {
		Status string `json:"status"`
	} `json:"state"`
}

// ParseLine parses one NDJSON event line.
func ParseLine(line string) (Event, bool) {
	if strings.TrimSpace(line) == "" {
		return Event{}, false
	}

	var wire wireEvent
	if err := json.Unmarshal([]byte(line), &wire); err != nil || wire.Type == "" {
		return Event{}, false
	}
	if wire.SessionID == "" {
		wire.SessionID = wire.Properties.SessionID
	}

	event := Event{Type: wire.Type, SessionID: wire.SessionID}
	switch {
	case wire.Type == "step_finish" && wire.Part.Reason == "stop":
		event.Terminal = agent.StatusCompleted
	case wire.Type == "step_finish" && wire.Part.Reason == "error":
		event.Terminal = agent.StatusFailed
	case wire.Type == "tool_use" && wire.Part.State.Status == "error":
		event.Terminal = agent.StatusFailed
	}
	return event, true
}

// Scanner incrementally consumes an OpenCode NDJSON stream.
type Scanner struct {
	mu         sync.RWMutex
	pending    []byte
	transcript []byte
	sessionID  string
	status     agent.Status
	limited    bool
}

// NewScanner returns a scanner in the running state.
func NewScanner() *Scanner {
	return &Scanner{status: agent.StatusRunning}
}

// Write consumes complete NDJSON lines while retaining a partial final line.
func (s *Scanner) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.transcript = append(s.transcript, p...)
	s.pending = append(s.pending, p...)
	s.consumeCompleteLines()
	return len(p), nil
}

// Flush consumes the final partial line, if any.
func (s *Scanner) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.pending) == 0 {
		return
	}
	s.consume(string(s.pending))
	s.pending = nil
}

// SessionID returns the first non-empty session ID observed.
func (s *Scanner) SessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionID
}

// Status returns the current or terminal session status.
func (s *Scanner) Status() agent.Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Limited reports whether a consumed line described a usage limit.
func (s *Scanner) Limited() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.limited
}

// Transcript returns every byte written to the scanner.
func (s *Scanner) Transcript() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return string(s.transcript)
}

func (s *Scanner) consumeCompleteLines() {
	for {
		index := -1
		for i, value := range s.pending {
			if value == '\n' {
				index = i
				break
			}
		}
		if index < 0 {
			return
		}
		s.consume(string(s.pending[:index]))
		s.pending = s.pending[index+1:]
	}
}

func (s *Scanner) consume(line string) {
	if agent.LimitedLine(line) {
		s.limited = true
	}
	event, ok := ParseLine(line)
	if !ok {
		return
	}
	if s.sessionID == "" && event.SessionID != "" {
		s.sessionID = event.SessionID
	}
	switch event.Terminal {
	case agent.StatusFailed:
		s.status = agent.StatusFailed
	case agent.StatusCompleted:
		if s.status != agent.StatusFailed {
			s.status = agent.StatusCompleted
		}
	}
}

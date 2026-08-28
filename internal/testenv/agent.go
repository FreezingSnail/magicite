package testenv

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// Agent controls one fake agent executable installed in an Env.
type Agent struct {
	t     *testing.T
	env   *Env
	name  string
	store string
}

type agentConfig struct {
	Scenarios map[string]string `json:"scenarios"`
}

type agentEvent struct {
	SessionID string `json:"session_id"`
	Line      string `json:"line"`
}

// NewAgent installs fake-agent under name. name must be kiro or opencode.
func NewAgent(t *testing.T, env *Env, name string) *Agent {
	t.Helper()
	if name != "kiro" && name != "opencode" {
		t.Fatalf("invalid fake agent name %q", name)
	}
	fake := &Agent{t: t, env: env, name: name, store: filepath.Join(env.Root, "fake-agent.json")}
	env.fakeAgentStore = fake.store
	env.fakeAgentEvents = filepath.Join(env.Root, "fake-agent-events.ndjson")
	env.fakeAgentPIDs = filepath.Join(env.Root, "fake-agent-pids")
	env.fakeAgentScenario = "complete"
	if err := writeAgentConfig(fake.store, agentConfig{Scenarios: make(map[string]string)}); err != nil {
		t.Fatalf("initialize fake agent store: %v", err)
	}
	env.Install(name, "./cmd/fake-agent")
	return fake
}

// Scenario selects name for subsequent fake-agent runs.
func (a *Agent) Scenario(name string) {
	a.t.Helper()
	a.env.fakeAgentScenario = name
}

// ScenarioFor selects name when the submitted plan contains taskID.
func (a *Agent) ScenarioFor(taskID, name string) {
	a.t.Helper()
	if taskID == "" {
		a.t.Fatal("empty fake agent task ID")
	}
	if err := updateAgentConfig(a.store, func(config *agentConfig) {
		if config.Scenarios == nil {
			config.Scenarios = make(map[string]string)
		}
		config.Scenarios[taskID] = name
	}); err != nil {
		a.t.Fatalf("set fake agent scenario for %q: %v", taskID, err)
	}
}

// Calls returns recorded invocations of this fake-agent binary.
func (a *Agent) Calls() []Entry {
	a.t.Helper()
	entries, err := Read(a.env.TracePath)
	if err != nil {
		a.t.Fatalf("read fake agent calls: %v", err)
	}
	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if len(entry.Argv) > 0 && filepath.Base(entry.Argv[0]) == a.name {
			result = append(result, entry)
		}
	}
	return result
}

// ChildPIDs returns pids spawned by hang scenarios.
func (a *Agent) ChildPIDs() []int {
	a.t.Helper()
	file, err := os.Open(a.env.fakeAgentPIDs)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		a.t.Fatalf("open fake agent pids: %v", err)
	}
	defer file.Close()

	var pids []int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		pid, err := strconv.Atoi(scanner.Text())
		if err != nil || pid < 1 {
			a.t.Fatalf("invalid fake agent child pid %q", scanner.Text())
		}
		pids = append(pids, pid)
	}
	if err := scanner.Err(); err != nil {
		a.t.Fatalf("read fake agent pids: %v", err)
	}
	return pids
}

// Events returns complete NDJSON lines emitted for sessionID.
func (a *Agent) Events(sessionID string) []string {
	a.t.Helper()
	file, err := os.Open(a.env.fakeAgentEvents)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		a.t.Fatalf("open fake agent events: %v", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event agentEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			a.t.Fatalf("decode fake agent event: %v", err)
		}
		if event.SessionID == sessionID {
			lines = append(lines, event.Line)
		}
	}
	if err := scanner.Err(); err != nil {
		a.t.Fatalf("read fake agent events: %v", err)
	}
	return lines
}

// AgentScenario returns the scenario selected by plan, or fallback when none matches.
func AgentScenario(store, plan, fallback string) (string, error) {
	if store == "" {
		return fallback, nil
	}
	config, err := readAgentConfig(store)
	if err != nil {
		return "", err
	}
	selected := ""
	for taskID := range config.Scenarios {
		if strings.Contains(plan, taskID) && len(taskID) > len(selected) {
			selected = taskID
		}
	}
	if selected == "" {
		return fallback, nil
	}
	return config.Scenarios[selected], nil
}

// RecordAgentEvent stores one already-emitted complete NDJSON line.
func RecordAgentEvent(path, sessionID, line string) error {
	if path == "" {
		return nil
	}
	contents, err := json.Marshal(agentEvent{SessionID: sessionID, Line: line})
	if err != nil {
		return err
	}
	return appendAgentLine(path, string(contents))
}

// RecordAgentChildPID stores one child pid from a hang scenario.
func RecordAgentChildPID(path string, pid int) error {
	if path == "" {
		return nil
	}
	return appendAgentLine(path, strconv.Itoa(pid))
}

func appendAgentLine(path, line string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	if _, err := io.WriteString(file, line+"\n"); err != nil {
		return err
	}
	return file.Sync()
}

func readAgentConfig(path string) (agentConfig, error) {
	var value agentConfig
	err := withAgentConfig(path, func(file *os.File) error {
		contents, err := io.ReadAll(file)
		if err != nil {
			return err
		}
		if len(contents) == 0 {
			value.Scenarios = make(map[string]string)
			return nil
		}
		if err := json.Unmarshal(contents, &value); err != nil {
			return err
		}
		if value.Scenarios == nil {
			value.Scenarios = make(map[string]string)
		}
		return nil
	})
	return value, err
}

func writeAgentConfig(path string, value agentConfig) error {
	return updateAgentConfig(path, func(config *agentConfig) { *config = value })
}

func updateAgentConfig(path string, update func(*agentConfig)) error {
	return withAgentConfig(path, func(file *os.File) error {
		contents, err := io.ReadAll(file)
		if err != nil {
			return err
		}
		value := agentConfig{Scenarios: make(map[string]string)}
		if len(contents) != 0 && json.Unmarshal(contents, &value) != nil {
			return fmt.Errorf("decode fake agent store")
		}
		if value.Scenarios == nil {
			value.Scenarios = make(map[string]string)
		}
		update(&value)
		contents, err = json.Marshal(value)
		if err != nil {
			return err
		}
		if err := file.Truncate(0); err != nil {
			return err
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if _, err := file.Write(contents); err != nil {
			return err
		}
		return file.Sync()
	})
}

func withAgentConfig(path string, action func(*os.File) error) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return action(file)
}

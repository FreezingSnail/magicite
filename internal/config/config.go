// Package config loads and validates magicite's daemon configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	BackendOpenCode = "opencode"
	BackendKiro     = "kiro"
)

// Config is the complete daemon configuration. Load always returns a fully
// validated Config; callers never need to apply defaults themselves.
type Config struct {
	Harness    HarnessConfig   `yaml:"harness"`
	Crew       CrewConfig      `yaml:"crew"`
	Concierge  RoleConfig      `yaml:"concierge"`
	Designer   RoleConfig      `yaml:"designer"`
	Fleet      RoleConfig      `yaml:"fleet"`
	Reviewer   RoleConfig      `yaml:"reviewer"`
	Repairer   RoleConfig      `yaml:"repairer"`
	Welfare    WelfareConfig   `yaml:"welfare"`
	Repos      ReposConfig     `yaml:"repos"`
	Workspaces WorkspaceConfig `yaml:"workspaces"`
}

// HarnessConfig identifies the running harness.
type HarnessConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// CrewConfig holds settings shared by every seat. An empty Backend preserves
// each role's backend selection.
type CrewConfig struct {
	Backend string `yaml:"backend"`
}

// RoleConfig configures an agent role and its named seats.
type RoleConfig struct {
	Enabled        bool         `yaml:"enabled"`
	Agent          string       `yaml:"agent"`
	Backend        string       `yaml:"backend"`
	Model          string       `yaml:"model"`
	KiroModel      string       `yaml:"kiro-model"`
	KiroModelLow   string       `yaml:"kiro-model-low"`
	KiroModelHigh  string       `yaml:"kiro-model-high"`
	EffortLow      string       `yaml:"effort-low"`
	EffortHigh     string       `yaml:"effort-high"`
	KiroEffortLow  string       `yaml:"kiro-effort-low"`
	KiroEffortHigh string       `yaml:"kiro-effort-high"`
	Fallback       string       `yaml:"fallback"`
	KiroFallback   string       `yaml:"kiro-fallback"`
	PollInterval   int          `yaml:"poll-interval"`
	Esper          string       `yaml:"esper"`
	MaxRetries     int          `yaml:"max-retries"`
	Seats          []SeatConfig `yaml:"seats"`
}

// SeatConfig overrides a role's backend, models, or effort for one named seat.
type SeatConfig struct {
	Name           string `yaml:"name"`
	Role           string `yaml:"role"`
	Backend        string `yaml:"backend"`
	Model          string `yaml:"model"`
	KiroModel      string `yaml:"kiro-model"`
	KiroModelLow   string `yaml:"kiro-model-low"`
	KiroModelHigh  string `yaml:"kiro-model-high"`
	EffortLow      string `yaml:"effort-low"`
	EffortHigh     string `yaml:"effort-high"`
	KiroEffortLow  string `yaml:"kiro-effort-low"`
	KiroEffortHigh string `yaml:"kiro-effort-high"`
}

// WelfareConfig controls coordination timeouts in seconds.
type WelfareConfig struct {
	HandoffTimeout int `yaml:"handoff-timeout"`
}

// ReposConfig controls repository discovery and filtering.
type ReposConfig struct {
	Discover string   `yaml:"discover"`
	Roots    []string `yaml:"roots"`
	Include  []string `yaml:"include"`
	Exclude  []string `yaml:"exclude"`
}

// WorkspaceConfig controls per-seat worktree placement below a repository.
type WorkspaceConfig struct {
	Path string `yaml:"path"`
}

// Error identifies the config key that failed to load or validate.
type Error struct {
	Key string
	Err error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return "config " + e.Key
	}
	return fmt.Sprintf("config %s: %v", e.Key, e.Err)
}

// Unwrap returns the underlying filesystem, YAML, or validation error.
func (e *Error) Unwrap() error { return e.Err }

// Default returns the configuration shipped by maduin 0.3.0, expressed as
// daemon-owned typed data.
func Default() Config {
	return Config{
		Harness: HarnessConfig{Name: "maduin", Version: "0.3.0"},
		Concierge: RoleConfig{
			Agent: "slugineer-planner-concierge", Backend: BackendOpenCode,
			Model: "opencode-go/deepseek-v4-pro", KiroModel: "claude-opus-5", KiroFallback: "claude-opus-5",
			Seats: []SeatConfig{{Name: "alexander", Role: "concierge", Model: "opencode-go/deepseek-v4-pro"}},
		},
		Designer: RoleConfig{
			Agent: "slugineer-planner-designer", Backend: BackendOpenCode,
			Model: "opencode-go/deepseek-v4-pro", KiroModel: "claude-opus-5", KiroFallback: "claude-opus-5",
			Seats: []SeatConfig{{Name: "ramuh", Role: "designer", Model: "opencode-go/deepseek-v4-pro"}},
		},
		Fleet: RoleConfig{
			Agent: "slugineer-worker", Backend: BackendOpenCode,
			Model: "opencode/deepseek-v4-flash-free", KiroModel: "gpt-5.6-terra",
			KiroModelLow: "gpt-5.6-luna", KiroModelHigh: "gpt-5.6-terra",
			KiroEffortLow: "medium", KiroEffortHigh: "high", KiroFallback: "gpt-5.6-terra",
			Fallback: "opencode-go/deepseek-v4-flash", PollInterval: 30,
			Seats: []SeatConfig{
				{Name: "ifrit", Role: "implementer", Model: "opencode/deepseek-v4-flash-free"},
				{Name: "shiva", Role: "implementer", Model: "opencode/deepseek-v4-flash-free"},
				{Name: "titan", Role: "implementer", Model: "opencode/deepseek-v4-flash-free"},
			},
		},
		Reviewer: RoleConfig{
			Enabled: true, Agent: "slugineer-reviewer", Backend: BackendOpenCode,
			Esper: "odin", Model: "opencode-go/deepseek-v4-pro", KiroModel: "gpt-5.6-terra", KiroFallback: "gpt-5.6-terra",
			Seats: []SeatConfig{{Name: "odin", Role: "reviewer"}}, MaxRetries: 3,
		},
		Repairer: RoleConfig{
			Enabled: true, Agent: "slugineer-repairer", Backend: BackendOpenCode,
			Esper: "phoenix", Model: "opencode-go/deepseek-v4-pro", KiroModel: "gpt-5.6-terra", KiroFallback: "gpt-5.6-terra",
			Seats: []SeatConfig{{Name: "phoenix", Role: "repairer"}}, MaxRetries: 3,
		},
		Welfare:    WelfareConfig{HandoffTimeout: 120},
		Repos:      ReposConfig{Discover: "explicit", Roots: []string{"/Users/connorfranc/code/magicite"}},
		Workspaces: WorkspaceConfig{Path: "harness/workspaces"},
	}
}

// Role returns a configured role by its runtime name. "implementer" maps to
// the fleet section, preserving maduin's role-section convention.
func (c Config) Role(name string) (RoleConfig, bool) {
	switch name {
	case "concierge":
		return c.Concierge, true
	case "designer":
		return c.Designer, true
	case "implementer", "fleet":
		return c.Fleet, true
	case "reviewer":
		return c.Reviewer, true
	case "repairer":
		return c.Repairer, true
	default:
		return RoleConfig{}, false
	}
}

// Load reads PATH as YAML, overlays supplied keys on Default, and validates
// the resulting complete configuration. A missing file intentionally returns
// the defaults. It performs neither process execution nor network access.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, &Error{Key: "path", Err: err}
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return Default(), nil
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return Config{}, &Error{Key: "config", Err: err}
	}
	root, err := documentRoot(&document)
	if err != nil {
		return Config{}, err
	}
	if err := validateNode(root, configSchema(), ""); err != nil {
		return Config{}, err
	}

	cfg := Default()
	if err := root.Decode(&cfg); err != nil {
		return Config{}, &Error{Key: "config", Err: err}
	}
	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

type valueKind uint8

const (
	mapping valueKind = iota
	sequence
	stringValue
	integer
	boolean
)

type valueSchema struct {
	kind     valueKind
	nullable bool
	children map[string]valueSchema
	element  *valueSchema
}

func configSchema() valueSchema {
	seat := valueSchema{kind: mapping, children: map[string]valueSchema{
		"name": stringSchema(false), "role": stringSchema(false), "backend": stringSchema(false),
		"model": stringSchema(false), "kiro-model": stringSchema(false),
		"kiro-model-low": stringSchema(true), "kiro-model-high": stringSchema(true),
		"effort-low": stringSchema(true), "effort-high": stringSchema(true),
		"kiro-effort-low": stringSchema(true), "kiro-effort-high": stringSchema(true),
	}}
	role := valueSchema{kind: mapping, children: map[string]valueSchema{
		"enabled": booleanSchema(false), "agent": stringSchema(false), "backend": stringSchema(false),
		"model": stringSchema(false), "kiro-model": stringSchema(false),
		"kiro-model-low": stringSchema(true), "kiro-model-high": stringSchema(true),
		"effort-low": stringSchema(true), "effort-high": stringSchema(true),
		"kiro-effort-low": stringSchema(true), "kiro-effort-high": stringSchema(true),
		"fallback": stringSchema(true), "kiro-fallback": stringSchema(false),
		"poll-interval": integerSchema(false), "esper": stringSchema(false), "max-retries": integerSchema(false),
		"seats": {kind: sequence, element: &seat},
	}}
	stringList := valueSchema{kind: sequence, nullable: true, element: ptr(stringSchema(false))}
	return valueSchema{kind: mapping, children: map[string]valueSchema{
		"harness":   {kind: mapping, children: map[string]valueSchema{"name": stringSchema(false), "version": stringSchema(false)}},
		"crew":      {kind: mapping, children: map[string]valueSchema{"backend": stringSchema(true)}},
		"concierge": role, "designer": role, "fleet": role, "reviewer": role, "repairer": role,
		"welfare": {kind: mapping, children: map[string]valueSchema{"handoff-timeout": integerSchema(false)}},
		"repos": {kind: mapping, children: map[string]valueSchema{
			"discover": stringSchema(false), "roots": stringList, "include": stringList, "exclude": stringList,
		}},
		"workspaces": {kind: mapping, children: map[string]valueSchema{"path": stringSchema(false)}},
	}}
}

func ptr(s valueSchema) *valueSchema { return &s }
func stringSchema(nullable bool) valueSchema {
	return valueSchema{kind: stringValue, nullable: nullable}
}
func integerSchema(nullable bool) valueSchema { return valueSchema{kind: integer, nullable: nullable} }
func booleanSchema(nullable bool) valueSchema { return valueSchema{kind: boolean, nullable: nullable} }

func documentRoot(document *yaml.Node) (*yaml.Node, error) {
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil, &Error{Key: "config", Err: errors.New("must contain one YAML document")}
	}
	return document.Content[0], nil
}

func validateNode(node *yaml.Node, schema valueSchema, path string) error {
	key := path
	if key == "" {
		key = "config"
	}
	if node.Tag == "!!null" {
		if schema.nullable {
			return nil
		}
		return &Error{Key: key, Err: errors.New("may not be null")}
	}

	switch schema.kind {
	case mapping:
		if node.Kind != yaml.MappingNode {
			return wrongType(key, "mapping")
		}
		seen := make(map[string]struct{}, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			name, value := node.Content[i], node.Content[i+1]
			if name.Kind != yaml.ScalarNode || name.Tag != "!!str" {
				return &Error{Key: key, Err: errors.New("key must be a string")}
			}
			childPath := name.Value
			if path != "" {
				childPath = path + "." + name.Value
			}
			child, ok := schema.children[name.Value]
			if !ok {
				return &Error{Key: childPath, Err: errors.New("unknown key")}
			}
			if _, duplicate := seen[name.Value]; duplicate {
				return &Error{Key: childPath, Err: errors.New("duplicate key")}
			}
			seen[name.Value] = struct{}{}
			if err := validateNode(value, child, childPath); err != nil {
				return err
			}
		}
	case sequence:
		if node.Kind != yaml.SequenceNode {
			return wrongType(key, "sequence")
		}
		for i, child := range node.Content {
			if err := validateNode(child, *schema.element, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case stringValue:
		if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
			return wrongType(key, "string")
		}
	case integer:
		if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
			return wrongType(key, "integer")
		}
	case boolean:
		if node.Kind != yaml.ScalarNode || node.Tag != "!!bool" {
			return wrongType(key, "boolean")
		}
	}
	return nil
}

func wrongType(key, want string) error {
	return &Error{Key: key, Err: fmt.Errorf("must be a %s", want)}
}

func validateConfig(c Config) error {
	if err := nonEmpty("harness.name", c.Harness.Name); err != nil {
		return err
	}
	if err := nonEmpty("harness.version", c.Harness.Version); err != nil {
		return err
	}
	if c.Crew.Backend != "" && !validBackend(c.Crew.Backend) {
		return invalid("crew.backend", "must be opencode or kiro")
	}
	roles := []struct {
		key, seatRole string
		value         RoleConfig
	}{
		{"concierge", "concierge", c.Concierge}, {"designer", "designer", c.Designer},
		{"fleet", "implementer", c.Fleet}, {"reviewer", "reviewer", c.Reviewer}, {"repairer", "repairer", c.Repairer},
	}
	for _, role := range roles {
		if err := validateRole(role.key, role.seatRole, role.value); err != nil {
			return err
		}
	}
	if c.Fleet.PollInterval <= 0 {
		return invalid("fleet.poll-interval", "must be positive")
	}
	if c.Reviewer.MaxRetries < 0 {
		return invalid("reviewer.max-retries", "must not be negative")
	}
	if c.Repairer.MaxRetries < 0 {
		return invalid("repairer.max-retries", "must not be negative")
	}
	if c.Welfare.HandoffTimeout <= 0 {
		return invalid("welfare.handoff-timeout", "must be positive")
	}
	if c.Repos.Discover != "explicit" && c.Repos.Discover != "project" {
		return invalid("repos.discover", "must be explicit or project")
	}
	for _, list := range []struct {
		key    string
		values []string
	}{{"repos.roots", c.Repos.Roots}, {"repos.include", c.Repos.Include}, {"repos.exclude", c.Repos.Exclude}} {
		for i, value := range list.values {
			if strings.TrimSpace(value) == "" {
				return invalid(fmt.Sprintf("%s[%d]", list.key, i), "must not be empty")
			}
		}
	}
	if err := nonEmpty("workspaces.path", c.Workspaces.Path); err != nil {
		return err
	}
	cleanPath := filepath.Clean(c.Workspaces.Path)
	if filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return invalid("workspaces.path", "must stay below the repository")
	}
	return nil
}

func validateRole(key, seatRole string, role RoleConfig) error {
	if err := nonEmpty(key+".agent", role.Agent); err != nil {
		return err
	}
	if !validBackend(role.Backend) {
		return invalid(key+".backend", "must be opencode or kiro")
	}
	if err := nonEmpty(key+".model", role.Model); err != nil {
		return err
	}
	if err := nonEmpty(key+".kiro-model", role.KiroModel); err != nil {
		return err
	}
	if err := nonEmpty(key+".kiro-fallback", role.KiroFallback); err != nil {
		return err
	}
	if role.Fallback != "" && strings.TrimSpace(role.Fallback) == "" {
		return invalid(key+".fallback", "must not be empty")
	}
	if err := validateEffort(key+".effort-low", role.EffortLow, false); err != nil {
		return err
	}
	if err := validateEffort(key+".effort-high", role.EffortHigh, false); err != nil {
		return err
	}
	if err := validateEffort(key+".kiro-effort-low", role.KiroEffortLow, true); err != nil {
		return err
	}
	if err := validateEffort(key+".kiro-effort-high", role.KiroEffortHigh, true); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(role.Seats))
	for i, seat := range role.Seats {
		prefix := fmt.Sprintf("%s.seats[%d]", key, i)
		if err := nonEmpty(prefix+".name", seat.Name); err != nil {
			return err
		}
		if _, duplicate := seen[seat.Name]; duplicate {
			return invalid(prefix+".name", "must be unique within its role")
		}
		seen[seat.Name] = struct{}{}
		if seat.Role != seatRole {
			return invalid(prefix+".role", "does not match its role section")
		}
		if seat.Backend != "" && !validBackend(seat.Backend) {
			return invalid(prefix+".backend", "must be opencode or kiro")
		}
		for _, field := range []struct{ key, value string }{{"model", seat.Model}, {"kiro-model", seat.KiroModel}, {"kiro-model-low", seat.KiroModelLow}, {"kiro-model-high", seat.KiroModelHigh}} {
			if field.value != "" && strings.TrimSpace(field.value) == "" {
				return invalid(prefix+"."+field.key, "must not be empty")
			}
		}
		if err := validateEffort(prefix+".effort-low", seat.EffortLow, false); err != nil {
			return err
		}
		if err := validateEffort(prefix+".effort-high", seat.EffortHigh, false); err != nil {
			return err
		}
		if err := validateEffort(prefix+".kiro-effort-low", seat.KiroEffortLow, true); err != nil {
			return err
		}
		if err := validateEffort(prefix+".kiro-effort-high", seat.KiroEffortHigh, true); err != nil {
			return err
		}
	}
	return nil
}

func validBackend(value string) bool { return value == BackendOpenCode || value == BackendKiro }

func validateEffort(key, value string, kiro bool) error {
	if value == "" {
		return nil
	}
	valid := map[string]bool{"minimal": true, "low": true, "medium": true, "high": true, "max": true}
	if kiro {
		valid = map[string]bool{"low": true, "medium": true, "high": true, "xhigh": true, "max": true}
	}
	if !valid[strings.ToLower(value)] {
		return invalid(key, "is not supported by its backend")
	}
	return nil
}

func nonEmpty(key, value string) error {
	if strings.TrimSpace(value) == "" {
		return invalid(key, "must not be empty")
	}
	return nil
}

func invalid(key, message string) error { return &Error{Key: key, Err: errors.New(message)} }

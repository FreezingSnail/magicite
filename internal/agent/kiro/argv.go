// Package kiro builds Kiro command arguments and validates session inputs.
package kiro

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Efforts lists the effort levels accepted by Kiro.
var Efforts = []string{"low", "medium", "high", "xhigh", "max"}

// ValidAgent reports whether name identifies an installed agent configuration.
// An empty name selects Kiro's default agent.
func ValidAgent(agentsDir, name string) bool {
	if name == "" {
		return true
	}
	if strings.ContainsAny(name, `/\\`) || strings.Contains(name, "..") {
		return false
	}

	info, err := os.Stat(filepath.Join(agentsDir, name+".json"))
	return err == nil && info.Mode().IsRegular()
}

// ValidModel reports whether model is a non-empty model name without a slash.
func ValidModel(model string) bool {
	return model != "" && !strings.Contains(model, "/")
}

// ValidEffort reports whether effort is an accepted effort level.
func ValidEffort(effort string) bool {
	if effort == "" || strings.Contains(effort, "/") {
		return false
	}
	for _, r := range effort {
		if unicode.IsSpace(r) {
			return false
		}
	}

	lower := strings.ToLower(effort)
	for _, allowed := range Efforts {
		if lower == allowed {
			return true
		}
	}
	return false
}

// RunArgs returns one argv slot per Kiro command argument.
func RunArgs(exe, model, agentName, effort, plan string) []string {
	args := []string{exe, "chat", "--no-interactive"}
	if agentName != "" {
		args = append(args, "--agent", agentName)
	}
	args = append(args, "--model", model)
	if ValidEffort(effort) {
		args = append(args, "--effort", effort)
	}
	return append(args, "--trust-all-tools", plan)
}

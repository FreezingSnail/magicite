package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
)

const (
	scenarioOK      = "scenario-ok"
	scenarioDenied  = "scenario-denied"
	scenarioLimited = "scenario-limited"
	scenarioChild   = "scenario-child"
	scenarioHang    = "scenario-hang"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run":
		run(os.Args[2:])
	case "chat":
		chat(os.Args[2:])
	case "sleep":
		waitUntilKilled()
	case "export":
		export()
	case "session":
		if len(os.Args) < 3 || os.Args[2] != "delete" {
			os.Exit(2)
		}
	default:
		os.Exit(2)
	}
}

func run(args []string) {
	scenario := scenario(args)
	prepare()
	switch scenario {
	case scenarioOK:
		line(map[string]any{"type": "session.created", "sessionID": "ses_fake"})
		line(map[string]any{"type": "step_finish", "part": map[string]any{"reason": "stop"}})
	case scenarioDenied:
		line(map[string]any{"type": "tool_use", "part": map[string]any{"state": map[string]any{"status": "error"}}})
	case scenarioLimited:
		line(map[string]any{"type": "session.error", "error": map[string]any{"message": "usage limit exceeded"}})
	case scenarioChild:
		spawnChild()
		line(map[string]any{"type": "session.created", "sessionID": "ses_fake"})
		waitUntilKilled()
	case scenarioHang:
		waitUntilKilled()
	default:
		os.Exit(2)
	}
}

func chat(args []string) {
	scenario := scenario(args)
	prepare()
	switch scenario {
	case scenarioOK:
		fmt.Print("\x1b[32mcompleted fake agent run\x1b[0m\n")
	case scenarioDenied:
		fmt.Print("\x1b[31mauthentication denied\x1b[0m\n")
	case scenarioLimited:
		fmt.Print("\x1b[31musage limit exceeded\x1b[0m\n")
	case scenarioChild:
		spawnChild()
		fmt.Print("\x1b[32mchild started\x1b[0m\n")
		waitUntilKilled()
	case scenarioHang:
		waitUntilKilled()
	default:
		os.Exit(2)
	}
}

func prepare() {
	mustWrite(".fakeagent.pid", fmt.Sprintf("%d\n", os.Getpid()))
	mustWrite("tracked.txt", "modified by fake agent\n")
	mustWrite("untracked.txt", "untracked by fake agent\n")
}

func spawnChild() {
	child := exec.Command(os.Args[0], "sleep")
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
	mustWrite(".fakeagent-child.pid", fmt.Sprintf("%d\n", child.Process.Pid))
}

func waitUntilKilled() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	<-signals
}

func scenario(args []string) string {
	for _, arg := range args {
		switch arg {
		case scenarioOK, scenarioDenied, scenarioLimited, scenarioChild, scenarioHang:
			return arg
		}
	}
	return ""
}

func line(value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		os.Exit(2)
	}
	fmt.Println(string(encoded))
}

func export() {
	line(map[string]any{
		"messages": []any{map[string]any{
			"info": map[string]any{
				"summary": map[string]any{
					"diffs": []any{
						map[string]any{"file": "tracked.txt", "patch": "diff --git a/tracked.txt b/tracked.txt\n", "additions": 1, "deletions": 1, "status": "modified"},
						map[string]any{"file": "untracked.txt", "patch": "diff --git a/untracked.txt b/untracked.txt\n", "additions": 1, "deletions": 0, "status": "untracked"},
					},
				},
			},
		}},
	})
}

func mustWrite(name, contents string) {
	if err := os.WriteFile(filepath.Clean(name), []byte(contents), 0o644); err != nil {
		os.Exit(2)
	}
}

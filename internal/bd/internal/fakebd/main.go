// Command fakebd is the test-only bd process double.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type entry struct {
	Match   []string `json:"match"`
	Stdout  string   `json:"stdout"`
	Stderr  string   `json:"stderr"`
	Exit    int      `json:"exit"`
	DelayMS int      `json:"delay_ms"`
}

func main() {
	argv := os.Args[1:]
	if err := recordCall(argv); err != nil {
		fmt.Fprintf(os.Stderr, "record fake bd call: %v\n", err)
		os.Exit(2)
	}

	entries, err := readScript()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read fake bd script: %v\n", err)
		os.Exit(2)
	}
	for _, candidate := range entries {
		if !matches(argv, candidate.Match) {
			continue
		}
		if candidate.DelayMS > 0 {
			time.Sleep(time.Duration(candidate.DelayMS) * time.Millisecond)
		}
		fmt.Fprint(os.Stdout, candidate.Stdout)
		fmt.Fprint(os.Stderr, candidate.Stderr)
		os.Exit(candidate.Exit)
	}

	fmt.Fprintf(os.Stderr, "unmatched fake bd argv: %q\n", argv)
	os.Exit(3)
}

func recordCall(argv []string) error {
	dir := filepath.Join("bdfake", "calls")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("%d-%d.json", os.Getpid(), time.Now().UnixNano())
	data, err := json.Marshal(argv)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), data, 0o644)
}

func readScript() ([]entry, error) {
	data, err := os.ReadFile(filepath.Join("bdfake", "script.json"))
	if err != nil {
		return nil, err
	}
	var entries []entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func matches(argv, match []string) bool {
	if len(match) == 0 {
		return true
	}
	for start := 0; start+len(match) <= len(argv); start++ {
		matched := true
		for index := range match {
			if argv[start+index] != match[index] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

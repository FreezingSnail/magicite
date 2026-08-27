// Package bd provides the sole argv-only bridge to the bd command-line tool.
package bd

import (
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"

	magicexec "github.com/FreezingSnail/magicite/internal/exec"
	"github.com/FreezingSnail/magicite/internal/logging"
)

// Result contains a bd invocation's exit status and independent output streams.
type Result struct {
	ExitCode       int
	Stdout, Stderr []byte
}

// Client invokes bd in one repository root.
type Client struct {
	Program string
	Root    string
	Log     *logging.Logger
}

// New creates a client for root. An empty program uses bd from PATH.
func New(program, root string) (*Client, error) {
	if root == "" {
		return nil, fmt.Errorf("bd root is empty")
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat bd root %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("bd root %q is not a directory", root)
	}
	if program == "" {
		program = "bd"
	}
	return &Client{Program: program, Root: root}, nil
}

// Run invokes bd with args in the client's root. A command exit status is data;
// failures to start or a terminated context are returned as errors.
func (c *Client) Run(ctx context.Context, args ...string) (Result, error) {
	argv := make([]string, 0, len(args)+2)
	argv = append(argv, "-C", c.Root)
	argv = append(argv, args...)

	stdout, stderr, exitCode, runErr := magicexec.Run(ctx, c.Root, c.Program, argv...)
	result := Result{ExitCode: exitCode, Stdout: stdout, Stderr: stderr}
	if runErr == nil {
		return result, nil
	}

	var exitErr *osexec.ExitError
	if errors.As(runErr, &exitErr) && ctx.Err() == nil {
		return result, nil
	}
	return result, runErr
}

func (c *Client) logEvent(level logging.Level, fields map[string]any) {
	if c.Log != nil {
		c.Log.Event(level, "bd.run", fields)
		return
	}
	logging.Event(level, "bd.run", fields)
}

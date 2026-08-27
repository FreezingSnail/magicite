// Package exec runs context-bounded external commands.
package exec

import (
	"bytes"
	"context"
	"errors"
	osexec "os/exec"
)

// Error reports a command start, execution, or context-cancellation failure.
type Error struct {
	err error
}

// Error returns the underlying failure message.
func (e *Error) Error() string {
	return e.err.Error()
}

// Unwrap returns the underlying command or context error.
func (e *Error) Unwrap() error {
	return e.err
}

// Run executes name with args in dir. It captures standard output and standard
// error separately. The child receives an explicitly empty environment.
func Run(ctx context.Context, dir, name string, args ...string) (stdout, stderr []byte, exitCode int, runErr *Error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, -1, &Error{err: err}
	}

	cmd := osexec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = []string{}

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()
	stdout, stderr = out.Bytes(), errOut.Bytes()
	if err == nil {
		return stdout, stderr, 0, nil
	}

	exitCode = -1
	var exitErr *osexec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}

	return stdout, stderr, exitCode, &Error{err: err}
}

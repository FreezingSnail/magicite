package bd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/FreezingSnail/magicite/internal/logging"
)

// CreateRequest contains the mutable fields accepted by Create.
type CreateRequest struct {
	Title, Type, Parent, Body, Design, Acceptance, Priority string
	Labels                                                  []string
}

// UpdateRequest contains the mutable fields accepted by Update.
type UpdateRequest struct {
	Status, Assignee, Body, Design, Acceptance, Defer string
	AddLabels, RemoveLabels                           []string
	Claim                                             bool
}

// Create creates a bead and returns its id.
func (c *Client) Create(ctx context.Context, req CreateRequest) (string, error) {
	if req.Title == "" {
		return "", c.localError("create", nil, KindUsage, "title is empty")
	}

	bodyFile, removeBody, err := tempText(req.Body)
	if err != nil {
		return "", c.localError("create", nil, KindFailed, err.Error())
	}
	defer removeBody()

	designFile, removeDesign, err := tempText(req.Design)
	if err != nil {
		return "", c.localError("create", nil, KindFailed, err.Error())
	}
	defer removeDesign()

	args := ArgsCreate(CreateArgs{
		Title: req.Title, Type: req.Type, Parent: req.Parent, BodyFile: bodyFile,
		DesignFile: designFile, Acceptance: req.Acceptance, Priority: req.Priority,
		Labels: req.Labels,
	})
	result, err := c.mutate(ctx, args)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(result.Stdout))
	if id == "" || strings.ContainsAny(id, "\r\n") {
		return "", c.localError("create", args[1:], KindFailed, "create returned an empty or multiline id")
	}
	return id, nil
}

// Update applies every non-empty field in req. An empty request does nothing.
func (c *Client) Update(ctx context.Context, id string, req UpdateRequest) error {
	if updateEmpty(req) {
		return nil
	}

	bodyFile, removeBody, err := tempText(req.Body)
	if err != nil {
		return c.localError("update", nil, KindFailed, err.Error())
	}
	defer removeBody()

	designFile, removeDesign, err := tempText(req.Design)
	if err != nil {
		return c.localError("update", nil, KindFailed, err.Error())
	}
	defer removeDesign()

	_, err = c.mutate(ctx, ArgsUpdate(id, UpdateArgs{
		Status: req.Status, Assignee: req.Assignee, BodyFile: bodyFile, DesignFile: designFile,
		Acceptance: req.Acceptance, Defer: req.Defer, AddLabels: req.AddLabels,
		RemoveLabels: req.RemoveLabels, Claim: req.Claim,
	}))
	return err
}

// Claim claims id for the current bd identity.
func (c *Client) Claim(ctx context.Context, id string) error {
	return c.Update(ctx, id, UpdateRequest{Claim: true})
}

// Release returns id to open status.
func (c *Client) Release(ctx context.Context, id string) error {
	return c.Update(ctx, id, UpdateRequest{Status: "open"})
}

// Close closes id with a file-backed reason.
func (c *Client) Close(ctx context.Context, id, reason string) error {
	file, remove, err := tempText(reason)
	if err != nil {
		return c.localError("close", nil, KindFailed, err.Error())
	}
	defer remove()
	_, err = c.mutate(ctx, ArgsClose(id, file))
	return err
}

// Comment adds a file-backed comment to id.
func (c *Client) Comment(ctx context.Context, id, text string) error {
	file, remove, err := tempText(text)
	if err != nil {
		return c.localError("comment", nil, KindFailed, err.Error())
	}
	defer remove()
	_, err = c.mutate(ctx, ArgsComment(id, file))
	return err
}

// LabelAdd adds label to id.
func (c *Client) LabelAdd(ctx context.Context, id, label string) error {
	_, err := c.mutate(ctx, ArgsLabelAdd(id, label))
	return err
}

// LabelRemove removes label from id.
func (c *Client) LabelRemove(ctx context.Context, id, label string) error {
	_, err := c.mutate(ctx, ArgsLabelRemove(id, label))
	return err
}

// Defer defers id until the caller-supplied date.
func (c *Client) Defer(ctx context.Context, id, until string) error {
	_, err := c.mutate(ctx, ArgsDefer(id, until))
	return err
}

// Undefer clears id's deferral date.
func (c *Client) Undefer(ctx context.Context, id string) error {
	_, err := c.mutate(ctx, ArgsUndefer(id))
	return err
}

// DepAdd records that id depends on dependsOn.
func (c *Client) DepAdd(ctx context.Context, id, dependsOn string) error {
	_, err := c.mutate(ctx, ArgsDepAdd(id, dependsOn))
	return err
}

func updateEmpty(req UpdateRequest) bool {
	return req.Status == "" && req.Assignee == "" && req.Body == "" && req.Design == "" &&
		req.Acceptance == "" && req.Defer == "" && len(req.AddLabels) == 0 &&
		len(req.RemoveLabels) == 0 && !req.Claim
}

func (c *Client) mutate(ctx context.Context, args []string) (Result, error) {
	if len(args) == 0 {
		return Result{}, c.localError("", nil, KindUsage, "missing bd subcommand")
	}
	op, operands := args[0], args[1:]
	for _, arg := range args {
		if strings.ContainsAny(arg, "\r\n") {
			return Result{}, c.localError(op, operands, KindUsage, "newline in bd argument")
		}
	}

	result, runErr := c.Run(ctx, args...)
	if runErr != nil {
		// Run already recorded its one event for launch and context failures.
		return result, runErr
	}
	classified := Classify(op, operands, result)
	if classified != nil {
		if Loggable(classified) {
			c.logMutation(classified)
		}
		return result, classified
	}
	return result, nil
}

func (c *Client) localError(op string, args []string, kind Kind, detail string) error {
	exitCode := 1
	if kind == KindUsage {
		exitCode = 2
	}
	err := Classify(op, args, Result{ExitCode: exitCode, Stderr: []byte(detail)})
	if Loggable(err) {
		c.logMutation(err)
	}
	return err
}

func (c *Client) logMutation(err error) {
	c.logEvent(logging.Error, map[string]any{"error": err.Error()})
}

func tempText(text string) (string, func(), error) {
	if text == "" {
		return "", func() {}, nil
	}

	file, err := os.CreateTemp("", "magicite-bd-")
	if err != nil {
		return "", nil, fmt.Errorf("create bd text file: %w", err)
	}
	path := file.Name()
	var once sync.Once
	remove := func() {
		once.Do(func() { _ = os.Remove(path) })
	}
	complete := false
	defer func() {
		if !complete {
			_ = file.Close()
			remove()
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return "", nil, fmt.Errorf("chmod bd text file: %w", err)
	}
	if _, err := file.WriteString(text); err != nil {
		return "", nil, fmt.Errorf("write bd text file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", nil, fmt.Errorf("close bd text file: %w", err)
	}
	complete = true
	return path, remove, nil
}

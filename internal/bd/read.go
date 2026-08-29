package bd

import (
	"context"

	"github.com/FreezingSnail/magicite/internal/logging"
)

// Show returns the bead identified by id.
func (c *Client) Show(ctx context.Context, id string) (Bead, error) {
	args := ArgsShow(id)
	result, err := c.read(ctx, "show", args)
	if err != nil {
		return Bead{}, err
	}

	beads, err := DecodeBeads(result.Stdout)
	if err != nil {
		return Bead{}, c.decodeError("show", args, result, err)
	}
	if len(beads) == 0 {
		return Bead{}, &Error{Op: "show", Args: append([]string(nil), args...), Kind: KindNotFound, ExitCode: result.ExitCode, Detail: "bead not found"}
	}
	return beads[0], nil
}

// List returns beads, optionally including closed beads.
func (c *Client) List(ctx context.Context, all bool) ([]Bead, error) {
	args := ArgsList(all)
	result, err := c.read(ctx, "list", args)
	if err != nil {
		return nil, err
	}

	beads, err := DecodeBeads(result.Stdout)
	if err != nil {
		return nil, c.decodeError("list", args, result, err)
	}
	return beads, nil
}

// Ready returns ready non-epic, non-human beads.
func (c *Client) Ready(ctx context.Context) ([]Bead, error) {
	args := ArgsReady()
	result, err := c.read(ctx, "ready", args)
	if err != nil {
		return nil, err
	}

	beads, err := DecodeBeads(result.Stdout)
	if err != nil {
		return nil, c.decodeError("ready", args, result, err)
	}
	return beads, nil
}

// Query returns beads matching q, optionally including closed beads.
func (c *Client) Query(ctx context.Context, q string, all bool) ([]Bead, error) {
	args := ArgsQuery(q, all)
	result, err := c.read(ctx, "query", args)
	if err != nil {
		return nil, err
	}

	beads, err := DecodeBeads(result.Stdout)
	if err != nil {
		return nil, c.decodeError("query", args, result, err)
	}
	return beads, nil
}

// Deps returns dependencies of id in bd's order.
func (c *Client) Deps(ctx context.Context, id string) ([]Dependency, error) {
	args := ArgsDepList(id)
	result, err := c.read(ctx, "dep", args)
	if err != nil {
		return nil, err
	}

	dependencies, err := DecodeDeps(result.Stdout)
	if err != nil {
		return nil, c.decodeError("dep", args, result, err)
	}
	return dependencies, nil
}

// Labels returns labels attached to id in bd's order.
func (c *Client) Labels(ctx context.Context, id string) ([]string, error) {
	args := ArgsLabelList(id)
	result, err := c.read(ctx, "label", args)
	if err != nil {
		return nil, err
	}

	labels, err := DecodeLabels(result.Stdout)
	if err != nil {
		return nil, c.decodeError("label", args, result, err)
	}
	return labels, nil
}

func (c *Client) read(ctx context.Context, op string, args []string) (Result, error) {
	result, runErr := c.Run(ctx, args...)
	classified := Classify(op, args, result)
	if runErr != nil {
		kind := KindFailed
		if ctx.Err() != nil {
			kind = KindUnavailable
		}
		err := &Error{Op: op, Args: append([]string(nil), args...), Kind: kind, ExitCode: result.ExitCode, Detail: runErr.Error(), Cause: runErr}
		c.logReadError(err)
		return Result{}, err
	}
	if classified != nil {
		c.logReadError(classified)
		return Result{}, classified
	}
	return result, nil
}

func (c *Client) decodeError(op string, args []string, result Result, cause error) error {
	err := &Error{Op: op, Args: append([]string(nil), args...), Kind: KindFailed, ExitCode: result.ExitCode, Detail: cause.Error(), Cause: cause}
	c.logReadError(err)
	return err
}

func (c *Client) logReadError(err error) {
	if !Loggable(err) {
		return
	}
	classified, ok := err.(*Error)
	if !ok {
		return
	}
	c.logEvent(logging.Error, map[string]any{
		"op":     classified.Op,
		"exit":   classified.ExitCode,
		"kind":   classified.Kind.String(),
		"detail": classified.Detail,
	})
}

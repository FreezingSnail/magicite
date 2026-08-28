package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/connorfranc/magicite/internal/agent"
	executil "github.com/connorfranc/magicite/internal/exec"
)

type exportDocument struct {
	Messages []struct {
		Info struct {
			Summary struct {
				Diffs []agent.FileDiff `json:"diffs"`
			} `json:"summary"`
		} `json:"info"`
	} `json:"messages"`
}

// Diff exports and flattens OpenCode's recorded session diffs.
func (a *Adapter) Diff(ctx context.Context, handle agent.Handle) ([]agent.FileDiff, error) {
	state, ok := a.state(handle)
	if !ok {
		return nil, fmt.Errorf("%w: %s", agent.ErrUnknownHandle, handle)
	}
	state.mu.RLock()
	sessionID := state.sessionID
	workdir := state.workdir
	state.mu.RUnlock()
	if sessionID == "" {
		return nil, fmt.Errorf("opencode session ID unavailable for %s", handle)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	stdout, stderr, _, runErr := executil.Run(ctx, workdir, a.executable, "export", sessionID)
	if runErr != nil {
		return nil, fmt.Errorf("opencode export %q: %w: %s", sessionID, runErr, strings.TrimSpace(string(stderr)))
	}
	diffs, err := parseExport(stdout)
	if err != nil {
		return nil, fmt.Errorf("parse opencode export %q: %w", sessionID, err)
	}
	return diffs, nil
}

func parseExport(data []byte) ([]agent.FileDiff, error) {
	var document exportDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	var diffs []agent.FileDiff
	for _, message := range document.Messages {
		diffs = append(diffs, message.Info.Summary.Diffs...)
	}
	return diffs, nil
}

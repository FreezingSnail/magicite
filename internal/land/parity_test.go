package land

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/parity"
)

func TestMaduinPipelineParity(t *testing.T) {
	for _, name := range pipelineParityNames() {
		t.Run(name, func(t *testing.T) {
			if name == "maduin-test-repair-plan-rebases-not-merges" {
				c := rebaseTestContext()
				pipeline := rebaseTestPipeline(t, newFakeRunner(fakeReply{Prefix: []string{"-C", c.Worktree, "rebase", c.Integration, c.Branch}}), nil)
				if result, err := pipeline.rebase(context.Background(), c); result != rebaseOK || err != nil {
					t.Fatalf("rebase() = (%v, %v)", result, err)
				}
				return
			}
			pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: newFakeRunner()})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if got := strings.Join(pipeline.gateArgv, " "); got != "make check" {
				t.Fatalf("default gate = %q", got)
			}
		})
	}
}

func pipelineParityNames() []string {
	counterparts := parity.OrchestrationCounterparts()
	names := make([]string, 0)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, "TestMaduinPipelineParity/") || strings.HasPrefix(testName, "TestMaduinDispatchParity/") && name == "maduin-test-repair-plan-rebases-not-merges" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

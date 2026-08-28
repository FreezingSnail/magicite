package parity

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/connorfranc/magicite/internal/testenv"
)

const parityBudget = time.Minute

var parityStarted time.Time

func TestMain(m *testing.M) {
	parityStarted = time.Now()
	os.Exit(m.Run())
}

func TestParityCoverage(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := LoadLedger()
	if err != nil {
		t.Fatal(err)
	}

	counterparts := SubstrateCounterparts()
	for name, counterpart := range OrchestrationCounterparts() {
		if existing, duplicate := counterparts[name]; duplicate {
			t.Fatalf("duplicate counterpart for %q: %q and %q", name, existing, counterpart)
		}
		counterparts[name] = counterpart
	}
	for name := range counterparts {
		if _, ok := catalog.ByName[name]; !ok {
			t.Errorf("counterpart names unknown invariant %q", name)
		}
	}

	report := Coverage(catalog, ledger, counterparts)
	if len(report.Covered)+len(report.Diverged) != len(catalog.Entries) || len(report.Missing) != 0 {
		var output bytes.Buffer
		if err := WriteReport(&output, report); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("parity coverage incomplete:\n%s", output.String())
	}
}

func TestParityOffline(t *testing.T) {
	env := testenv.New(t)
	testenv.NewBD(t, env)
	testenv.NewAgent(t, env, "kiro")
	testenv.NewAgent(t, env, "opencode")

	command := exec.Command(os.Args[0], "-test.run=^TestParityOfflineProbe$")
	command.Env = append(env.Env(), "MAGICITE_PARITY_OFFLINE_PROBE=1", "MAGICITE_PARITY_OFFLINE_ROOT="+env.Root)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("offline probe: %v\n%s", err, output)
	}
}

func TestParityOfflineProbe(t *testing.T) {
	if os.Getenv("MAGICITE_PARITY_OFFLINE_PROBE") == "" {
		return
	}
	root := os.Getenv("MAGICITE_PARITY_OFFLINE_ROOT")
	if root == "" {
		t.Fatal("offline probe root is unset")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolve offline probe root %q: %v", root, err)
	}
	for _, name := range []string{"bd", "kiro", "opencode"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Errorf("resolve %s at %q: %v", name, path, err)
			continue
		}
		relative, err := filepath.Rel(resolvedRoot, resolved)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			t.Errorf("%s resolves outside test root: %q", name, resolved)
		}
	}
}

func TestParityBudget(t *testing.T) {
	elapsed := time.Since(parityStarted)
	if elapsed > parityBudget {
		t.Fatalf("parity suite exceeded %s: %s", parityBudget, elapsed)
	}
}

package parity

import (
	"bufio"
	"bytes"
	"fmt"
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

const maduinPathEnv = "MADUIN_PATH"

func TestCatalogDrift(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	path := configuredMaduinPath()
	if !maduinCheckoutAvailable(path) {
		t.Skip(catalogDriftSkipReason(path))
	}
	run := func(args ...string) ([]byte, error) {
		return runMaduinGit(path, args...)
	}
	if err := checkCatalogDrift(catalog, path, run); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogDriftDetectsMismatches(t *testing.T) {
	revision := "0123456789012345678901234567890123456789"
	catalog := Catalog{Revision: revision, Entries: []Invariant{{Name: "one"}}}

	t.Run("revision", func(t *testing.T) {
		run := func(args ...string) ([]byte, error) {
			return []byte("abcdefabcdefabcdefabcdefabcdefabcdefabcd\n"), nil
		}
		err := checkCatalogDrift(catalog, "maduin", run)
		if err == nil || !strings.Contains(err.Error(), "catalog revision") {
			t.Fatalf("checkCatalogDrift() error = %v, want revision mismatch", err)
		}
	})
	t.Run("count", func(t *testing.T) {
		run := func(args ...string) ([]byte, error) {
			if args[0] == "rev-parse" {
				return []byte(revision + "\n"), nil
			}
			return []byte("(ert-deftest maduin-test-one () nil)\n(ert-deftest maduin-test-two () nil)\n"), nil
		}
		err := checkCatalogDrift(catalog, "maduin", run)
		if err == nil || !strings.Contains(err.Error(), "catalog count") {
			t.Fatalf("checkCatalogDrift() error = %v, want count mismatch", err)
		}
	})
}

func TestCatalogDriftSkipsWithoutCheckout(t *testing.T) {
	path := filepath.Join(dataPath(catalogFile), "maduin-unavailable")
	if maduinCheckoutAvailable(path) {
		t.Fatalf("maduinCheckoutAvailable(%q) = true, want false", path)
	}
	if got := catalogDriftSkipReason(path); !strings.Contains(got, path) {
		t.Fatalf("catalogDriftSkipReason() = %q, want path %q", got, path)
	}
}

func configuredMaduinPath() string {
	if path := os.Getenv(maduinPathEnv); path != "" {
		return path
	}
	return filepath.Clean(filepath.Join(filepath.Dir(dataPath(catalogFile)), "..", "..", "..", "..", "maduin"))
}

func maduinCheckoutAvailable(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

func catalogDriftSkipReason(path string) string {
	return fmt.Sprintf("catalog drift check skipped: wanted Maduin checkout at %q", path)
}

func checkCatalogDrift(catalog Catalog, path string, run func(...string) ([]byte, error)) error {
	head, err := run("rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("read Maduin HEAD at %q: %w", path, err)
	}
	if got := strings.TrimSpace(string(head)); got != catalog.Revision {
		return fmt.Errorf("catalog revision %q does not match Maduin HEAD %q", catalog.Revision, got)
	}
	source, err := run("show", catalog.Revision+":harness/maduin-test.el")
	if err != nil {
		return fmt.Errorf("read Maduin tests at %q: %w", path, err)
	}
	if got, want := countMaduinInvariants(source), len(catalog.Entries); got != want {
		return fmt.Errorf("catalog count %d does not match Maduin invariant count %d", want, got)
	}
	return nil
}

func runMaduinGit(path string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", path}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func countMaduinInvariants(source []byte) int {
	scanner := bufio.NewScanner(bytes.NewReader(source))
	count := 0
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "(ert-deftest maduin-test-") {
			count++
		}
	}
	return count
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

type parityMutationSample struct {
	name        string
	invariant   string
	counterpart string
}

var parityMutationSamples = []parityMutationSample{
	{"config Kiro tier", "maduin-test-config-difficulty-model-kiro-tiers", "TestMaduinConfigParity/maduin-test-config-difficulty-model-kiro-tiers"},
	{"bd semantic negative", "maduin-test-backcompat-single-repo-claim-show-close-propagate", "TestMaduinBDParity/maduin-test-backcompat-single-repo-claim-show-close-propagate"},
	{"repo bead routing", "maduin-test-repo-lookups-get-and-bead-routing", "TestMaduinRepoParity/maduin-test-repo-lookups-get-and-bead-routing"},
	{"workspace sync seam", "maduin-test-workspace-repo-life-sync-uses-repo-branch-and-seam", "TestMaduinWorktreeParity/maduin-test-workspace-repo-life-sync-uses-repo-branch-and-seam"}, {"logging level filter", "maduin-test-log-respects-level", "TestMaduinLoggingParity/maduin-test-log-respects-level"},
	{"main daemon lifecycle", "maduin-test-main-stop-tears-down", "TestMaduinOrchestrationParity/maduin-test-main-stop-tears-down"},
	{"designer repo dispatch", "maduin-test-designer-design-and-epic-dispatch-repo", "TestMaduinOrchestrationParity/maduin-test-designer-design-and-epic-dispatch-repo"},
	{"dispatch queue", "maduin-test-dispatch-queue-round-robin", "TestMaduinDispatchParity/maduin-test-dispatch-queue-round-robin"},
	{"dispatch spawn", "maduin-test-dispatch-spawn-high-uses-terra", "TestMaduinDispatchParity/maduin-test-dispatch-spawn-high-uses-terra"},
	{"dispatch outcome", "maduin-test-dispatch-completion-requires-task-provenance", "TestMaduinDispatchParity/maduin-test-dispatch-completion-requires-task-provenance"},
	{"agent session", "maduin-test-session-completion-hook-runs-once", "TestMaduinAgentParity/maduin-test-session-completion-hook-runs-once"},
	{"Kiro argv", "maduin-test-kiro-run-uses-required-argv-and-workdir", "TestMaduinAgentParity/maduin-test-kiro-run-uses-required-argv-and-workdir"},
	{"land task stamp", "maduin-test-pipeline-task-landed-requires-exact-trailer", "TestMaduinPipelineParity/maduin-test-pipeline-task-landed-requires-exact-trailer"},
	{"stamp round trip", "maduin-test-stamp-parse-roundtrip", "TestMaduinStampParity/maduin-test-stamp-parse-roundtrip"},
	{"review disabled close", "maduin-test-review-disabled-gate-still-closes-completed-epic", "TestMaduinReviewParity/maduin-test-review-disabled-gate-still-closes-completed-epic"},
}

func TestParityMutationSamplesRemainBound(t *testing.T) {
	counterparts := SubstrateCounterparts()
	for invariant, counterpart := range OrchestrationCounterparts() {
		counterparts[invariant] = counterpart
	}
	for _, sample := range parityMutationSamples {
		if got := counterparts[sample.invariant]; got != sample.counterpart {
			t.Errorf("%s counterpart = %q, want %q", sample.name, got, sample.counterpart)
		}
	}
}

func TestBlanketDivergencesArePending(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := LoadLedger()
	if err != nil {
		t.Fatal(err)
	}
	pending, err := PendingDomains()
	if err != nil {
		t.Fatal(err)
	}
	permanent := map[string]struct{}{"cockpit": {}, "terminal": {}}
	for _, divergence := range ledger.Reasons() {
		if _, domain := catalog.ByDomain[divergence.Target]; !domain {
			continue
		}
		if _, exempt := permanent[divergence.Target]; exempt {
			continue
		}
		if _, listed := pending[divergence.Target]; !listed {
			t.Errorf("blanket divergence %q is not pending conversion", divergence.Target)
		}
	}
	for domain := range pending {
		if _, blanket := ledger.byTarget[domain]; !blanket {
			t.Errorf("pending domain %q has no blanket divergence", domain)
		}
	}
}

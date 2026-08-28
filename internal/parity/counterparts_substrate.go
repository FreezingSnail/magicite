package parity

import "fmt"

// SubstrateCounterparts maps package-local parity subtests. Config and logging
// use explicit rows so Bindings can enforce one assertion per invariant.
func SubstrateCounterparts() map[string]string {
	counterparts := map[string]string{
		"maduin-test-bd-json-decode-parity":                                  "TestMaduinBDParity/maduin-test-bd-json-decode-parity",
		"maduin-test-bd-json-decode-fallback":                                "TestMaduinBDParity/maduin-test-bd-json-decode-fallback",
		"maduin-test-bd-json-decode-garbage":                                 "TestMaduinBDParity/maduin-test-bd-json-decode-garbage",
		"maduin-test-bd-json-data-array":                                     "TestMaduinBDParity/maduin-test-bd-json-data-array",
		"maduin-test-bd-json-data-non-json":                                  "TestMaduinBDParity/maduin-test-bd-json-data-non-json",
		"maduin-test-bd-json-data-empty":                                     "TestMaduinBDParity/maduin-test-bd-json-data-empty",
		"maduin-test-bd-json-data-object":                                    "TestMaduinBDParity/maduin-test-bd-json-data-object",
		"maduin-test-bd-close-file-symbols-are-gone":                         "TestMaduinBDParity/maduin-test-bd-close-file-symbols-are-gone",
		"maduin-test-bd-remember-and-forget":                                 "TestMaduinBDParity/maduin-test-bd-remember-and-forget",
		"maduin-test-bd-in-progress-query-includes-epics":                    "TestMaduinBDParity/maduin-test-bd-in-progress-query-includes-epics",
		"maduin-test-bd-repo-worktree-helpers-route-and-parse":               "TestMaduinBDParity/maduin-test-bd-repo-worktree-helpers-route-and-parse",
		"maduin-test-bd-repo-worktree-isolates-real-git-repositories":        "TestMaduinBDParity/maduin-test-bd-repo-worktree-isolates-real-git-repositories",
		"maduin-test-backcompat-single-repo-canned-snapshot-and-status-argv": "TestMaduinBDParity/maduin-test-backcompat-single-repo-canned-snapshot-and-status-argv",
		"maduin-test-backcompat-single-repo-ready-order-and-gate-hold":       "TestMaduinBDParity/maduin-test-backcompat-single-repo-ready-order-and-gate-hold",
		"maduin-test-backcompat-single-repo-claim-show-close-propagate":      "TestMaduinBDParity/maduin-test-backcompat-single-repo-claim-show-close-propagate",

		"maduin-test-repo-admits-git-beads-worktree-roots":                   "TestMaduinRepoParity/maduin-test-repo-admits-git-beads-worktree-roots",
		"maduin-test-repo-falls-back-to-ambient-repository":                  "TestMaduinRepoParity/maduin-test-repo-falls-back-to-ambient-repository",
		"maduin-test-repo-fallback-respects-explicit-and-filters":            "TestMaduinRepoParity/maduin-test-repo-fallback-respects-explicit-and-filters",
		"maduin-test-repo-include-filter":                                    "TestMaduinRepoParity/maduin-test-repo-include-filter",
		"maduin-test-repo-explicit-roots-ignore-discovery":                   "TestMaduinRepoParity/maduin-test-repo-explicit-roots-ignore-discovery",
		"maduin-test-repo-names-prefixes-and-cache-are-stable":               "TestMaduinRepoParity/maduin-test-repo-names-prefixes-and-cache-are-stable",
		"maduin-test-repo-prefix-sources-validate-and-fall-back":             "TestMaduinRepoParity/maduin-test-repo-prefix-sources-validate-and-fall-back",
		"maduin-test-repo-malformed-config-and-records-never-signal":         "TestMaduinRepoParity/maduin-test-repo-malformed-config-and-records-never-signal",
		"maduin-test-repo-lookups-get-and-bead-routing":                      "TestMaduinRepoParity/maduin-test-repo-lookups-get-and-bead-routing",
		"maduin-test-repo-current-project-then-directory":                    "TestMaduinRepoParity/maduin-test-repo-current-project-then-directory",
		"maduin-test-repo-read-completes-and-empty-registry-errors":          "TestMaduinRepoParity/maduin-test-repo-read-completes-and-empty-registry-errors",
		"maduin-test-multirepo-isolation-workspace-paths-use-distinct-roots": "TestMaduinRepoParity/maduin-test-multirepo-isolation-workspace-paths-use-distinct-roots",
		"maduin-test-multirepo-isolation-land-target-and-provenance":         "TestMaduinRepoParity/maduin-test-multirepo-isolation-land-target-and-provenance",
		"maduin-test-multirepo-isolation-review-drift-stays-in-a":            "TestMaduinRepoParity/maduin-test-multirepo-isolation-review-drift-stays-in-a",
		"maduin-test-multirepo-isolation-failure-warns-once-b-dispatches":    "TestMaduinRepoParity/maduin-test-multirepo-isolation-failure-warns-once-b-dispatches",
		"maduin-test-multirepo-isolation-global-cap-is-three-sessions":       "TestMaduinRepoParity/maduin-test-multirepo-isolation-global-cap-is-three-sessions",
		"maduin-test-multirepo-isolation-cockpit-rows-are-contiguous":        "TestMaduinRepoParity/maduin-test-multirepo-isolation-cockpit-rows-are-contiguous",

		"maduin-test-workspace-repo-root-and-path-scoped":                  "TestMaduinWorktreeParity/maduin-test-workspace-repo-root-and-path-scoped",
		"maduin-test-workspace-repo-path-config-override":                  "TestMaduinWorktreeParity/maduin-test-workspace-repo-path-config-override",
		"maduin-test-workspace-repo-invalid-input-no-fallback":             "TestMaduinWorktreeParity/maduin-test-workspace-repo-invalid-input-no-fallback",
		"maduin-test-workspace-repo-exists-scoped-by-repo":                 "TestMaduinWorktreeParity/maduin-test-workspace-repo-exists-scoped-by-repo",
		"maduin-test-workspace-repo-harden-writes-target-repo":             "TestMaduinWorktreeParity/maduin-test-workspace-repo-harden-writes-target-repo",
		"maduin-test-workspace-repo-harden-failure-warns-per-key":          "TestMaduinWorktreeParity/maduin-test-workspace-repo-harden-failure-warns-per-key",
		"maduin-test-workspace-repo-harden-real-git-idempotent":            "TestMaduinWorktreeParity/maduin-test-workspace-repo-harden-real-git-idempotent",
		"maduin-test-workspace-repo-life-isolates-seat-worktrees":          "TestMaduinWorktreeParity/maduin-test-workspace-repo-life-isolates-seat-worktrees",
		"maduin-test-workspace-repo-life-stale-and-refusal":                "TestMaduinWorktreeParity/maduin-test-workspace-repo-life-stale-and-refusal",
		"maduin-test-workspace-repo-life-sync-uses-repo-branch-and-seam":   "TestMaduinWorktreeParity/maduin-test-workspace-repo-life-sync-uses-repo-branch-and-seam",
		"maduin-test-workspace-repo-life-invalid-refuses-once-without-git": "TestMaduinWorktreeParity/maduin-test-workspace-repo-life-invalid-refuses-once-without-git",

		"maduin-test-config-loads":                                            "TestMaduinConfigParity/maduin-test-config-loads",
		"maduin-test-config-concierge-seats":                                  "TestMaduinConfigParity/maduin-test-config-concierge-seats",
		"maduin-test-config-designer-seats":                                   "TestMaduinConfigParity/maduin-test-config-designer-seats",
		"maduin-test-config-fleet-seats":                                      "TestMaduinConfigParity/maduin-test-config-fleet-seats",
		"maduin-test-config-seats":                                            "TestMaduinConfigParity/maduin-test-config-seats",
		"maduin-test-config-seat-models":                                      "TestMaduinConfigParity/maduin-test-config-seat-models",
		"maduin-test-config-fleet-fallback":                                   "TestMaduinConfigParity/maduin-test-config-fleet-fallback",
		"maduin-test-config-fleet-tier-model-keys":                            "TestMaduinConfigParity/maduin-test-config-fleet-tier-model-keys",
		"maduin-test-config-difficulty-model-kiro-tiers":                      "TestMaduinConfigParity/maduin-test-config-difficulty-model-kiro-tiers",
		"maduin-test-config-difficulty-model-seat-override":                   "TestMaduinConfigParity/maduin-test-config-difficulty-model-seat-override",
		"maduin-test-config-difficulty-model-opencode-ignores-tier":           "TestMaduinConfigParity/maduin-test-config-difficulty-model-opencode-ignores-tier",
		"maduin-test-config-difficulty-model-unknown-tier-defaults":           "TestMaduinConfigParity/maduin-test-config-difficulty-model-unknown-tier-defaults",
		"maduin-test-config-difficulty-model-rejects-prefixed-kiro":           "TestMaduinConfigParity/maduin-test-config-difficulty-model-rejects-prefixed-kiro",
		"maduin-test-config-difficulty-effort-kiro-tiers":                     "TestMaduinConfigParity/maduin-test-config-difficulty-effort-kiro-tiers",
		"maduin-test-config-difficulty-effort-opencode-unset":                 "TestMaduinConfigParity/maduin-test-config-difficulty-effort-opencode-unset",
		"maduin-test-config-difficulty-effort-seat-override":                  "TestMaduinConfigParity/maduin-test-config-difficulty-effort-seat-override",
		"maduin-test-config-difficulty-effort-rejects-invalid":                "TestMaduinConfigParity/maduin-test-config-difficulty-effort-rejects-invalid",
		"maduin-test-config-difficulty-effort-nil-tier":                       "TestMaduinConfigParity/maduin-test-config-difficulty-effort-nil-tier",
		"maduin-test-config-fleet-tier-effort-keys":                           "TestMaduinConfigParity/maduin-test-config-fleet-tier-effort-keys",
		"maduin-test-config-tier-schema-rows":                                 "TestMaduinConfigParity/maduin-test-config-tier-schema-rows",
		"maduin-test-config-poll-interval":                                    "TestMaduinConfigParity/maduin-test-config-poll-interval",
		"maduin-test-config-backend-role-defaults":                            "TestMaduinConfigParity/maduin-test-config-backend-role-defaults",
		"maduin-test-config-backend-inherits-role-default":                    "TestMaduinConfigParity/maduin-test-config-backend-inherits-role-default",
		"maduin-test-config-backend-seat-override-wins":                       "TestMaduinConfigParity/maduin-test-config-backend-seat-override-wins",
		"maduin-test-config-backend-seat-mutation-isolated":                   "TestMaduinConfigParity/maduin-test-config-backend-seat-mutation-isolated",
		"maduin-test-config-backend-invalid-input-does-not-mutate":            "TestMaduinConfigParity/maduin-test-config-backend-invalid-input-does-not-mutate",
		"maduin-test-config-crew-backend-unset-preserves-existing-precedence": "TestMaduinConfigParity/maduin-test-config-crew-backend-unset-preserves-existing-precedence",
		"maduin-test-config-crew-backend-overrides-every-seat-and-model":      "TestMaduinConfigParity/maduin-test-config-crew-backend-overrides-every-seat-and-model",
		"maduin-test-config-crew-backend-invalid-input-does-not-mutate":       "TestMaduinConfigParity/maduin-test-config-crew-backend-invalid-input-does-not-mutate",
		"maduin-test-config-crew-backend-schema-and-panel-visible":            "TestMaduinConfigParity/maduin-test-config-crew-backend-schema-and-panel-visible",
		"maduin-test-config-backend-save-refuses-unsafe-rewrite":              "TestMaduinConfigParity/maduin-test-config-backend-save-refuses-unsafe-rewrite",
		"maduin-test-config-role-models-are-backend-specific":                 "TestMaduinConfigParity/maduin-test-config-role-models-are-backend-specific",
		"maduin-test-config-seat-models-preserve-opencode-models":             "TestMaduinConfigParity/maduin-test-config-seat-models-preserve-opencode-models",
		"maduin-test-config-seat-models-resolve-kiro-per-role":                "TestMaduinConfigParity/maduin-test-config-seat-models-resolve-kiro-per-role",
		"maduin-test-config-seat-kiro-model-overrides-role":                   "TestMaduinConfigParity/maduin-test-config-seat-kiro-model-overrides-role",
		"maduin-test-config-seat-kiro-model-requires-explicit-mapping":        "TestMaduinConfigParity/maduin-test-config-seat-kiro-model-requires-explicit-mapping",
		"maduin-test-config-option-schema-coverage":                           "TestMaduinConfigParity/maduin-test-config-option-schema-coverage",
		"maduin-test-config-option-get":                                       "TestMaduinConfigParity/maduin-test-config-option-get",
		"maduin-test-config-option-options":                                   "TestMaduinConfigParity/maduin-test-config-option-options",
		"maduin-test-config-option-set":                                       "TestMaduinConfigParity/maduin-test-config-option-set",
		"maduin-test-config-option-rejects-invalid-type":                      "TestMaduinConfigParity/maduin-test-config-option-rejects-invalid-type",
		"maduin-test-config-option-rejects-unknown-key":                       "TestMaduinConfigParity/maduin-test-config-option-rejects-unknown-key",
		"maduin-test-config-option-rejects-invalid-choice":                    "TestMaduinConfigParity/maduin-test-config-option-rejects-invalid-choice",
		"maduin-test-config-option-adds-missing-key":                          "TestMaduinConfigParity/maduin-test-config-option-adds-missing-key",
		"maduin-test-config-option-save-rejected":                             "TestMaduinConfigParity/maduin-test-config-option-save-rejected",
		"maduin-test-config-workspaces-keys":                                  "TestMaduinConfigParity/maduin-test-config-workspaces-keys",
		"maduin-test-config-repairer-keys":                                    "TestMaduinConfigParity/maduin-test-config-repairer-keys",
		"maduin-test-config-planner-kiro-models-are-opus":                     "TestMaduinConfigParity/maduin-test-config-planner-kiro-models-are-opus",

		"maduin-test-log-appends-lines":                               "TestMaduinLoggingParity/maduin-test-log-appends-lines",
		"maduin-test-log-respects-level":                              "TestMaduinLoggingParity/maduin-test-log-respects-level",
		"maduin-test-log-format-string-without-args-is-verbatim":      "TestMaduinLoggingParity/maduin-test-log-format-string-without-args-is-verbatim",
		"maduin-test-log-never-signals":                               "TestMaduinLoggingParity/maduin-test-log-never-signals",
		"maduin-test-log-trims-to-max-lines":                          "TestMaduinLoggingParity/maduin-test-log-trims-to-max-lines",
		"maduin-test-log-event-string":                                "TestMaduinLoggingParity/maduin-test-log-event-string",
		"maduin-test-log-level-threshold":                             "TestMaduinLoggingParity/maduin-test-log-level-threshold",
		"maduin-test-log-mode-bindings-are-evil-aware":                "TestMaduinLoggingParity/maduin-test-log-mode-bindings-are-evil-aware",
		"maduin-test-log-repo-name-is-safe-and-normalized":            "TestMaduinLoggingParity/maduin-test-log-repo-name-is-safe-and-normalized",
		"maduin-test-log-repo-land-close-recover-and-malformed-entry": "TestMaduinLoggingParity/maduin-test-log-repo-land-close-recover-and-malformed-entry",
		"maduin-test-log-repo-review-events-and-fleet-hold":           "TestMaduinLoggingParity/maduin-test-log-repo-review-events-and-fleet-hold",
		"maduin-test-log-repo-tick-details-debug-only":                "TestMaduinLoggingParity/maduin-test-log-repo-tick-details-debug-only",
	}

	owners := map[string]string{
		"state": "TestMaduinStateParity", "bd": "TestMaduinBDParity",
		"repo": "TestMaduinRepoParity", "workspace": "TestMaduinWorktreeParity",
		"multirepo": "TestMaduinRepoParity", "backcompat": "TestMaduinBDParity",
	}
	catalog, err := LoadCatalog()
	if err != nil {
		panic(fmt.Sprintf("load substrate parity catalog: %v", err))
	}
	for _, invariant := range catalog.Entries {
		if owner, ok := owners[invariant.Domain]; ok {
			counterparts[invariant.Name] = owner + "/" + invariant.Name
		}
	}
	return counterparts
}

package parity

import "fmt"

// SubstrateCounterparts maps package-local parity subtests. Config and logging
// use explicit rows so Bindings can enforce one assertion per invariant.
func SubstrateCounterparts() map[string]string {
	counterparts := map[string]string{
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

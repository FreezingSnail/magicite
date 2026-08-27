// Package version reports build provenance.
package version

import "runtime/debug"

// Version is the semantic version for this build.
const Version = "0.1.0"

// Info returns the semantic version and the source revision embedded by Go.
func Info() string {
	revision := "unknown"

	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			if setting.Key == "vcs.revision" && setting.Value != "" {
				revision = setting.Value
				break
			}
		}
	}

	return Version + "+" + revision
}

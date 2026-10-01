package version

import "fmt"

// Set by ldflags at build time (see the Justfile).
var (
	Version    = "dev"
	CommitHash = "unknown"
	BuildDate  = "unknown"
)

// Info returns the build information as a single line.
func Info() string {
	return fmt.Sprintf("Version: %s (commit: %s, built: %s)", Version, CommitHash, BuildDate)
}

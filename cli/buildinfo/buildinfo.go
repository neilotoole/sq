// Package buildinfo hosts build info variables populated via ldflags.
package buildinfo

// DefaultVersion is the default value for Version if not
// set via ldflags.
const DefaultVersion = "v0.0.0-dev"

var (
	// Version is the build version. If not set at build time via
	// ldflags, Version takes the value of DefaultVersion.
	Version = DefaultVersion

	// Commit is the commit hash.
	Commit string

	// Timestamp is the timestamp of when the cli was built.
	Timestamp string
)

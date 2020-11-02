// Package buildinfo hosts build info variables populated via ldflags.
package buildinfo

// defaultVersion is the default value for Version if not
// set via ldflags.
const defaultVersion = "v0.0.0-dev"

var (
	// Version is the build version. If not set at build time via
	// ldflags, Version takes the value of defaultVersion.
	Version = defaultVersion

	// Commit is the commit hash.
	Commit string

	// Timestamp is the timestamp of when the cli was built.
	Timestamp string
)

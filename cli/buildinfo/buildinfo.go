// Package buildinfo hosts build info variables populated via ldflags.
package buildinfo

var (
	// Version is the build version.
	Version string

	// Commit is the commit hash.
	Commit string

	// Timestamp is the timestamp of when the cli was built.
	Timestamp string
)

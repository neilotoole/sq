// Package buildinfo hosts build info variables populated via ldflags.
package buildinfo

import (
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"golang.org/x/exp/slog"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"golang.org/x/mod/semver"
)

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

// BuildInfo encapsulates Version, Commit and Timestamp.
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// String returns a string representation of BuildInfo.
func (bi BuildInfo) String() string {
	s := bi.Version
	if bi.Commit != "" {
		s += " " + bi.Commit
	}
	if bi.Timestamp != "" {
		s += " " + bi.Timestamp
	}
	return s
}

// LogValue implements slog.LogValuer.
func (bi BuildInfo) LogValue() slog.Value {
	gv := slog.GroupValue(
		slog.String(lga.Version, bi.Version),
		slog.String(lga.Commit, bi.Commit),
		slog.String(lga.Timestamp, bi.Timestamp))

	return gv
}

// Info returns BuildInfo.
func Info() BuildInfo {
	return BuildInfo{
		Version:   Version,
		Commit:    Commit,
		Timestamp: Timestamp,
	}
}

func init() { //nolint:gochecknoinits
	if strings.HasSuffix(Version, "~dev") {
		Version = strings.Replace(Version, "~dev", "-dev", 1)
	}

	if Version != "" && !semver.IsValid(Version) {
		// We want to panic here because it is a pipeline/build failure
		// to have an invalid non-empty Version.
		panic(fmt.Sprintf("Invalid BuildInfo.Version value: %q", Version))
	}

	if Timestamp != "" {
		// Make sure Timestamp is normalized
		t := stringz.TimestampToRFC3339(Timestamp)
		if t != "" {
			Timestamp = t
		}
	}
}

// IsDefaultVersion returns true if Version is empty or DefaultVersion.
func IsDefaultVersion() bool {
	return Version == "" || Version == DefaultVersion
}

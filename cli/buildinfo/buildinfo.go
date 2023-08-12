// Package buildinfo hosts build info variables populated via ldflags.
//
// For testing, you can override the build version
// using envar SQ_BUILD_VERSION (panics if not a valid semver).
package buildinfo

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/neilotoole/sq/libsq/core/timez"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

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
	Version   string `json:"version" yaml:"version"`
	Commit    string `json:"commit,omitempty" yaml:"commit,omitempty"`
	Timestamp string `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
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

// Get returns BuildInfo.
func Get() BuildInfo {
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
		panic(fmt.Sprintf("Invalid BuildInfo.Version value: %s", Version))
	}

	if Timestamp != "" {
		// Make sure Timestamp is normalized
		t := timez.TimestampToRFC3339(Timestamp)
		if t != "" {
			Timestamp = t
		}
	}

	if v, ok := os.LookupEnv(EnvOverrideVersion); ok {
		if !semver.IsValid(v) {
			panic(fmt.Sprintf("Invalid semver value from %s: %s", EnvOverrideVersion, v))
		}

		Version = v
	}
}

// EnvOverrideVersion is used for testing build version, e.g. for
// config upgrades.
const EnvOverrideVersion = `SQ_BUILD_VERSION`

// IsDefaultVersion returns true if Version is empty or DefaultVersion.
func IsDefaultVersion() bool {
	return Version == "" || Version == DefaultVersion
}

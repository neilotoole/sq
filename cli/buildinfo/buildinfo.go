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
	"time"

	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/timez"
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

// Info encapsulates Version, Commit and Timestamp.
type Info struct { //nolint:govet // field alignment
	Version   string    `json:"version" yaml:"version"`
	Commit    string    `json:"commit,omitempty" yaml:"commit,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
}

// String returns a string representation of Info.
func (bi Info) String() string {
	s := bi.Version
	if bi.Commit != "" {
		s += " " + bi.Commit
	}
	if !bi.Timestamp.IsZero() {
		s += " " + bi.Timestamp.Format(timez.RFC3339Z)
	}
	return s
}

// UserAgent returns a string suitable for use in an HTTP User-Agent header.
func (bi Info) UserAgent() string {
	if bi.Version == "" {
		return "sq/0.0.0-dev"
	}

	ua := "sq/" + strings.TrimPrefix(bi.Version, "v")
	return ua
}

// ShortCommit returns the short commit hash.
func (bi Info) ShortCommit() string {
	switch {
	case bi.Commit == "":
		return ""
	case len(bi.Commit) > 7:
		return bi.Commit[:7]
	default:
		return bi.Commit
	}
}

// LogValue implements slog.LogValuer.
func (bi Info) LogValue() slog.Value {
	gv := slog.GroupValue(
		slog.String(lga.Version, bi.Version),
		slog.String(lga.Commit, bi.Commit),
		slog.Time(lga.Timestamp, bi.Timestamp))

	return gv
}

// Get returns Info. If buildinfo.Timestamp cannot be parsed,
// the returned Info.Timestamp will be the zero value.
func Get() Info {
	var t time.Time
	if Timestamp != "" {
		got, err := timez.ParseTimestampUTC(Timestamp)
		if err == nil {
			t = got
		}
	}

	return Info{
		Version:   Version,
		Commit:    Commit,
		Timestamp: t,
	}
}

func init() { //nolint:gochecknoinits
	if strings.HasSuffix(Version, "~dev") {
		Version = strings.Replace(Version, "~dev", "-dev", 1)
	}

	if Version != "" && !semver.IsValid(Version) {
		// We want to panic here because it is a pipeline/build failure
		// to have an invalid non-empty Version.
		panic("Invalid Info.Version value: " + Version)
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

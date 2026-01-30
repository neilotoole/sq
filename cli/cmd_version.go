package cli

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/hostinfo"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version info",
		Long: `Show version info.
The output notes if a new version of sq is available.

Use --verbose, or --json or --yaml for more detail.

Before upgrading, check the changelog: https://sq.io/changelog`,
		RunE: execVersion,
		Example: `  # Show version (note that an update is available)
  $ sq version
  sq v0.38.0    Update available: v0.39.0

  # Verbose output
  $ sq version -v
  sq v0.38.0
  Version:         v0.38.0
  Commit:          #4e176716
  Timestamp:       2023-06-21T11:39:39Z
  Latest version:  v0.39.0
  Host:            darwin arm64 | Darwin 22.5.0 | macOS 13.4
  [...]

  # JSON output
  $ sq version -j
  {
    "version": "v0.38.0",
    "commit": "4e176716",
    "timestamp": "2023-06-19T22:08:54Z",
    "latest_version": "v0.39.0",
    "host": {
      "platform": "darwin",
      "arch": "arm64",
      "kernel": "Darwin",
      "kernel_version": "22.5.0",
      "variant": "macOS",
      "variant_version": "13.4"
    }
  }

  # Extract just the semver string
  $ sq version -j | jq -r .version
  v0.38.0`,
	}

	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	addOptionFlag(cmd.Flags(), OptCompact)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)

	return cmd
}

func execVersion(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())

	// We'd like to display that there's an update available, but
	// we don't want to wait around long for that.
	// So, we swallow (but log) any error from the goroutine.
	//
	// See also: https://github.com/neilotoole/sq/issues/531
	//
	// At some point, we could make the should-check-for-update behavior
	// configurable. For now, we'll make this timeout pretty short so that
	// "sq version" returns quickly even with low/slow/no-connectivity.
	ctx, cancelFn := context.WithTimeout(cmd.Context(), time.Millisecond*500)
	defer cancelFn()

	// Buffered so the goroutine can complete its send even if ctx times out
	// and no receiver is waiting, avoiding a goroutine leak.
	resultCh := make(chan string, 1)
	go func() {
		var err error
		v, err := fetchBrewVersion(ctx)
		lg.WarnIfError(lg.From(cmd), "Get brew version", err)

		// OK if v is empty
		resultCh <- v
	}()

	var latestVersion string
	select {
	case <-ctx.Done():
	case latestVersion = <-resultCh:
		if latestVersion != "" && !strings.HasPrefix(latestVersion, "v") {
			latestVersion = "v" + latestVersion
			if !semver.IsValid(latestVersion) {
				return errz.Errorf("invalid semver from brew formula: {%s}", latestVersion)
			}
		}
	}

	return ru.Writers.Version.Version(buildinfo.Get(), latestVersion, hostinfo.Get())
}

// fetchBrewVersion fetches the latest available sq version via
// the published homebrew-core formula. Previously this function fetched from
// the personal tap (neilotoole/homebrew-sq), but since sq was accepted into
// homebrew-core, we now check there as it's the canonical source for most users
// who install via "brew install sq".
func fetchBrewVersion(ctx context.Context) (string, error) {
	// Old URL (personal tap): https://raw.githubusercontent.com/neilotoole/homebrew-sq/master/sq.rb
	const u = `https://raw.githubusercontent.com/Homebrew/homebrew-core/HEAD/Formula/s/sq.rb`

	lg.FromContext(ctx).Debug("Fetching brew formula for version check", lga.URL, u)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return "", errz.Err(err)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose
	if err != nil {
		return "", errz.Wrap(err, "failed to check sq brew formula")
	}
	defer ioz.Close(ctx, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", errz.Errorf("failed to check sq brew formula: %d %s",
			resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errz.Wrap(err, "failed to read sq brew formula body")
	}

	return getVersionFromBrewFormula(body)
}

// getVersionFromBrewFormula returns the brew version from f, which is a brew
// ruby formula. The version is returned without a "v" prefix, e.g. "0.1.2",
// not "v0.1.2".
//
// It supports two formula formats:
//   - Explicit version: `version "0.48.11"` (used by personal tap/GoReleaser)
//   - URL-based version: `url ".../tags/v0.48.11.tar.gz"` (used by homebrew-core)
//
// While scanning the formula (up to the "bottle" section), explicit version
// takes precedence over URL-based version. A version declared after the
// "bottle" section is not considered.
func getVersionFromBrewFormula(f []byte) (string, error) {
	var (
		val        string
		urlVersion string // URL-based version (fallback)
	)

	sc := bufio.NewScanner(bytes.NewReader(f))
	for sc.Scan() {
		val = strings.TrimSpace(sc.Text())

		// Check for explicit version line: version "0.48.11"
		// Explicit version always takes precedence, so return immediately.
		if strings.HasPrefix(val, `version "`) {
			val = val[9:]
			val = strings.TrimSuffix(val, `"`)
			if !semver.IsValid("v" + val) { // semver pkg requires "v" prefix
				return "", errz.Errorf("invalid brew formula: invalid semver {%s}", val)
			}
			return val, nil
		}

		// Check for URL-based version (homebrew-core format):
		// url "https://github.com/neilotoole/sq/archive/refs/tags/v0.48.11.tar.gz"
		// Don't return immediately; continue scanning for explicit version.
		if strings.HasPrefix(val, `url "`) && strings.Contains(val, "/tags/v") {
			// Extract version from URL pattern /tags/vX.Y.Z.tar.gz
			idx := strings.Index(val, "/tags/v")
			if idx != -1 {
				// Start after "/tags/v"
				verStart := idx + len("/tags/v")
				remainder := val[verStart:]
				// Find the end at .tar.gz or .zip
				if endIdx := strings.Index(remainder, ".tar.gz"); endIdx != -1 {
					val = remainder[:endIdx]
				} else if endIdx := strings.Index(remainder, ".zip"); endIdx != -1 {
					val = remainder[:endIdx]
				} else {
					// Unrecognized archive extension; skip and keep scanning.
					continue
				}
				if !semver.IsValid("v" + val) {
					return "", errz.Errorf("invalid brew formula: invalid semver in URL {%s}", val)
				}
				urlVersion = val
			}
		}

		if strings.HasPrefix(val, "bottle") {
			// Reached the bottle section; stop scanning.
			break
		}
	}

	if sc.Err() != nil {
		return "", errz.Wrap(sc.Err(), "invalid brew formula")
	}

	if urlVersion != "" {
		return urlVersion, nil
	}

	return "", errz.New("invalid brew formula")
}

package cli

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/neilotoole/sq/cli/hostinfo"

	"github.com/neilotoole/sq/libsq/core/ioz"

	"github.com/neilotoole/sq/cli/run"

	"github.com/neilotoole/sq/cli/flag"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/cli/buildinfo"
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

	addTextFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)

	return cmd
}

func execVersion(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())

	// We'd like to display that there's an update available, but
	// we don't want to wait around long for that.
	// So, we swallow (but log) any error from the goroutine.
	ctx, cancelFn := context.WithTimeout(cmd.Context(), time.Second*2)
	defer cancelFn()

	resultCh := make(chan string)
	go func() {
		var err error
		v, err := fetchBrewVersion(ctx)
		if err != nil {
			lg.Error(logFrom(cmd), "Fetch brew version", err)
		}

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
				return errz.Errorf("invalid semver from brew repo: {%s}", latestVersion)
			}
		}
	}

	return ru.Writers.Version.Version(buildinfo.Get(), latestVersion, hostinfo.Get())
}

// fetchBrewVersion fetches the latest available sq version via
// the published brew formula.
func fetchBrewVersion(ctx context.Context) (string, error) {
	const u = `https://raw.githubusercontent.com/neilotoole/homebrew-sq/master/sq.rb`

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return "", errz.Err(err)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose
	if err != nil {
		return "", errz.Wrap(err, "failed to check sq brew repo")
	}
	defer ioz.Close(ctx, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", errz.Errorf("failed to check sq brew repo: %d %s",
			resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errz.Wrap(err, "failed to read sq brew repo body")
	}

	return getVersionFromBrewFormula(body)
}

// getVersionFromBrewFormula returns the first brew version
// from f, which is a brew ruby formula. The version is returned
// without a "v" prefix, e.g. "0.1.2", not "v0.1.2".
func getVersionFromBrewFormula(f []byte) (string, error) {
	var (
		line string
		val  string
		err  error
	)

	sc := bufio.NewScanner(bytes.NewReader(f))
	for sc.Scan() {
		line = sc.Text()
		if err = sc.Err(); err != nil {
			return "", errz.Err(err)
		}

		val = strings.TrimSpace(line)
		if strings.HasPrefix(val, `version "`) {
			// found it
			val = val[9:]
			val = strings.TrimSuffix(val, `"`)
			if !semver.IsValid("v" + val) { // semver pkg requires "v" prefix
				return "", errz.Errorf("invalid brew formula: invalid semver")
			}
			return val, nil
		}

		if strings.HasPrefix(line, "bottle") {
			// Gone too far
			return "", errz.New("unable to parse brew formula")
		}
	}

	if sc.Err() != nil {
		return "", errz.Wrap(err, "invalid brew formula")
	}

	return "", errz.New("invalid brew formula")
}

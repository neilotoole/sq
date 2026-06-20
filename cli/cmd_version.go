package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/hostinfo"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/cli/updatecheck"
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

	latestVersion, err := updatecheck.FetchLatestWithWait(cmd.Context(), updatecheck.CacheDirForRun(ru), 500*time.Millisecond)
	if err != nil {
		return err
	}

	return ru.Writers.Version.Version(buildinfo.Get(), latestVersion, hostinfo.Get())
}

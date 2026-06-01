package cli

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/source"
)

func newConfigKeyringLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Args:    cobra.NoArgs,
		Short:   "List keyring paths referenced by sources",
		Long: `Walk every source's Location and print one row per ${keyring:<path>}
placeholder it references, paired with the source's handle and driver.
${env:...} and ${file:...} refs are not listed here — those backends are
external; sq reads them at connect time and has nothing to manage.

Output columns:
  PATH      The keyring entry path, e.g. "j2k7m3pxtz".
  HANDLE    The @handle that references the entry.
  DRIVER    The source's driver type (postgres, mysql, sqlite, ...).

When the same path appears in multiple sources, each source gets its own
row — clustered together because rows are sorted by path. A repeated
path across rows means the underlying keyring entry is shared.

The listing is derived from the YAML config; no persistent index is
maintained. Keyring entries that no source references (orphans) are
not surfaced — that requires keyring enumeration, deferred to a
future release.`,
		RunE: execConfigKeyringLs,
	}
	addKeyringFormatFlags(cmd)
	return cmd
}

func execConfigKeyringLs(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())
	refs := collectKeyringRefs(ru.Config.Collection.Sources())
	return ru.Writers.Keyring.List(refs)
}

// collectKeyringRefs extracts (path, handle, driver) rows from srcs,
// sorted by path then handle so shared paths cluster on adjacent rows.
// Only ${keyring:...} refs are included; env and file refs are ignored.
// No deduplication: a shared path yields one row per referencing source,
// which is how sharing becomes visible in the output.
//
// Malformed placeholders are silently skipped here. Surfacing them
// would require a return-error variant; in practice the same sources
// also fail to open via "sq ping", which is the better error venue.
func collectKeyringRefs(srcs []*source.Source) []output.KeyringRef {
	var rows []output.KeyringRef
	for _, src := range srcs {
		refs, err := secret.ExtractRefs(src.Location)
		if err != nil {
			continue
		}
		for _, ref := range refs {
			if ref.Scheme != "keyring" {
				continue
			}
			rows = append(rows, output.KeyringRef{
				Path:   ref.Path,
				Handle: src.Handle,
				Driver: string(src.Type),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Path != rows[j].Path {
			return rows[i].Path < rows[j].Path
		}
		return rows[i].Handle < rows[j].Handle
	})
	return rows
}

// addKeyringFormatFlags registers the output-format flags supported by
// the keyring subcommands: --text/--json/-j. The default format is
// text/table; --json selects the JSON impl.
func addKeyringFormatFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP(flag.Text, flag.TextShort, false, flag.TextUsage)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
}

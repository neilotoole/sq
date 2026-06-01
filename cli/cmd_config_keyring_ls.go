package cli

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

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
	return cmd
}

type lsRow struct {
	path   string // keyring path (the body after "keyring:")
	handle string
	driver string
}

func execConfigKeyringLs(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())

	rows, err := buildLsRows(ru.Config.Collection.Sources())
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	printLsRows(ru.Out, rows)
	return nil
}

// buildLsRows extracts (path, handle, driver) rows from sources, sorted
// by path then handle so shared paths cluster on adjacent lines. Only
// ${keyring:...} refs are included; env and file refs are ignored.
// No deduplication: a shared path produces one row per referencing
// source, which is how sharing becomes visible in the output.
func buildLsRows(srcs []*source.Source) ([]lsRow, error) {
	var rows []lsRow
	for _, src := range srcs {
		refs, err := secret.ExtractRefs(src.Location)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			if ref.Scheme != "keyring" {
				continue
			}
			rows = append(rows, lsRow{
				path:   ref.Path,
				handle: src.Handle,
				driver: string(src.Type),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].path != rows[j].path {
			return rows[i].path < rows[j].path
		}
		return rows[i].handle < rows[j].handle
	})
	return rows, nil
}

// printLsRows writes rows to out as three space-aligned columns.
func printLsRows(out io.Writer, rows []lsRow) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.path, r.handle, r.driver)
	}
	_ = tw.Flush()
}

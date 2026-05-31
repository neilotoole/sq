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

func newConfigSecretsLsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Args:    cobra.NoArgs,
		Short:   "List secret refs known to sq, cross-referenced by source",
		Long: `Walk every source's Location and print one row per ${scheme:path}
placeholder it contains, paired with the source's handle and driver.

Output columns:
  REF       The placeholder body, e.g. "keyring:j2k7m3pxtz" or "env:DB_PW".
  HANDLE    The @handle that references the placeholder.
  DRIVER    The source's driver type (postgres, mysql, sqlite, ...).

When the same ref appears in multiple sources, each source gets its own
row — clustered together because rows are sorted by ref. A repeated ref
across rows means the underlying secret is shared.

The listing is derived from the YAML config; no persistent index is
maintained. Keyring entries that no source references (orphans) are
not surfaced — that requires keyring enumeration, deferred to a
future release.`,
		RunE: execConfigSecretsLs,
	}
	return cmd
}

type lsRow struct {
	ref    string // "<scheme>:<path>"
	handle string
	driver string
}

func execConfigSecretsLs(cmd *cobra.Command, _ []string) error {
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

// buildLsRows extracts (ref, handle, driver) rows from sources, sorted
// by ref then handle so shared refs cluster on adjacent lines. No
// deduplication: a shared ref produces one row per referencing source,
// which is how sharing becomes visible in the output.
func buildLsRows(srcs []*source.Source) ([]lsRow, error) {
	var rows []lsRow
	for _, src := range srcs {
		refs, err := secret.ExtractRefs(src.Location)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			rows = append(rows, lsRow{
				ref:    ref.Scheme + ":" + ref.Path,
				handle: src.Handle,
				driver: string(src.Type),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ref != rows[j].ref {
			return rows[i].ref < rows[j].ref
		}
		return rows[i].handle < rows[j].handle
	})
	return rows, nil
}

// printLsRows writes rows to out as three space-aligned columns.
// text/tabwriter handles width variance — short Crockford IDs and long
// op:// refs align cleanly without manual width math.
func printLsRows(out io.Writer, rows []lsRow) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.ref, r.handle, r.driver)
	}
	_ = tw.Flush()
}

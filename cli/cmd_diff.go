package cli

import (
	"github.com/neilotoole/sq/cli/diff"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/samber/lo"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"
)

var OptDiffNumLines = options.NewInt(
	"diff.lines",
	"unified",
	'U',
	3,
	"Generate diffs with <n> lines of context",
	`Generate diffs with <n> lines of context, where n >= 0.`,
)

var allDiffElementsFlags = []string{
	flag.DiffAll,
	flag.DiffSummary,
	flag.DiffTable,
	flag.DiffDBProps,
	flag.DiffRowCount,
	flag.DiffData,
}

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff @HANDLE1[.TABLE] @HANDLE2[.TABLE]",
		Short: "Compare sources, or tables",
		Long: `BETA: Compare sources, or tables.

When comparing sources, by default the source summary, table structure,
and table row counts are compared.

When comparing tables, by default the table structure and table row counts
are compared.

Use flags to specify the elements you want to compare. See the examples.
Note that --summary and --dbprops only apply to source diff, not table diff.

Flag --unified (-U) controls the number of lines to show surrounding a diff.
The default can be changed via "sq config set diff.lines".`,
		Args: cobra.ExactArgs(2),
		ValidArgsFunction: (&handleTableCompleter{
			handleRequired: true,
			max:            2,
		}).complete,
		RunE: execDiff,
		Example: `  # Diff sources (compare default elements).
  $ sq diff @prod/sakila @staging/sakila

  # As above, but show 7 lines surrounding each diff.
  $ sq diff @prod/sakila @staging/sakila -U7

  # Diff sources, but only compare source summary.
  $ sq diff @prod/sakila @staging/sakila --summary

  # Compare source summary, and DB properties.
  $ sq diff @prod/sakila @staging/sakila -sp

  # Compare schema table structure, and row counts.
  $ sq diff @prod/sakila @staging/sakila --Tc

  # Compare everything, including table data. Caution: this can be slow.
  $ sq diff @prod/sakila @staging/sakila --all

  # Compare actor table in prod vs staging
  $ sq diff @prod/sakila.actor @staging/sakila.actor

  # Compare data in the actor tables. Caution: this can be slow.
  $ sq diff @prod/sakila.actor @staging/sakila.actor --data`,
	}

	addOptionFlag(cmd.Flags(), OptDiffNumLines)

	cmd.Flags().BoolP(flag.DiffSummary, flag.DiffSummaryShort, false, flag.DiffSummaryUsage)
	cmd.Flags().BoolP(flag.DiffDBProps, flag.DiffDBPropsShort, false, flag.DiffDBPropsUsage)
	cmd.Flags().BoolP(flag.DiffTable, flag.DiffTableShort, false, flag.DiffTableUsage)
	cmd.Flags().BoolP(flag.DiffRowCount, flag.DiffRowCountShort, false, flag.DiffRowCountUsage)
	cmd.Flags().BoolP(flag.DiffData, flag.DiffDataShort, false, flag.DiffDataUsage)
	cmd.Flags().BoolP(flag.DiffAll, flag.DiffAllShort, false, flag.DiffAllUsage)

	// If flag.DiffAll is provided, no other diff elements flag can be provided.
	nonAllFlags := lo.Drop(allDiffElementsFlags, 0)
	for i := range nonAllFlags {
		cmd.MarkFlagsMutuallyExclusive(flag.DiffAll, nonAllFlags[i])
	}

	return cmd
}

// execDiff compares sources or tables.
func execDiff(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)

	handle1, table1, err := source.ParseTableHandle(args[0])
	if err != nil {
		return errz.Wrapf(err, "invalid input (1st arg): %s", args[0])
	}

	handle2, table2, err := source.ParseTableHandle(args[1])
	if err != nil {
		return errz.Wrapf(err, "invalid input (2nd arg): %s", args[1])
	}

	o, err := getOptionsFromCmd(cmd)
	if err != nil {
		return err
	}

	numLines := OptDiffNumLines.Get(o)
	if numLines < 0 {
		return errz.Errorf("number of lines to show must be >= 0")
	}

	switch {
	case table1 == "" && table2 == "":
		elems := getDiffSourceElements(cmd)
		return diff.ExecSourceDiff(ctx, ru, numLines, elems, handle1, handle2)
	case table1 == "" || table2 == "":
		return errz.Errorf("invalid args: both must be either @HANDLE or @HANDLE.TABLE")
	default:
		elems := getDiffTableElements(cmd)
		return diff.ExecTableDiff(ctx, ru, numLines, elems, handle1, table1, handle2, table2)
	}
}

func getDiffSourceElements(cmd *cobra.Command) *diff.Elements {
	if !isAnyDiffElementsFlagChanged(cmd) {
		// Default
		return &diff.Elements{
			Summary:      true,
			DBProperties: false,
			Table:        true,
			RowCount:     true,
			Data:         false,
		}
	}

	if cmdFlagChanged(cmd, flag.DiffAll) {
		return &diff.Elements{
			Summary:      true,
			DBProperties: true,
			Table:        true,
			RowCount:     true,
			Data:         true,
		}
	}

	return &diff.Elements{
		Summary:      cmdFlagIsSetTrue(cmd, flag.DiffSummary),
		DBProperties: cmdFlagIsSetTrue(cmd, flag.DiffDBProps),
		Table:        cmdFlagIsSetTrue(cmd, flag.DiffTable),
		RowCount:     cmdFlagIsSetTrue(cmd, flag.DiffRowCount),
		Data:         cmdFlagIsSetTrue(cmd, flag.DiffData),
	}
}

func getDiffTableElements(cmd *cobra.Command) *diff.Elements {
	if !isAnyDiffElementsFlagChanged(cmd) {
		// Default
		return &diff.Elements{
			Table:    true,
			RowCount: true,
		}
	}

	if cmdFlagChanged(cmd, flag.DiffAll) {
		return &diff.Elements{
			Table:    true,
			RowCount: true,
			Data:     true,
		}
	}

	return &diff.Elements{
		Table:    cmdFlagIsSetTrue(cmd, flag.DiffTable),
		RowCount: cmdFlagIsSetTrue(cmd, flag.DiffRowCount),
		Data:     cmdFlagIsSetTrue(cmd, flag.DiffData),
	}
}

func isAnyDiffElementsFlagChanged(cmd *cobra.Command) bool {
	for _, name := range allDiffElementsFlags {
		if cmdFlagChanged(cmd, name) {
			return true
		}
	}
	return false
}

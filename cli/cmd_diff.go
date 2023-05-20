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

var allDiffOptionFlags = []string{
	flag.DiffAll,
	flag.DiffSummary,
	flag.DiffTables,
	flag.DiffDBProps,
	flag.DiffRowCount,
	flag.DiffData,
}

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff @HANDLE1[.TABLE] @HANDLE2[.TABLE]",
		Short: "Compare sources, or tables",
		Long: `BETA: Compare sources, or tables.

When comparing sources, use flag --summary to perform only a high-level
diff. Note that this may miss column-level differences.

Flag --unified (-U) controls the number of lines to show surrounding a diff. The
default is 3, but this can be changed via "sq config set diff.lines".`,
		Args: cobra.ExactArgs(2),
		ValidArgsFunction: (&handleTableCompleter{
			handleRequired: true,
			max:            2,
		}).complete,
		RunE: execDiff,
		// FIXME: Rewrite this example text with new usage model.
		Example: `  # Diff sources
  $ sq diff @prod/sakila @staging/sakila

  # As above, but show 7 lines surrounding each diff
  $ sq diff @prod/sakila @staging/sakila -U7

  # Diff sources, but don't diff individual tables (summary diff only)
  $ sq diff --tables=false @prod/sakila @staging/sakila

  # Compare "actor" table in prod vs staging
  $ sq diff @prod/sakila.actor @staging/sakila.actor`,
	}

	addOptionFlag(cmd.Flags(), OptDiffNumLines)

	cmd.Flags().BoolP(flag.DiffSummary, flag.DiffSummaryShort, false, flag.DiffSummaryUsage)
	cmd.Flags().BoolP(flag.DiffDBProps, flag.DiffDBPropsShort, false, flag.DiffDBPropsUsage)
	cmd.Flags().BoolP(flag.DiffTables, flag.DiffTablesShort, false, flag.DiffTablesUsage)
	cmd.Flags().BoolP(flag.DiffRowCount, flag.DiffRowCountShort, false, flag.DiffRowCountUsage)
	cmd.Flags().BoolP(flag.DiffData, flag.DiffDataShort, false, flag.DiffDataUsage)
	cmd.Flags().BoolP(flag.DiffAll, flag.DiffAllShort, false, flag.DiffAllUsage)

	// If flag.DiffAll is provided, no other flag can be provided.
	nonAllFlags := lo.Drop(allDiffOptionFlags, 0)
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
		diffOpts := getDiffSourceOptions(cmd)
		return diff.ExecSourceDiff(ctx, ru, numLines, diffOpts, handle1, handle2)
	case table1 == "" || table2 == "":
		return errz.Errorf("invalid args: both must be either @HANDLE or @HANDLE.TABLE")
	default:
		return diff.ExecTableDiff(ctx, ru, numLines, handle1, table1, handle2, table2)
	}
}

func getDiffSourceOptions(cmd *cobra.Command) *diff.Options {
	if !isAnyDiffSourceOptionFlagChanged(cmd) {
		// Default
		return &diff.Options{
			Summary:      true,
			DBProperties: false,
			Tables:       true,
			RowCount:     true,
			Data:         false,
		}
	}

	if cmdFlagChanged(cmd, flag.DiffAll) {
		return &diff.Options{
			Summary:      true,
			DBProperties: true,
			Tables:       true,
			RowCount:     true,
			Data:         true,
		}
	}

	return &diff.Options{
		Summary:      cmdFlagIsSetTrue(cmd, flag.DiffSummary),
		DBProperties: cmdFlagIsSetTrue(cmd, flag.DiffDBProps),
		Tables:       cmdFlagIsSetTrue(cmd, flag.DiffTables),
		RowCount:     cmdFlagIsSetTrue(cmd, flag.DiffRowCount),
		Data:         cmdFlagIsSetTrue(cmd, flag.DiffData),
	}
}

func isAnyDiffSourceOptionFlagChanged(cmd *cobra.Command) bool {
	for _, name := range allDiffOptionFlags {
		if cmdFlagChanged(cmd, name) {
			return true
		}
	}
	return false
}

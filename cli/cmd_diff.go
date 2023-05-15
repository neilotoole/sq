package cli

import (
	"github.com/neilotoole/sq/cli/diff"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/cli/run"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"
)

var OptDiffNumLines = options.NewInt(
	"diff.num-lines",
	'n',
	3,
	"Number of lines to show surrounding diff",
	`Number of lines to show surrounding diff.`,
)

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "diff @HANDLE1[.TABLE] @HANDLE2[.TABLE]",
		Hidden: true, // hidden during development
		Short:  "Compare sources, or tables",
		Long: `BETA: Compare sources, or tables.

When comparing sources, use flag --summary to perform only a high-level
diff. Note that this may miss column-level differences.

Flag --lines (-n) controls the number of lines to show surrounding a diff. The
default is 3.`,
		Args: cobra.ExactArgs(2),
		ValidArgsFunction: (&handleTableCompleter{
			handleRequired: true,
			max:            2,
		}).complete,
		RunE: execDiff,
		Example: `  # Diff sources
  $ sq diff @prod/sakila @staging/sakila

  # As above, but show 7 lines surrounding each diff
  $ sq @prod/sakila @staging/sakila -n7

  # Diff sources, summary only
  $ sq diff --summary @prod/sakila @staging/sakila

  # Compare "actor" table in prod vs staging
  $ sq diff @prod/sakila.actor @staging/sakila.actor`,
	}

	cmd.Flags().Bool(flag.DiffSummary, false, flag.DiffSummaryUsage)
	cmd.Flags().IntP(flag.DiffNumLines, flag.DiffNumLinesShort, OptDiffNumLines.Default(), flag.DiffNumLinesUsage)

	return cmd
}

// execDiff compares schemas or tables.
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

	showSummary := cmdFlagTrue(cmd, flag.DiffSummary)

	o, err := getOptionsFromCmd(cmd)
	if err != nil {
		return err
	}

	// TODO: Unify num lines flag with option
	numLines := OptDiffNumLines.Get(o)
	if numLines < 0 {
		return errz.Errorf("config: {%s} must be a non-negative integer, but got %d",
			flag.DiffNumLines, numLines)
	}

	if cmdFlagChanged(cmd, flag.DiffNumLines) {
		numLines, err = cmd.Flags().GetInt(flag.DiffNumLines)
		if err != nil {
			return errz.Err(err)
		}

		if numLines < 0 {
			return errz.Errorf("flag --%s must be a non-negative integer, but got %d",
				flag.DiffNumLines, numLines)
		}
	}

	switch {
	case table1 == "" && table2 == "":
		return diff.ExecSourceDiff(ctx, ru, numLines, showSummary, handle1, handle2)
	case table1 == "" || table2 == "":
		return errz.Errorf("invalid args: both must be @HANDLE or @HANDLE.TABLE")
	default:
		return diff.ExecTableDiff(ctx, ru, numLines, handle1, table1, handle2, table2)
	}
}

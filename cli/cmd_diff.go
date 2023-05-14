package cli

import (
	"github.com/neilotoole/sq/cli/diff"

	"github.com/neilotoole/sq/cli/run"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "diff @HANDLE1[.TABLE] @HANDLE2[.TABLE]",
		Hidden: true, // hidden during development
		Short:  "Compare schemas or tables",
		Long:   `BETA: Compare two schemas, or tables.`,
		Args:   cobra.ExactArgs(2),
		ValidArgsFunction: (&handleTableCompleter{
			handleRequired: true,
			max:            2,
		}).complete,
		RunE: execDiff,
		Example: `  # Compare sources
  $ sq diff @prod/sakila @staging/sakila

  # Compare "actor" table in prod vs staging
  $ sq diff @prod/sakila.actor @staging/sakila.actor`,
	}

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

	switch {
	case table1 == "" && table2 == "":
		return diff.ExecSourceDiff(ctx, ru, handle1, handle2)
	case table1 == "" || table2 == "":
		return errz.Errorf("invalid args: both must be @HANDLE or @HANDLE.TABLE")
	default:
		return diff.ExecTableDiff(ctx, ru, handle1, table1, handle2, table2)
	}
}

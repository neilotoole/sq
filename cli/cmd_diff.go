package cli

// FIXME: remove nolint

import (
	"github.com/neilotoole/sq/cli/diff"

	"github.com/neilotoole/sq/cli/run"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "diff @HANDLE1[.TABLE] @HANDLE[.TABLE]",
		Hidden: true, // hidden during development
		Short:  "Compare schemas or tables",
		Long:   `BETA: Compare two schemas, or tables. `,
		Args:   cobra.ExactArgs(2),
		ValidArgsFunction: (&handleTableCompleter{
			handleRequired: true,
			max:            2,
		}).complete,
		RunE: execDiff,
		Example: `  # Compare actor table in prod vs staging
  $ sq diff @prod/sakila.actor @staging/sakila.actor`,
	}

	return cmd
}

// execDiff compares schemas or tables.
func execDiff(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())

	handle1, table1, err := source.ParseTableHandle(args[0])
	if err != nil {
		return errz.Wrapf(err, "invalid input (1st arg): %s", args[0])
	}

	handle2, table2, err := source.ParseTableHandle(args[1])
	if err != nil {
		return errz.Wrapf(err, "invalid input (2nd arg): %s", args[1])
	}

	if table1 == "" || table2 == "" {
		return errz.Errorf("invalid input: TABLE value in @HANDLE.TABLE must not be empty")
	}

	return diff.ExecTableDiff(cmd.Context(), ru, handle1, table1, handle2, table2)
}

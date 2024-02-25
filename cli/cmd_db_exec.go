package cli

import (
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/progress"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/execz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

func newDBExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec [@src] [--f SCRIPT.sql] [-c 'SQL'] [--print]",
		Short: "Execute SQL script or command",
		Long: `Execute SQL script or command using the db-native tool.

If no source is specified, the active source is used.

If --file is specified, the SQL is read from that file; otherwise if --command
is specified, that command string is used; otherwise the SQL commands are
read from stdin.

If --print or --print-long are specified, the SQL is not executed, but instead
the db-native command is printed to stdout. Note that the output will include DB
credentials.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeHandle(1, true),
		RunE:              execDBExec,
		Example: `  # Execute query.sql on @sakila_pg
  $ sq db exec @sakila_pg -f query.sql

  # Same as above, but use stdin
  $ sq db exec @sakila_pg < query.sql

  # Execute a command string against the active source
  $ sq db exec -c 'SELECT 777'
  777

  # Print the db-native command, but don't execute it
  $ sq db exec -f query.sql --print
  psql -d 'postgres://alice:abc123@db.acme.com:5432/sales' -f query.sql

  # Execute against an alternative catalog or schema
  $ sq db exec @sakila_pg --schema inventory.public -f query.sql`,
	}

	cmdMarkPlainStdout(cmd)

	cmd.Flags().StringP(flag.DBExecFile, flag.DBExecFileShort, "", flag.DBExecFileUsage)
	cmd.Flags().StringP(flag.DBExecCmd, flag.DBExecCmdShort, "", flag.DBExecCmdUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.DBExecFile, flag.DBExecCmd)

	cmd.Flags().Bool(flag.DBPrintToolCmd, false, flag.DBPrintToolCmdUsage)
	cmd.Flags().Bool(flag.DBPrintLongToolCmd, false, flag.DBPrintLongToolCmdUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.DBPrintToolCmd, flag.DBPrintLongToolCmd)

	cmd.Flags().String(flag.ActiveSchema, "", flag.ActiveSchemaUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.ActiveSchema,
		activeSchemaCompleter{getActiveSourceViaArgs}.complete))

	return cmd
}

func execDBExec(cmd *cobra.Command, args []string) error {
	var (
		ru  = run.FromContext(cmd.Context())
		src *source.Source
		err error

		// scriptFile is the (optional) path to the SQL file.
		// If empty, cmdString or stdin is used.
		scriptFile string

		// scriptString is the optional SQL command string.
		// If empty, scriptFile or stdin is used.
		cmdString string
	)

	if src, err = getCmdSource(cmd, args); err != nil {
		return err
	}

	errPrefix := "db exec: " + src.Handle
	if cmdFlagChanged(cmd, flag.DBExecFile) {
		if scriptFile = strings.TrimSpace(cmd.Flag(flag.DBExecFile).Value.String()); scriptFile == "" {
			return errz.Errorf("%s: %s is specified, but empty", errPrefix, flag.DBExecFile)
		}
	}

	if cmdFlagChanged(cmd, flag.DBExecCmd) {
		if cmdString = strings.TrimSpace(cmd.Flag(flag.DBExecCmd).Value.String()); cmdString == "" {
			return errz.Errorf("%s: %s is specified, but empty", errPrefix, flag.DBExecCmd)
		}
	}

	if err = applySourceOptions(cmd, src); err != nil {
		return err
	}

	var execCmd *execz.Cmd

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		params := &postgres.ExecToolParams{
			Verbose:    OptVerbose.Get(src.Options),
			ScriptFile: scriptFile,
			CmdString:  cmdString,
			LongFlags:  cmdFlagChanged(cmd, flag.DBPrintLongToolCmd),
		}
		execCmd, err = postgres.ExecCmd(src, params)
	default:
		return errz.Errorf("%s: not supported for %s", errPrefix, src.Type)
	}

	if err != nil {
		return errz.Wrap(err, errPrefix)
	}

	execCmd.NoProgress = !progress.OptEnable.Get(src.Options)
	execCmd.Label = src.Handle + ": " + execCmd.Name
	execCmd.Stdin = ru.Stdin
	execCmd.Stdout = ru.Stdout
	execCmd.Stderr = ru.ErrOut
	execCmd.ErrPrefix = errPrefix

	if cmdFlagBool(cmd, flag.DBPrintToolCmd) || cmdFlagBool(cmd, flag.DBPrintLongToolCmd) {
		lg.FromContext(cmd.Context()).Info("Printing external cmd", lga.Cmd, execCmd)
		s := execCmd.String()
		_, err = fmt.Fprintln(ru.Out, s)
		return errz.Err(err)
	}

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		lg.FromContext(cmd.Context()).Info("Executing external cmd", lga.Cmd, execCmd)
		return execz.Exec(cmd.Context(), execCmd)
	default:
		return errz.Errorf("%s: not supported for %s", errPrefix, src.Type)
	}
}

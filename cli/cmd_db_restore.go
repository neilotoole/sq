package cli

import (
	"fmt"
	"strings"

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

func newDBRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore db catalog or cluster from dump",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	return cmd
}

func newDBRestoreCatalogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog @src [--from file.dump] [--cmd]",
		Short: "Restore db catalog from dump",
		Long: `Restore into @src from dump file, using the db-native restore tool.

If --from is specified, the dump is read from that file; otherwise stdin is used.

When --no-owner is specified, the source user will own the restored objects: the
ownership (and ACLs) from the dump file are disregarded.

If --cmd or --cmd-long are specified, the restore command is not executed, but
instead the db-native command is printed to stdout. Note that the command output
will include DB credentials. For a Postgres source, it would look something like:

 pg_restore -d 'postgres://alice:abc123@localhost:5432/sales' backup.dump`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHandle(1, true),
		RunE:              execDBRestoreCatalog,
		Example: `  # Restore @sakila_pg from backup.dump
  $ sq db restore catalog @sakila_pg -f backup.dump

  # With verbose output, and reading from stdin
  $ sq db restore catalog -v @sakila_pg < backup.dump

  # Don't use ownership from dump; the source user will own the restored objects
  $ sq db restore catalog  @sakila_pg --no-owner < backup.dump

  # Print the db-native restore command, but don't execute it
  $ sq db restore catalog @sakila_pg -f backup.dump --cmd`,
	}

	cmdMarkPlainStdout(cmd)
	cmd.Flags().StringP(flag.RestoreFrom, flag.RestoreFromShort, "", flag.RestoreFromUsage)
	cmd.Flags().Bool(flag.RestoreNoOwner, false, flag.RestoreNoOwnerUsage)
	cmd.Flags().StringP(flag.FileOutput, flag.FileOutputShort, "", flag.FileOutputUsage)
	cmd.Flags().Bool(flag.PrintToolCmd, false, flag.PrintToolCmdUsage)
	cmd.Flags().Bool(flag.PrintLongToolCmd, false, flag.PrintLongToolCmdUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.PrintToolCmd, flag.PrintLongToolCmd)

	return cmd
}

func execDBRestoreCatalog(cmd *cobra.Command, args []string) error {
	var (
		ru  = run.FromContext(cmd.Context())
		src *source.Source
		err error
		// fpDump is the (optional) path to the dump file.
		// If empty, stdin is used.
		dumpFile string
	)

	if src, err = ru.Config.Collection.Get(args[0]); err != nil {
		return err
	}

	errPrefix := "db restore catalog: " + src.Handle
	if cmdFlagChanged(cmd, flag.RestoreFrom) {
		if dumpFile = strings.TrimSpace(cmd.Flag(flag.RestoreFrom).Value.String()); dumpFile == "" {
			return errz.Errorf("%s: %s is specified, but empty", errPrefix, flag.RestoreFrom)
		}
	}

	if err = applySourceOptions(cmd, src); err != nil {
		return err
	}

	verbose := cmdFlagBool(cmd, flag.Verbose)
	noOwner := cmdFlagBool(cmd, flag.RestoreNoOwner)

	var execCmd *execz.Cmd

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		params := &postgres.ToolParams{
			Verbose: verbose,
			NoOwner: noOwner,
			File:    dumpFile,
		}
		execCmd, err = postgres.RestoreCatalogCmd(src, params)
	default:
		return errz.Errorf("%s: not supported for %s", errPrefix, src.Type)
	}

	if err != nil {
		return errz.Wrap(err, errPrefix)
	}

	execCmd.NoProgress = !OptProgress.Get(src.Options)
	execCmd.Label = src.Handle + ": " + execCmd.Name
	execCmd.Stdin = ru.Stdin
	execCmd.Stdout = ru.Stdout
	execCmd.Stderr = ru.ErrOut
	execCmd.ErrPrefix = errPrefix

	if cmdFlagBool(cmd, flag.PrintToolCmd) || cmdFlagBool(cmd, flag.PrintLongToolCmd) {
		lg.FromContext(cmd.Context()).Info("Printing OS cmd", lga.Cmd, execCmd)
		_, err = fmt.Fprintln(ru.Out, execCmd.String())
		return errz.Err(err)
	}

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		lg.FromContext(cmd.Context()).Info("Executing OS cmd", lga.Cmd, execCmd)
		return execz.Exec(cmd.Context(), execCmd)
	default:
		return errz.Errorf("%s: cmd not supported for %s", errPrefix, src.Type)
	}
}

func newDBRestoreClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster @src [--from file.dump] [--cmd]",
		Short: "Restore db cluster from dump",
		Long: `Restore entire db cluster into @src from dump file, using the db-native restore
tool.

If --from is specified, the dump is read from that file; otherwise stdin is used.

When --no-owner is specified, the source user will own the restored objects: the
ownership (and ACLs) from the dump file are disregarded.

If --cmd or --cmd-long are specified, the restore command is not executed, but
instead the db-native command is printed to stdout. Note that the command output
will include DB credentials. For a Postgres source, it would look something like:

FIXME: example command
 psql -d 'postgres://alice:abc123@localhost:5432/sales' backup.dump`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHandle(1, true),
		RunE:              execDBRestoreCluster,
		Example: `  # Restore @sakila_pg from backup.dump
  $ sq db restore cluster @sakila_pg -f backup.dump

  # With verbose output, and reading from stdin
  $ sq db restore cluster -v @sakila_pg < backup.dump

  # Don't use ownership from dump; the source user will own the restored objects
  $ sq db restore cluster  @sakila_pg --no-owner < backup.dump

  # Print the db-native restore command, but don't execute it
  $ sq db restore cluster @sakila_pg -f backup.dump --cmd`,
	}

	cmdMarkPlainStdout(cmd)
	cmd.Flags().StringP(flag.RestoreFrom, flag.RestoreFromShort, "", flag.RestoreFromUsage)
	cmd.Flags().Bool(flag.RestoreNoOwner, false, flag.RestoreNoOwnerUsage)
	cmd.Flags().StringP(flag.FileOutput, flag.FileOutputShort, "", flag.FileOutputUsage)
	cmd.Flags().Bool(flag.PrintToolCmd, false, flag.PrintToolCmdUsage)
	cmd.Flags().Bool(flag.PrintLongToolCmd, false, flag.PrintLongToolCmdUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.PrintToolCmd, flag.PrintLongToolCmd)

	return cmd
}

func execDBRestoreCluster(cmd *cobra.Command, args []string) error {
	var (
		ru  = run.FromContext(cmd.Context())
		src *source.Source
		err error
		// dumpFile is the (optional) path to the dump file.
		// If empty, stdin is used.
		dumpFile string
	)

	if src, err = ru.Config.Collection.Get(args[0]); err != nil {
		return err
	}

	errPrefix := "db restore cluster: " + src.Handle
	if cmdFlagChanged(cmd, flag.RestoreFrom) {
		if dumpFile = strings.TrimSpace(cmd.Flag(flag.RestoreFrom).Value.String()); dumpFile == "" {
			return errz.Errorf("%s: %s is specified, but empty", errPrefix, flag.RestoreFrom)
		}
	}

	if err = applySourceOptions(cmd, src); err != nil {
		return err
	}

	verbose := cmdFlagBool(cmd, flag.Verbose)

	// FIXME: get rid of noOwner from this command?
	// noOwner := cmdFlagBool(cmd, flag.RestoreNoOwner)

	var execCmd *execz.Cmd

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		params := &postgres.ToolParams{
			Verbose: verbose,
			File:    dumpFile,
		}
		execCmd, err = postgres.RestoreClusterCmd(src, params)
	default:
		return errz.Errorf("%s: not supported for %s", errPrefix, src.Type)
	}

	if err != nil {
		return errz.Wrap(err, errPrefix)
	}

	execCmd.NoProgress = !OptProgress.Get(src.Options)
	execCmd.Label = src.Handle + ": " + execCmd.Name
	execCmd.Stdin = ru.Stdin
	execCmd.Stdout = ru.Stdout
	execCmd.Stderr = ru.ErrOut
	execCmd.ErrPrefix = errPrefix

	if cmdFlagBool(cmd, flag.PrintToolCmd) || cmdFlagBool(cmd, flag.PrintLongToolCmd) {
		lg.FromContext(cmd.Context()).Info("Printing OS cmd", lga.Cmd, execCmd)
		s := execCmd.String()
		_, err = fmt.Fprintln(ru.Out, s)
		return errz.Err(err)
	}

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		lg.FromContext(cmd.Context()).Info("Executing OS cmd", lga.Cmd, execCmd)
		return execz.Exec(cmd.Context(), execCmd)
	default:
		return errz.Errorf("%s: not supported for %s", errPrefix, src.Type)
	}
}

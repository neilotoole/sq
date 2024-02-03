package cli

import (
	"strings"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/spf13/cobra"
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

	cmd.Flags().StringP(flag.RestoreFrom, flag.RestoreFromShort, "", flag.RestoreFromUsage)
	cmd.Flags().Bool(flag.RestoreNoOwner, false, flag.RestoreNoOwnerUsage)
	cmd.Flags().Bool(flag.PrintToolCmd, false, flag.PrintToolCmdUsage)
	cmd.Flags().Bool(flag.PrintLongToolCmd, false, flag.PrintLongToolCmdUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.PrintToolCmd, flag.PrintLongToolCmd)

	return cmd
}

//nolint:dupl
func execDBRestoreCatalog(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())

	var (
		src      *source.Source
		err      error
		shellCmd []string
		shellEnv []string
		// fpDump is the (optional) path to the dump file.
		// If empty, stdin is used.
		fpDump string
	)
	// $ sq db restore @sakila_pg --from backup.dump
	if src, err = ru.Config.Collection.Get(args[0]); err != nil {
		return err
	}

	errPrefix := "db restore catalog: " + src.Handle
	if cmdFlagChanged(cmd, flag.RestoreFrom) {
		if fpDump = strings.TrimSpace(cmd.Flag(flag.RestoreFrom).Value.String()); fpDump == "" {
			return errz.Errorf("%s: %s is specified, but empty", errPrefix, flag.RestoreFrom)
		}
	}

	if err = applySourceOptions(cmd, src); err != nil {
		return err
	}

	verbose := cmdFlagBool(cmd, flag.Verbose)
	noOwner := cmdFlagBool(cmd, flag.RestoreNoOwner)

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		params := &postgres.ToolParams{
			Verbose: verbose,
			NoOwner: noOwner,
			File:    fpDump,
		}
		shellCmd, shellEnv, err = postgres.RestoreCatalogCmd(src, params)
	default:
		return errz.Errorf("%s: not supported for %s", errPrefix, src.Type)
	}

	if err != nil {
		return errz.Wrap(err, errPrefix)
	}

	if cmdFlagBool(cmd, flag.PrintToolCmd) || cmdFlagBool(cmd, flag.PrintLongToolCmd) {
		return printToolCmd(ru, shellCmd, shellEnv)
	}

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		//return shellExecPgRestoreCatalog(ru, src, shellCmd, shellEnv)
		return shellExec(ru, errPrefix, shellCmd, shellEnv, true)
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

	cmd.Flags().StringP(flag.RestoreFrom, flag.RestoreFromShort, "", flag.RestoreFromUsage)
	cmd.Flags().Bool(flag.RestoreNoOwner, false, flag.RestoreNoOwnerUsage)
	cmd.Flags().Bool(flag.PrintToolCmd, false, flag.PrintToolCmdUsage)
	cmd.Flags().Bool(flag.PrintLongToolCmd, false, flag.PrintLongToolCmdUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.PrintToolCmd, flag.PrintLongToolCmd)

	return cmd
}

//nolint:dupl
func execDBRestoreCluster(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())

	var (
		src      *source.Source
		err      error
		shellCmd []string
		shellEnv []string
		// fpDump is the (optional) path to the dump file.
		// If empty, stdin is used.
		fpDump string
	)
	// $ sq db restore @sakila_pg --from backup.dump
	if src, err = ru.Config.Collection.Get(args[0]); err != nil {
		return err
	}

	if cmdFlagChanged(cmd, flag.RestoreFrom) {
		if fpDump = strings.TrimSpace(cmd.Flag(flag.RestoreFrom).Value.String()); fpDump == "" {
			return errz.Errorf("db restore cluster: %s: %s is specified, but empty", src.Handle, flag.RestoreFrom)
		}
	}

	if err = applySourceOptions(cmd, src); err != nil {
		return err
	}

	verbose := cmdFlagBool(cmd, flag.Verbose)
	noOwner := cmdFlagBool(cmd, flag.RestoreNoOwner)

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		params := &postgres.ToolParams{
			Verbose: verbose,
			NoOwner: noOwner,
			File:    fpDump,
		}
		shellCmd, shellEnv, err = postgres.RestoreCatalogCmd(src, params)
	default:
		return errz.Errorf("db restore cluster: %s: not supported for %s", src.Handle, src.Type)
	}

	if err != nil {
		return errz.Wrapf(err, "db restore cluster: %s", src.Handle)
	}

	if cmdFlagBool(cmd, flag.PrintToolCmd) || cmdFlagBool(cmd, flag.PrintLongToolCmd) {
		return printToolCmd(ru, shellCmd, shellEnv)
	}

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		return shellExecPgRestoreCluster(ru, src, shellCmd, shellEnv)
	default:
		return errz.Errorf("db restore cluster: %s: not supported for %s", src.Handle, src.Type)
	}
}

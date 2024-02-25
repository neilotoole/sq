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

func newDBDumpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump db catalog or cluster",
		Long:  `Execute or print db-native dump command for db catalog or cluster.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	return cmd
}

func newDBDumpCatalogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "catalog @src [--print]",
		Short:             "Dump db catalog",
		Long:              `Dump db catalog using database-native dump tool.`,
		ValidArgsFunction: completeHandle(1, true),
		Args:              cobra.MaximumNArgs(1),
		RunE:              execDBDumpCatalog,
		Example: `  # Dump @sakila_pg to file sakila.dump using pg_dump
  $ sq db dump catalog @sakila_pg -o sakila.dump

  # Same as above, but verbose mode, and dump via stdout
  $ sq db dump catalog @sakila_pg -v > sakila.dump

  # Dump without ownership or ACL
  $ sq db dump catalog --no-owner @sakila_pg > sakila.dump

  # Print the dump command, but don't execute it
  $ sq db dump catalog @sakila_pg --print

  # Dump a catalog (db) other than the source's current catalog
  $ sq db dump catalog @sakila_pg --catalog sales > sales.dump`,
	}

	// Calling cmdMarkPlainStdout means that ru.Stdout will be
	// the plain os.Stdout, and won't be decorated with color, or
	// progress listeners etc. The dump commands handle their own output.
	cmdMarkPlainStdout(cmd)

	cmd.Flags().String(flag.DBDumpCatalog, "", flag.DBDumpCatalogUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.DBDumpCatalog, completeCatalog(0)))
	cmd.Flags().Bool(flag.DBDumpNoOwner, false, flag.DBDumpNoOwnerUsage)
	cmd.Flags().StringP(flag.FileOutput, flag.FileOutputShort, "", flag.FileOutputUsage)
	cmd.Flags().Bool(flag.DBPrintToolCmd, false, flag.DBPrintToolCmdUsage)
	cmd.Flags().Bool(flag.DBPrintLongToolCmd, false, flag.DBPrintLongToolCmdUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.DBPrintToolCmd, flag.DBPrintLongToolCmd)

	return cmd
}

func execDBDumpCatalog(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())

	var (
		src *source.Source
		err error
	)

	if len(args) == 0 {
		if src = ru.Config.Collection.Active(); src == nil {
			return errz.New(msgNoActiveSrc)
		}
	} else if src, err = ru.Config.Collection.Get(args[0]); err != nil {
		return err
	}

	if err = applySourceOptions(cmd, src); err != nil {
		return err
	}

	if cmdFlagChanged(cmd, flag.DBDumpCatalog) {
		// Use a different catalog than the source's current catalog.
		if src.Catalog, err = cmd.Flags().GetString(flag.DBDumpCatalog); err != nil {
			return err
		}
	}

	var (
		errPrefix     = fmt.Sprintf("db dump catalog: %s", src.Handle)
		dumpVerbose   = OptVerbose.Get(src.Options)
		dumpNoOwner   = cmdFlagBool(cmd, flag.DBDumpNoOwner)
		dumpLongFlags = cmdFlagBool(cmd, flag.DBPrintLongToolCmd)
		dumpFile      string
	)

	if cmdFlagChanged(cmd, flag.FileOutput) {
		if dumpFile, err = cmd.Flags().GetString(flag.FileOutput); err != nil {
			return err
		}

		if dumpFile = strings.TrimSpace(dumpFile); dumpFile == "" {
			return errz.Errorf("%s: %s is specified, but empty", errPrefix, flag.FileOutput)
		}
	}

	var execCmd *execz.Cmd

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		params := &postgres.ToolParams{
			Verbose:   dumpVerbose,
			NoOwner:   dumpNoOwner,
			File:      dumpFile,
			LongFlags: dumpLongFlags,
		}
		execCmd, err = postgres.DumpCatalogCmd(src, params)
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
		_, err = fmt.Fprintln(ru.Out, execCmd.String())
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

func newDBDumpClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "cluster @src [--print]",
		Short:             "Dump entire db cluster",
		Long:              `Dump all catalogs in src's db cluster using the db-native dump tool.`,
		ValidArgsFunction: completeHandle(1, true),
		Args:              cobra.MaximumNArgs(1),
		RunE:              execDBDumpCluster,
		Example: `  # Dump all catalogs in @sakila_pg's cluster using pg_dumpall
  $ sq db dump cluster @sakila_pg -f all.dump

  # Same as above, but verbose mode and using stdout
  $ sq db dump cluster @sakila_pg -v > all.dump

  # Dump without ownership or ACL
  $ sq db dump cluster @sakila_pg --no-owner > all.dump

  # Print the dump command, but don't execute it
  $ sq db dump cluster @sakila_pg -f all.dump --print`,
	}

	// Calling cmdMarkPlainStdout means that ru.Stdout will be
	// the plain os.Stdout, and won't be decorated with color, or
	// progress listeners etc. The dump commands handle their own output.
	cmdMarkPlainStdout(cmd)
	cmd.Flags().Bool(flag.DBDumpNoOwner, false, flag.DBDumpNoOwnerUsage)
	cmd.Flags().StringP(flag.FileOutput, flag.FileOutputShort, "", flag.FileOutputUsage)
	cmd.Flags().Bool(flag.DBPrintToolCmd, false, flag.DBPrintToolCmdUsage)
	cmd.Flags().Bool(flag.DBPrintLongToolCmd, false, flag.DBPrintLongToolCmdUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.DBPrintToolCmd, flag.DBPrintLongToolCmd)

	return cmd
}

func execDBDumpCluster(cmd *cobra.Command, args []string) error {
	var (
		ru  = run.FromContext(cmd.Context())
		src *source.Source
		err error
	)

	if len(args) == 0 {
		if src = ru.Config.Collection.Active(); src == nil {
			return errz.New(msgNoActiveSrc)
		}
	} else if src, err = ru.Config.Collection.Get(args[0]); err != nil {
		return err
	}

	if err = applySourceOptions(cmd, src); err != nil {
		return err
	}

	var (
		errPrefix     = fmt.Sprintf("db dump cluster: %s", src.Handle)
		dumpVerbose   = OptVerbose.Get(src.Options)
		dumpNoOwner   = cmdFlagBool(cmd, flag.DBDumpNoOwner)
		dumpLongFlags = cmdFlagBool(cmd, flag.DBPrintLongToolCmd)
		dumpFile      string
	)

	if cmdFlagChanged(cmd, flag.FileOutput) {
		if dumpFile, err = cmd.Flags().GetString(flag.FileOutput); err != nil {
			return err
		}

		if dumpFile = strings.TrimSpace(dumpFile); dumpFile == "" {
			return errz.Errorf("%s: %s is specified, but empty", errPrefix, flag.FileOutput)
		}
	}

	var execCmd *execz.Cmd

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		params := &postgres.ToolParams{
			Verbose:   dumpVerbose,
			NoOwner:   dumpNoOwner,
			File:      dumpFile,
			LongFlags: dumpLongFlags,
		}
		execCmd, err = postgres.DumpClusterCmd(src, params)
	default:
		err = errz.Errorf("%s: not supported for %s", errPrefix, src.Type)
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
		_, err = fmt.Fprintln(ru.Out, execCmd.String())
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

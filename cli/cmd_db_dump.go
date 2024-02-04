package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/neilotoole/sq/libsq/core/execz"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/spf13/cobra"
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
		Use:               "catalog @src [--cmd]",
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
  $ sq db dump catalog @sakila_pg --cmd

  # Dump a catalog (db) other than the source's current catalog
  $ sq db dump catalog @sakila_pg --catalog sales > sales.dump`,
	}

	// Calling markCmdPlainStdout means that ru.Stdout will be
	// the plain os.Stdout, and won't be decorated with color, or
	// progress listeners etc. The dump commands handle their own output.
	markCmdPlainStdout(cmd)

	cmd.Flags().String(flag.DumpCatalog, "", flag.DumpCatalogUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.DumpCatalog, completeCatalog(0)))
	cmd.Flags().Bool(flag.DumpNoOwner, false, flag.DumpNoOwnerUsage)
	cmd.Flags().StringP(flag.FileOutput, flag.FileOutputShort, "", flag.FileOutputUsage)
	cmd.Flags().Bool(flag.PrintToolCmd, false, flag.PrintToolCmdUsage)
	cmd.Flags().Bool(flag.PrintLongToolCmd, false, flag.PrintLongToolCmdUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.PrintToolCmd, flag.PrintLongToolCmd)

	return cmd
}

func execDBDumpCatalog(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())

	var (
		src      *source.Source
		err      error
		shellCmd []string
		shellEnv []string
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

	if cmdFlagChanged(cmd, flag.DumpCatalog) {
		// Use a different catalog than the source's current catalog.
		if src.Catalog, err = cmd.Flags().GetString(flag.DumpCatalog); err != nil {
			return err
		}
	}

	var (
		errPrefix     = fmt.Sprintf("db dump catalog: %s", src.Handle)
		dumpVerbose   = cmdFlagBool(cmd, flag.Verbose)
		dumpNoOwner   = cmdFlagBool(cmd, flag.DumpNoOwner)
		dumpLongFlags = cmdFlagBool(cmd, flag.PrintLongToolCmd)
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

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		params := &postgres.ToolParams{
			Verbose:   dumpVerbose,
			NoOwner:   dumpNoOwner,
			File:      dumpFile,
			LongFlags: dumpLongFlags,
		}
		shellCmd, shellEnv, err = postgres.DumpCatalogCmd(src, params)
	default:
		return errz.Errorf("%s: not supported for %s", errPrefix, src.Type)
	}

	if err != nil {
		return errz.Wrap(err, errPrefix)
	}

	if cmdFlagBool(cmd, flag.PrintToolCmd) || cmdFlagBool(cmd, flag.PrintLongToolCmd) {
		return execz.PrintToolCmd(ru.Out, shellCmd, shellEnv)
	}

	c := &execz.ShellCommand{
		Stdin:              os.Stdin,
		Stdout:             os.Stdout,
		Stderr:             os.Stderr,
		ProgressFromStderr: false,
		ErrPrefix:          errPrefix,
		UsesOutputFile:     dumpFile,
		Name:               shellCmd[0],
		Args:               shellCmd[1:],
		Env:                shellEnv,
		CmdDirPath:         false,
	}

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		// return ShellExec(ru, errPrefix, false, dumpFile, shellCmd, shellEnv, false)
		return execz.ShellExec2(cmd.Context(), c)
	default:
		return errz.Errorf("%s: not supported for %s", errPrefix, src.Type)
	}
}

func newDBDumpClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "cluster @src [--cmd]",
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
  $ sq db dump cluster @sakila_pg -f all.dump --cmd`,
	}

	// Calling markCmdPlainStdout means that ru.Stdout will be
	// the plain os.Stdout, and won't be decorated with color, or
	// progress listeners etc. The dump commands handle their own output.
	markCmdPlainStdout(cmd)
	cmd.Flags().Bool(flag.DumpNoOwner, false, flag.DumpNoOwnerUsage)
	cmd.Flags().StringP(flag.FileOutput, flag.FileOutputShort, "", flag.FileOutputUsage)
	cmd.Flags().Bool(flag.PrintToolCmd, false, flag.PrintToolCmdUsage)
	cmd.Flags().Bool(flag.PrintLongToolCmd, false, flag.PrintLongToolCmdUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.PrintToolCmd, flag.PrintLongToolCmd)

	return cmd
}

func execDBDumpCluster(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())

	var (
		src      *source.Source
		err      error
		shellCmd []string
		shellEnv []string
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
		dumpVerbose   = cmdFlagBool(cmd, flag.Verbose)
		dumpNoOwner   = cmdFlagBool(cmd, flag.DumpNoOwner)
		dumpLongFlags = cmdFlagBool(cmd, flag.PrintLongToolCmd)
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

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		params := &postgres.ToolParams{
			Verbose:   dumpVerbose,
			NoOwner:   dumpNoOwner,
			File:      dumpFile,
			LongFlags: dumpLongFlags,
		}
		shellCmd, shellEnv, err = postgres.DumpClusterCmd(src, params)
	default:
		err = errz.Errorf("%s: not supported for %s", errPrefix, src.Type)
	}

	if err != nil {
		return errz.Wrap(err, errPrefix)
	}

	if cmdFlagBool(cmd, flag.PrintToolCmd) || cmdFlagBool(cmd, flag.PrintLongToolCmd) {
		return execz.PrintToolCmd(ru.Out, shellCmd, shellEnv)
	}

	c := &execz.ShellCommand{
		Stdin:              os.Stdin,
		Stdout:             os.Stdout,
		Stderr:             os.Stderr,
		ProgressFromStderr: false,
		ErrPrefix:          errPrefix,
		UsesOutputFile:     dumpFile,
		Name:               shellCmd[0],
		Args:               shellCmd[1:],
		Env:                shellEnv,
		CmdDirPath:         true,
	}

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		// return shellExecPgDumpCluster(ru, src, shellCmd, shellEnv)
		return execz.ShellExec2(cmd.Context(), c)
		// return ShellExec(ru, errPrefix, false, dumpFile, shellCmd, shellEnv, true)
	default:
		return errz.Errorf("db dump cluster: %s: not supported for %s", src.Handle, src.Type)
	}
}

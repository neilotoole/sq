package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/neilotoole/sq/libsq/core/stringz"

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
		Use:               "dump @src [--cmd] [--all] [-v]",
		Short:             "Dump db",
		Long:              `Dump db the database-native dump tool`,
		ValidArgsFunction: completeHandle(1, true),
		Args:              cobra.MaximumNArgs(1),
		RunE:              execDBDump,
		Example: `  # Dump @sakila_pg using pg_dump, in verbose mode
	$ sq db dump -v @sakila_pg > sakila.dump

	# Print the dump command, but don't execute it
	$ sq db dump @sakila_pg --cmd

	# Dump the entire db cluster (all catalogs)
	$ sq db dump @sakila_pg --all

	# Dump a catalog (db) other than the source's current catalog
  $ sq db dump @sakila_pg --catalog sales > sales.dump`,
	}

	// TODO:
	// - Add more examples above

	// TODO: Add options:
	// --format=archive,text,dir?
	// --schema bool (if not set, dump all schemas)
	//

	markCmdPlainStdout(cmd)
	cmd.Flags().String(flag.DumpCatalog, "", flag.DumpCatalogUsage)
	cmd.Flags().Bool(flag.DumpAll, false, flag.DumpAllUsage)
	cmd.MarkFlagsMutuallyExclusive(flag.DumpAll, flag.DumpCatalog)
	cmd.Flags().Bool(flag.DumpNoOwner, false, flag.DumpNoOwnerUsage)
	cmd.Flags().StringP(flag.DumpFile, flag.DumpFileShort, "", flag.DumpNoOwnerUsage)
	cmd.Flags().Bool(flag.DumpCmd, false, flag.DumpCmdUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.DumpCatalog, completeCatalog(0)))

	return cmd
}

func execDBDump(cmd *cobra.Command, args []string) error {
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
		dumpAll     = cmdFlagBool(cmd, flag.DumpAll)
		dumpVerbose = cmdFlagBool(cmd, flag.Verbose)
		dumpNoOwner = cmdFlagBool(cmd, flag.DumpNoOwner)
	)

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		params := &postgres.ToolParams{
			Verbose:   dumpVerbose,
			NoOwner:   dumpNoOwner,
			File:      "",
			LongFlags: false,
		}
		if dumpAll {
			shellCmd, shellEnv, err = postgres.DumpAllCmd(src, dumpVerbose)
			break
		}

		shellCmd, shellEnv, err = postgres.DumpCmd(src, dumpVerbose)
	default:
		err = errz.Errorf("not supported for %s", src.Type)
	}

	if err != nil {
		return errz.Wrapf(err, "db dump: %s", src.Handle)
	}

	if cmdFlagBool(cmd, flag.DumpCmd) {
		for i := range shellCmd {
			shellCmd[i] = stringz.ShellEscape(shellCmd[i])
		}
		for i := range shellEnv {
			shellEnv[i] = stringz.ShellEscape(shellEnv[i])
		}

		if len(shellEnv) == 0 {
			fmt.Fprintln(ru.Out, strings.Join(shellCmd, " "))
		} else {
			fmt.Fprintln(ru.Out, strings.Join(shellEnv, " ")+" "+strings.Join(shellCmd, " "))
		}

		return nil
	}

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		if dumpAll {
			return shellExecPgDumpAll(ru, src, shellCmd, shellEnv)
		}
		return shellExecPgDump(ru, src, shellCmd, shellEnv)
	default:
		return errz.Errorf("db dump: %s: cmd execution not supported for %s", src.Handle, src.Type)
	}
}

func shellExecPgDump(ru *run.Run, src *source.Source, shellCmd, shellEnv []string) error {
	c := exec.CommandContext(ru.Cmd.Context(), shellCmd[0], shellCmd[1:]...) //nolint:gosec
	c.Env = append(c.Env, shellEnv...)

	// FIXME: switch to ru.Out?
	c.Stdout = os.Stdout
	c.Stderr = &bytes.Buffer{}

	if err := c.Run(); err != nil {
		return newShellExecError(fmt.Sprintf("db dump: %s", src.Handle), c, err)
	}
	return nil
}

func shellExecPgDumpAll(ru *run.Run, src *source.Source, shellCmd, shellEnv []string) error {
	c := exec.CommandContext(ru.Cmd.Context(), shellCmd[0], shellCmd[1:]...) //nolint:gosec

	// PATH shenanigans are required to ensure that pg_dumpall can find pg_dump.
	// Otherwise we see this error:
	//
	//  pg_dumpall: error: program "pg_dump" is needed by pg_dumpall but was not
	//   found in the same directory as "pg_dumpall"
	c.Env = append(c.Env, "PATH="+filepath.Dir(c.Path))
	c.Env = append(c.Env, shellEnv...)

	c.Stdout = os.Stdout
	c.Stderr = &bytes.Buffer{}
	if err := c.Run(); err != nil {
		return newShellExecError(fmt.Sprintf("db dump --all: %s", src.Handle), c, err)
	}
	return nil
}

package cli

import (
	"bytes"
	"fmt"
	"os/exec"
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

func newDBRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore @src [--from file.dump] [--cmd]",
		Short: "Restore db from dump",
		Long: `
Restore db into @src from dump file, using the db-native restore tool.

If --from is specified, the dump is read from that file; otherwise stdin is used.

When --no-owner is specified, the source user will own the restored objects: the
ownership (and ACLs) from the dump file are disregarded.

If --cmd is specified, the restore command is not executed, but instead the
db-native command is printed to stdout. Note that the command output will
include DB credentials. For a Postgres source, it would look something like:

 pg_restore -d 'postgres://alice:abc123@localhost:5432/sales' backup.dump`,
		ValidArgsFunction: completeHandle(1, true),
		Args:              cobra.ExactArgs(1),
		RunE:              execDBRestore,
		Example: `  # Restore @sakila_pg from backup.dump
  $ sq db restore @sakila_pg -f backup.dump

  # With verbose output, and reading from stdin
  $ sq db restore -v @sakila_pg < backup.dump

  # Don't use ownership from dump; the source user will own the restored objects
  $ sq db restore @sakila_pg --no-owner < backup.dump

  # Print the db-native restore command, but don't execute it
  $ sq db restore @sakila_pg -f backup.dump --cmd`,
	}

	cmd.Flags().StringP(flag.RestoreFrom, flag.RestoreFromShort, "", flag.RestoreFromUsage)
	cmd.Flags().Bool(flag.RestoreNoOwner, false, flag.RestoreNoOwnerUsage)
	cmd.Flags().Bool(flag.RestoreCmd, false, flag.RestoreCmdUsage)

	return cmd
}

func execDBRestore(cmd *cobra.Command, args []string) error {
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
			return errz.Errorf("db restore: %s: %s is specified, but empty", src.Handle, flag.RestoreFrom)
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
		//if restoreAll {
		//	shellCmd, shellEnv, err = postgres.RestoreAllCmd(src, restoreVerbose)
		//	break
		//}
		shellCmd, shellEnv, err = postgres.RestoreCmd(src, params)
	default:
		err = errz.Errorf("not supported for %s", src.Type)
	}

	if err != nil {
		return errz.Wrapf(err, "db restore: %s", src.Handle)
	}

	if cmdFlagBool(cmd, flag.RestoreCmd) {
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
		return shellExecPgRestore(ru, src, shellCmd, shellEnv)
	default:
		return errz.Errorf("db restore: %s: cmd execution not supported for %s", src.Handle, src.Type)
	}
}

// shellExecPgRestore executes the pg_restore command. Arg dump is always
// closed after this function returns.
//
//nolint:gocritic
func shellExecPgRestore(ru *run.Run, src *source.Source, shellCmd, shellEnv []string) error {
	// - https://www.postgresql.org/docs/9.6/app-pgrestore.html

	c := exec.CommandContext(ru.Cmd.Context(), shellCmd[0], shellCmd[1:]...) //nolint:gosec
	c.Env = append(c.Env, shellEnv...)
	c.Stdin = ru.Stdin
	c.Stdout = ru.Out
	c.Stderr = &bytes.Buffer{}

	if err := c.Run(); err != nil {
		return newShellExecError(fmt.Sprintf("db restore: %s", src.Handle), c, err)
	}
	return nil
}

//nolint:gocritic
func shellExecPgRestoreAll(ru *run.Run, src *source.Source, shellCmd, shellEnv []string) error {
	_ = ru
	_ = src
	_ = shellCmd
	_ = shellEnv

	return errz.New("not implemented")
	//c := exec.CommandContext(ru.Cmd.Context(), shellCmd[0], shellCmd[1:]...) //nolint:gosec
	//
	//// PATH shenanigans are required to ensure that pg_dumpall can find pg_dump.
	//// Otherwise we see this error:
	////
	////  pg_dumpall: error: program "pg_dump" is needed by pg_dumpall but was not
	////   found in the same directory as "pg_dumpall"
	//c.Env = append(c.Env, "PATH="+filepath.Dir(c.Path))
	//c.Env = append(c.Env, shellEnv...)
	//
	//c.Stdout = os.Stdout
	//c.Stderr = &bytes.Buffer{}
	//if err := c.Run(); err != nil {
	//	return newShellExecError(fmt.Sprintf("db dump --all: %s", src.Handle), c, err)
	//}
	//return nil
}

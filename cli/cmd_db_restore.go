package cli

import (
	"fmt"
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
		Use:               "restore @src [--cmd] [--all] [-v]",
		Short:             "Restore db from dump",
		Long:              `Restore db from dump using the database-native dump tool`,
		ValidArgsFunction: completeHandle(1, true),
		Args:              cobra.MaximumNArgs(1),
		RunE:              execDBRestore,
		Example: `  # Restore using db-native tool (pg_restore for PostgreSQL)
	$ sq db restore @sakila_pg < backup.dump

	# Print the restore command, but don't execute it
	$ sq db restore @sakila_pg --cmd`,
	}

	// TODO:
	// - Add more examples above

	// TODO: Add options:
	// --format=archive,text,dir?
	// --schema bool (if not set, dump all schemas)
	//

	markCmdPlainStdout(cmd)
	cmd.Flags().Bool(flag.RestoreCmd, false, flag.RestoreCmdUsage)
	cmd.Flags().Bool(flag.RestoreCmdAll, false, flag.RestoreCmdAllUsage)

	return cmd
}

func execDBRestore(cmd *cobra.Command, args []string) error {
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
		restoreAll     = cmdFlagBool(cmd, flag.RestoreCmdAll)
		restoreVerbose = cmdFlagBool(cmd, flag.Verbose)
	)

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		if restoreAll {
			shellCmd, shellEnv, err = postgres.DumpAllCmd(src, restoreVerbose)
			break
		}
		shellCmd, shellEnv, err = postgres.RestoreAllCmd(src, restoreVerbose)
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
		if restoreAll {
			return shellExecPgRestoreAll(ru, src, shellCmd, shellEnv)
		}
		return shellExecPgRestore(ru, src, shellCmd, shellEnv)
	default:
		return errz.Errorf("db restore: %s: cmd execution not supported for %s", src.Handle, src.Type)
	}
}

//nolint:gocritic
func shellExecPgRestore(ru *run.Run, src *source.Source, shellCmd, shellEnv []string) error {
	_ = ru
	_ = src
	_ = shellCmd
	_ = shellEnv
	return errz.New("not implemented")

	// https://www.postgresql.org/docs/current/app-pgrestore.html

	//c := exec.CommandContext(ru.Cmd.Context(), shellCmd[0], shellCmd[1:]...) //nolint:gosec
	//c.Env = append(c.Env, shellEnv...)
	//
	//// FIXME: switch to ru.Out?
	//c.Stdout = os.Stdout
	//c.Stderr = &bytes.Buffer{}
	//
	//if err := c.Run(); err != nil {
	//	return newShellExecError(fmt.Sprintf("db dump: %s", src.Handle), c, err)
	//}
	//return nil
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

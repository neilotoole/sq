package cli

import (
	"bytes"
	"fmt"
	"os"
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

func newDBDumpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "dump @src [--cmd]",
		Short:             "Dump database",
		Long:              `Dump database using the database-native dump tool`,
		ValidArgsFunction: completeHandle(1, true),
		Args:              cobra.MaximumNArgs(1),
		RunE:              execDBDump,
		Example: `  # Dump using the db-native dump tool
	$ sq db dump @sakila > sakila.dump

	# Print the dump command, but don't execute it
	$ sq db dump @sakila --cmd`,
	}

	// TODO: Add options:
	// --format=archive,text,dir?
	// --schema bool (if not set, dump all schemas)
	//

	markCmdPlainStdout(cmd)
	cmd.Flags().Bool(flag.DumpCmd, false, flag.DumpCmdUsage)
	cmd.Flags().Bool(flag.DumpCmdAll, false, flag.DumpCmdAllUsage)

	return cmd
}

func execDBDump(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())

	var (
		src      *source.Source
		err      error
		shellCmd []string
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

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		if cmdFlagBool(cmd, flag.DumpCmdAll) {
			shellCmd, err = postgres.DumpAllCmd(src)
			break
		}
		shellCmd, err = postgres.DumpCmd(src)
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
		fmt.Fprintln(ru.Out, strings.Join(shellCmd, " "))
		return nil
	}

	switch src.Type { //nolint:exhaustive
	case drivertype.Pg:
		return shellExecDumpCmdPg(ru, src, shellCmd)
	default:
		return errz.Errorf("db dump: %s: cmd execution not supported for %s", src.Handle, src.Type)
	}
}

func shellExecDumpCmdPg(ru *run.Run, src *source.Source, shellCmd []string) error {
	ctx := ru.Cmd.Context()

	execCmd := exec.CommandContext(ctx, shellCmd[0], shellCmd[1:]...) //nolint:gosec

	// FIXME: switch to ru.Out?
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = &bytes.Buffer{}

	execErr := execCmd.Run()
	if execErr == nil {
		return nil
	}

	msg := fmt.Sprintf("db dump: %s", src.Handle)
	shellErr := newShellExecError(msg, execCmd, execErr)
	return shellErr
}

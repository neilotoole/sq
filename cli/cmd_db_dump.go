package cli

import (
	"fmt"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
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

	//addTextFormatFlags(cmd)
	cmd.Flags().Bool(flag.DumpCmd, false, flag.DumpCmdUsage)
	panicOn(cmd.MarkFlagRequired(flag.DumpCmd)) // FIXME: temporarily required until fully implemented
	//cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)

	return cmd

}

func execDBDump(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)

	var err error
	var src *source.Source
	if len(args) == 0 {
		// Get the active data source
		src = ru.Config.Collection.Active()
		if src == nil {
			return errz.New(msgNoActiveSrc)
		}
	} else if src, err = ru.Config.Collection.Get(args[0]); err != nil {
		return err
	}

	switch src.Type {
	case drivertype.Pg:
		return execDBDumpPostgres(cmd, src)
	default:
		return errz.New("Currently only postgres is supported")

	}
}

func execDBDumpPostgres(cmd *cobra.Command, src *source.Source) error {
	ru := run.FromContext(cmd.Context())
	isPrintDump := cmdFlagBool(cmd, flag.DumpCmd)
	if !isPrintDump {
		return errz.New("dump: currently only --cmd is currently supported")
	}

	fmt.Fprintf(ru.Out, "pg_dump: %s\n", src.Location)
	return nil
}

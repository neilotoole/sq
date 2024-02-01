package cli

import (
	"fmt"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/spf13/cobra"
	"strconv"
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
	//addTextFormatFlags(cmd)
	cmd.Flags().Bool(flag.DumpCmd, false, flag.DumpCmdUsage)
	//panicOn(cmd.MarkFlagRequired(flag.DumpCmd)) // FIXME: temporarily required until fully implemented
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
	// https://www.postgresql.org/docs/current/app-pgdump.html
	// https://www.postgresql.org/docs/9.6/app-pgdump.html
	// pg_dump -Fc mydb > db.dump
	// pg_dump -c -F=c mydb > db.dump
	// pg_dump -Fc -n 'public' mydb > db.dump

	ctx := cmd.Context()
	ru := run.FromContext(ctx)
	isPrintDump := cmdFlagBool(cmd, flag.DumpCmd)
	if !isPrintDump {
		return errz.New("dump: currently only --cmd is currently supported")
	}

	//grip, err := ru.Grips.Open(ctx, src)
	//if err != nil {
	//	return err
	//}
	//
	//db, err := grip.DB(ctx)
	//if err != nil {
	//	return err
	//}
	//
	//drvr := grip.SQLDriver()
	//catalog, err := drvr.CurrentCatalog(ctx, db)
	//if err != nil {
	//	return err
	//}
	//if err = grip.Close(); err != nil {
	//	return err
	//}

	pgCfg, err := postgres.NativeConfig(src)
	if err != nil {
		return err
	}

	var text string

	if pgCfg.Password == "" {
		text = "PGPASSWORD=''"
	} else {
		text = "PGPASSWORD=" + stringz.ShellEscape(pgCfg.Password)
	}

	text += " pg_dump -Fc"
	if pgCfg.Port != 0 && pgCfg.Port != 5432 {
		text += " -p " + strconv.Itoa(int(pgCfg.Port))
	}

	if pgCfg.User != "" {
		text += " -U " + stringz.ShellEscape(pgCfg.User)
	}

	if pgCfg.Host != "" {
		text += " -h " + pgCfg.Host
	}

	if pgCfg.Database != "" {
		text += " " + pgCfg.Database
	}

	fmt.Fprintln(ru.Out, text)
	return nil
}

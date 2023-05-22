package cli

import (
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
)

func newInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "inspect [@HANDLE|@HANDLE.TABLE|.TABLE]",
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: (&handleTableCompleter{
			max: 1,
		}).complete,
		RunE:  execInspect,
		Short: "Inspect data source schema and stats",
		Long: `Inspect a data source, or a particular table in a source,
listing table details such as column names and row counts, etc.

NOTE: If a schema is large, it may take some time for the command to complete.

The --dbprops flag shows the database properties for a source's *underlying*
database. The flag is disregarded when inspecting a table.

Use the --verbose flag to see more detail in some output formats.

If @HANDLE is not provided, the active data source is assumed.`,
		Example: `  # Inspect active data source
  $ sq inspect

  # Inspect @pg1 data source
  $ sq inspect @pg1

  # Inspect @pg1 data source, showing verbose output
  $ sq inspect -v @pg1

  # Show DB properties for @pg1
  $ sq inspect --dbprops @pg1

  # Inspect 'actor' in @pg1 data source
  $ sq inspect @pg1.actor

  # Inspect 'actor' in active data source
  $ sq inspect .actor

  # Inspect piped data
  $ cat data.xlsx | sq inspect`,
	}

	addTextFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)

	cmd.Flags().BoolP(flag.InspectDBProps, flag.InspectDBPropsShort, false, flag.InspectDBPropsUsage)

	return cmd
}

func execInspect(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)

	coll := ru.Config.Collection

	var src *source.Source
	var table string
	var err error

	if len(args) == 0 {
		// No args supplied.

		// There are two paths from here:
		// - There's input on stdin, which we'll inspect, or
		// - We're inspecting the active src

		// check if there's input on stdin
		src, err = checkStdinSource(ctx, ru)
		if err != nil {
			return err
		}

		if src != nil {
			// We have a valid source on stdin.

			// Add the source to the set.
			err = coll.Add(src)
			if err != nil {
				return err
			}

			// Collection the stdin pipe data source as the active source,
			// as it's commonly the only data source the user is acting upon.
			src, err = coll.SetActive(src.Handle, false)
			if err != nil {
				return err
			}
		} else {
			// No source on stdin. Let's see if there's an active source.
			src = coll.Active()
			if src == nil {
				return errz.Errorf("no data source specified and no active data source")
			}
		}
	} else {
		// We received an argument, which can be one of these forms:
		//   @sakila			  -- inspect the named source
		//   @sakila.actor	-- inspect a table of the named source
		//   .actor		      -- inspect a table from the active source
		var handle string
		handle, table, err = source.ParseTableHandle(args[0])
		if err != nil {
			return errz.Wrap(err, "invalid input")
		}

		if handle == "" {
			src = coll.Active()
			if src == nil {
				return errz.Errorf("no data source specified and no active data source")
			}
		} else {
			src, err = coll.Get(handle)
			if err != nil {
				return err
			}
		}
	}

	if err = applySourceOptions(cmd, src); err != nil {
		return err
	}

	dbase, err := ru.Databases.Open(ctx, src)
	if err != nil {
		return errz.Wrapf(err, "failed to inspect %s", src.Handle)
	}

	if table != "" {
		var tblMeta *source.TableMetadata
		tblMeta, err = dbase.TableMetadata(ctx, table)
		if err != nil {
			return err
		}

		return ru.Writers.Metadata.TableMetadata(tblMeta)
	}

	if cmdFlagIsSetTrue(cmd, flag.InspectDBProps) {
		sqlDrvr := dbase.SQLDriver()
		var props map[string]any
		if props, err = sqlDrvr.DBProperties(ctx, dbase.DB()); err != nil {
			return err
		}

		return ru.Writers.Metadata.DBProperties(props)
	}

	srcMeta, err := dbase.SourceMetadata(ctx)
	if err != nil {
		return errz.Wrapf(err, "failed to read %s source metadata", src.Handle)
	}

	// This is a bit hacky, but it works... if not "--verbose", then just zap
	// the DBVars, as we usually don't want to see those
	if !cmdFlagIsSetTrue(cmd, flag.Verbose) {
		srcMeta.DBProperties = nil
	}

	return ru.Writers.Metadata.SourceMetadata(srcMeta)
}

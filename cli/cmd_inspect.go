package cli

import (
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
listing table details such as column names and row counts, etc. Use
the --verbose flag to see more detail.
If @HANDLE is not provided, the active data source is assumed.`,
		Example: `  # inspect active data source
  $ sq inspect
  
  # inspect @pg1 data source
  $ sq inspect @pg1
  
  # inspect @pg1 data source, showing verbose output
  $ sq inspect -v @pg1

  # inspect 'actor' in @pg1 data source
  $ sq inspect @pg1.actor
  
  # inspect 'actor' in active data source
  $ sq inspect .actor
  
  # inspect piped data
  $ cat data.xlsx | sq inspect`,
	}

	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)
	cmd.Flags().BoolP(flagTable, flagTableShort, false, flagTableUsage)

	return cmd
}

func execInspect(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	rc := RunContextFrom(ctx)

	srcs := rc.Config.Sources

	var src *source.Source
	var table string
	var err error

	if len(args) == 0 { //nolint:nestif
		// No args supplied.

		// There are two paths from here:
		// - There's input on stdin, which we'll inspect, or
		// - We're inspecting the active src

		// check if there's input on stdin
		src, err = checkStdinSource(ctx, rc)
		if err != nil {
			return err
		}

		if src != nil {
			// We have a valid source on stdin.

			// Add the source to the set.
			err = srcs.Add(src)
			if err != nil {
				return err
			}

			// Set the stdin pipe data source as the active source,
			// as it's commonly the only data source the user is acting upon.
			src, err = srcs.SetActive(src.Handle)
			if err != nil {
				return err
			}
		} else {
			// No source on stdin. Let's see if there's an active source.
			src = srcs.Active()
			if src == nil {
				return errz.Errorf("no data source specified and no active data source")
			}
		}
	} else {
		// We received an argument, which can be one of these forms:
		//   @my1			-- inspect the named source
		//   @my1.tbluser	-- inspect a table of the named source
		//   .tbluser		-- inspect a table from the active source
		var handle string
		handle, table, err = source.ParseTableHandle(args[0])
		if err != nil {
			return errz.Wrap(err, "invalid input")
		}

		if handle == "" {
			src = srcs.Active()
			if src == nil {
				return errz.Errorf("no data source specified and no active data source")
			}
		} else {
			src, err = srcs.Get(handle)
			if err != nil {
				return err
			}
		}
	}

	dbase, err := rc.databases.Open(ctx, src)
	if err != nil {
		return errz.Wrapf(err, "failed to inspect %s", src.Handle)
	}

	if table != "" {
		var tblMeta *source.TableMetadata
		tblMeta, err = dbase.TableMetadata(ctx, table)
		if err != nil {
			return err
		}

		return rc.writers.metaw.TableMetadata(tblMeta)
	}

	meta, err := dbase.SourceMetadata(ctx)
	if err != nil {
		return errz.Wrapf(err, "failed to read %s source metadata", src.Handle)
	}

	// This is a bit hacky, but it works... if not "--verbose", then just zap
	// the DBVars, as we usually don't want to see those
	if !cmdFlagTrue(cmd, flagVerbose) {
		meta.DBVars = nil
	}

	return rc.writers.metaw.SourceMetadata(meta)
}

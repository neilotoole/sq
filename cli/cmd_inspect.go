package cli

import (
	"context"
	"database/sql"
	"slices"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
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

If @HANDLE is not provided, the active data source is assumed. If @HANDLE.TABLE
is provided, table inspection is performed (as opposed to source inspection).

There are several modes of operation, controlled by flags. Each of the following
modes apply only to source inspection, not table inspection. When no mode flag
is supplied, the default is to show the source metadata and schema.

  --overview:  Displays the source's metadata, but not the schema.

  --dbprops:   Displays database properties for the sources' *underlying* database.

  --catalogs:  List the catalogs (databases) available via the source.

  --schemata:  List the schemas available in the source's active catalog.

Use --verbose with --text format to see more detail. The --json and --yaml
formats both show extensive detail.`,
		Example: `  # Inspect active data source.
  $ sq inspect

  # Inspect @pg1 data source.
  $ sq inspect @pg1

  # Inspect @pg1 data source, showing verbose output.
  $ sq inspect -v @pg1

  # Show output in JSON (useful for piping to jq).
  $ sq inspect --json @pg1

  # Show output in YAML.
  $ sq inspect --yaml @pg1

  # Show only the DB properties for @pg1.
  $ sq inspect --dbprops @pg1

  # Show only the source metadata (and not schema details).
  $ sq inspect --overview @pg1

  # List the schemas in @pg1.
  $ sq inspect --schemata @pg1

  # List the catalogs in @pg1.
  $ sq inspect --catalogs @pg1

  # Inspect table "actor" in @pg1 data source.
  $ sq inspect @pg1.actor

  # Inspect "actor" in active data source.
  $ sq inspect .actor

  # Inspect a non-default schema in source @my1.
  $ sq inspect @my1 --src.schema information_schema

  # Inspect piped data.
  $ cat data.xlsx | sq inspect`,
	}

	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)

	cmd.Flags().BoolP(flag.InspectOverview, flag.InspectOverviewShort, false, flag.InspectOverviewUsage)
	cmd.Flags().BoolP(flag.InspectDBProps, flag.InspectDBPropsShort, false, flag.InspectDBPropsUsage)
	cmd.Flags().BoolP(flag.InspectCatalogs, flag.InspectCatalogsShort, false, flag.InspectCatalogsUsage)
	cmd.Flags().BoolP(flag.InspectSchemata, flag.InspectSchemataShort, false, flag.InspectSchemataUsage)

	cmd.MarkFlagsMutuallyExclusive(flag.InspectOverview, flag.InspectDBProps, flag.InspectCatalogs, flag.InspectSchemata)

	cmd.Flags().String(flag.ActiveSchema, "", flag.ActiveSchemaUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.ActiveSchema,
		activeSchemaCompleter{getActiveSourceViaArgs}.complete))

	return cmd
}

func execInspect(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ru, log := run.FromContext(ctx), lg.FromContext(ctx)

	src, table, err := determineInspectTarget(ctx, ru, args)
	if err != nil {
		return err
	}

	// Handle flag.ActiveSchema (--src.schema=SCHEMA). This func will mutate
	// src's Catalog and Schema fields if appropriate.
	if err = processFlagActiveSchema(cmd, src); err != nil {
		return err
	}

	if err = applySourceOptions(cmd, src); err != nil {
		return err
	}

	pool, err := ru.Sources.Open(ctx, src)
	if err != nil {
		return errz.Wrapf(err, "failed to inspect %s", src.Handle)
	}

	if table != "" {
		if flagName, changed := cmdFlagAnyChanged(
			cmd,
			flag.InspectCatalogs,
			flag.InspectSchemata,
			flag.InspectDBProps,
			flag.InspectOverview,
		); changed {
			return errz.Errorf("flag --%s is not valid when inspecting a table", flagName)
		}

		var tblMeta *metadata.Table
		tblMeta, err = pool.TableMetadata(ctx, table)
		if err != nil {
			return err
		}

		return ru.Writers.Metadata.TableMetadata(tblMeta)
	}

	if cmdFlagIsSetTrue(cmd, flag.InspectCatalogs) {
		var db *sql.DB
		if db, err = pool.DB(ctx); err != nil {
			return err
		}
		var catalogs []string
		if catalogs, err = pool.SQLDriver().ListCatalogs(ctx, db); err != nil {
			return err
		}

		var currentCatalog string
		if len(catalogs) > 0 {
			currentCatalog = catalogs[0]
		}

		slices.Sort(catalogs)
		return ru.Writers.Metadata.Catalogs(currentCatalog, catalogs)
	}

	if cmdFlagIsSetTrue(cmd, flag.InspectSchemata) {
		var db *sql.DB
		if db, err = pool.DB(ctx); err != nil {
			return err
		}
		var schemas []*metadata.Schema
		if schemas, err = pool.SQLDriver().ListSchemaMetadata(ctx, db); err != nil {
			return err
		}

		var currentSchema string
		if currentSchema, err = pool.SQLDriver().CurrentSchema(ctx, db); err != nil {
			return err
		}

		return ru.Writers.Metadata.Schemata(currentSchema, schemas)
	}

	if cmdFlagIsSetTrue(cmd, flag.InspectDBProps) {
		var db *sql.DB
		if db, err = pool.DB(ctx); err != nil {
			return err
		}
		defer lg.WarnIfCloseError(log, lgm.CloseDB, db)
		var props map[string]any
		if props, err = pool.SQLDriver().DBProperties(ctx, db); err != nil {
			return err
		}

		return ru.Writers.Metadata.DBProperties(props)
	}

	overviewOnly := cmdFlagIsSetTrue(cmd, flag.InspectOverview)

	srcMeta, err := pool.SourceMetadata(ctx, overviewOnly)
	if err != nil {
		return errz.Wrapf(err, "failed to read %s source metadata", src.Handle)
	}

	// This is a bit hacky, but it works... if not "--verbose", then just zap
	// the DBVars, as we usually don't want to see those
	if !cmdFlagIsSetTrue(cmd, flag.Verbose) {
		srcMeta.DBProperties = nil
	}

	return ru.Writers.Metadata.SourceMetadata(srcMeta, !overviewOnly)
}

// determineInspectTarget determines the source (and, optionally, table)
// to inspect.
func determineInspectTarget(ctx context.Context, ru *run.Run, args []string) (
	src *source.Source, table string, err error,
) {
	coll := ru.Config.Collection
	if len(args) == 0 {
		// No args supplied.

		// There are two paths from here:
		// - There's input on stdin, which we'll inspect, or
		// - We're inspecting the active src

		// check if there's input on stdin
		src, err = checkStdinSource(ctx, ru)
		if err != nil {
			return nil, "", err
		}

		if src != nil {
			// We have a valid source on stdin.

			// Add the source to the set.
			err = coll.Add(src)
			if err != nil {
				return nil, "", err
			}

			// Set the stdin pipe data source as the active source,
			// as it's commonly the only data source the user is acting upon.
			src, err = coll.SetActive(src.Handle, false)
			if err != nil {
				return nil, "", err
			}
		} else {
			// No source on stdin. Let's see if there's an active source.
			src = coll.Active()
			if src == nil {
				return nil, "", errz.Errorf("no data source specified and no active data source")
			}
		}

		return src, "", nil
	}

	// Else, we received an argument, which can be one of these forms:
	//   @sakila			  -- inspect the named source
	//   @sakila.actor	-- inspect a table of the named source
	//   .actor		      -- inspect a table from the active source
	var handle string
	handle, table, err = source.ParseTableHandle(args[0])
	if err != nil {
		return nil, "", errz.Wrap(err, "invalid input")
	}

	if handle == "" {
		src = coll.Active()
		if src == nil {
			return nil, "", errz.Errorf("no data source specified and no active data source")
		}
	} else {
		src, err = coll.Get(handle)
		if err != nil {
			return nil, "", err
		}
	}

	return src, table, nil
}

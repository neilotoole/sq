package cli

import (
	"context"
	"database/sql"
	"slices"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/termz"
	"github.com/neilotoole/sq/libsq/driver"
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
formats both show extensive detail. The --markdown and --html formats each
render a schema document that includes a Mermaid entity-relationship diagram;
--html produces a standalone page (use --output to save it to a file).`,
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

  # Show output as a Markdown schema doc with a Mermaid ER diagram.
  $ sq inspect --markdown @pg1

  # Show output as a standalone HTML schema doc with a Mermaid ER diagram.
  $ sq inspect --html @pg1

  # Write the HTML schema doc to a file instead of stdout.
  $ sq inspect --html @pg1 -o pg1-schema.html

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

	addOptionFlag(cmd.Flags(), OptFormat)
	// Only suggest the formats inspect actually implements a metadata writer
	// for; any other format falls back to text output (see newWriters), so
	// offering e.g. csv/xlsx/xml here would imply support that doesn't exist.
	panicOn(cmd.RegisterFlagCompletionFunc(
		OptFormat.Flag().Name,
		completeStrings(-1,
			format.Text.String(),
			format.JSON.String(),
			format.YAML.String(),
			format.Markdown.String(),
			format.HTML.String(),
			format.MermaidERD.String(),
			format.SVGERD.String(),
			format.PNGERD.String(),
		),
	))
	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	addOptionFlag(cmd.Flags(), OptCompact)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	// Override the generic flag usage: for inspect these emit a schema
	// document (with a Mermaid ER diagram), not a Markdown/HTML data table.
	cmd.Flags().Bool(flag.Markdown, false, "Output a Markdown schema document")
	cmd.Flags().Bool(flag.HTML, false, "Output a standalone HTML schema document")
	// Modifier for --html / -f=html: inline assets (Mermaid.js) for offline use.
	addOptionFlag(cmd.Flags(), OptHTMLEmbedAssets)

	cmd.Flags().BoolP(flag.InspectOverview, flag.InspectOverviewShort, false, flag.InspectOverviewUsage)
	cmd.Flags().BoolP(flag.InspectDBProps, flag.InspectDBPropsShort, false, flag.InspectDBPropsUsage)
	cmd.Flags().BoolP(flag.InspectCatalogs, flag.InspectCatalogsShort, false, flag.InspectCatalogsUsage)
	cmd.Flags().BoolP(flag.InspectSchemata, flag.InspectSchemataShort, false, flag.InspectSchemataUsage)

	cmd.MarkFlagsMutuallyExclusive(flag.InspectOverview, flag.InspectDBProps, flag.InspectCatalogs, flag.InspectSchemata)

	cmd.Flags().String(flag.ActiveSchema, "", flag.ActiveSchemaUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.ActiveSchema,
		activeSchemaCompleter{getActiveSourceViaArgs}.complete))
	addOptionFlag(cmd.Flags(), driver.OptIngestCache)

	cmd.Flags().StringP(flag.FileOutput, flag.FileOutputShort, "", flag.FileOutputUsage)

	cmd.Flags().StringP(flag.Input, flag.InputShort, "", flag.InputUsage)
	panicOn(cmd.Flags().MarkHidden(flag.Input)) // Hide for now; this is mostly used for testing.
	return cmd
}

func execInspect(cmd *cobra.Command, args []string) error {
	// inspect is wholly read-only; every open it performs passes the mode
	// as an explicit argument.
	ctx := cmd.Context()
	ru, log := run.FromContext(ctx), lg.FromContext(ctx)

	o, err := getOptionsFromCmd(cmd)
	if err != nil {
		return err
	}
	if err = errBinaryFormatToTerminal(
		getFormat(cmd, o),
		cmdFlagChanged(cmd, flag.FileOutput),
		termz.IsTerminal(ru.Stdout),
	); err != nil {
		return err
	}

	src, table, err := determineInspectTarget(ctx, ru, args)
	if err != nil {
		return err
	}

	// Handle flag.ActiveSchema (--src.schema=SCHEMA). This func will mutate
	// src's Catalog and Schema fields if appropriate.
	var srcModified bool
	if srcModified, err = processFlagActiveSchema(cmd, src); err != nil {
		return err
	}

	if err = applySourceOptions(cmd, src); err != nil {
		return err
	}

	if srcModified {
		if err = verifySourceCatalogSchema(ctx, ru, src, driver.ModeReadOnly); err != nil {
			return err
		}
	}

	grip, err := ru.Grips.Open(ctx, src, driver.ModeReadOnly)
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
		tblMeta, err = grip.TableMetadata(ctx, table)
		if err != nil {
			return err
		}

		return ru.Writers.Metadata.TableMetadata(tblMeta)
	}

	if cmdFlagIsSetTrue(cmd, flag.InspectCatalogs) {
		var db *sql.DB
		if db, err = grip.DB(ctx); err != nil {
			return err
		}
		var catalogs []string
		if catalogs, err = grip.SQLDriver().ListCatalogs(ctx, db); err != nil {
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
		if db, err = grip.DB(ctx); err != nil {
			return err
		}
		var schemas []*metadata.Schema
		if schemas, err = grip.SQLDriver().ListSchemaMetadata(ctx, db); err != nil {
			return err
		}

		var currentSchema string
		if currentSchema, err = grip.SQLDriver().CurrentSchema(ctx, db); err != nil {
			return err
		}

		return ru.Writers.Metadata.Schemata(currentSchema, schemas)
	}

	if cmdFlagIsSetTrue(cmd, flag.InspectDBProps) {
		var db *sql.DB
		if db, err = grip.DB(ctx); err != nil {
			return err
		}
		defer lg.WarnIfCloseError(log, lgm.CloseDB, db)
		var props map[string]any
		if props, err = grip.SQLDriver().DBProperties(ctx, db); err != nil {
			return err
		}

		return ru.Writers.Metadata.DBProperties(props)
	}

	overviewOnly := cmdFlagIsSetTrue(cmd, flag.InspectOverview)

	srcMeta, err := grip.SourceMetadata(ctx, overviewOnly)
	if err != nil {
		return errz.Wrapf(err, "failed to read %s source metadata", src.Handle)
	}

	// Reset srcMeta.Location to the stored (template) location for
	// display. Drivers populate srcMeta.Location from the grip's stored
	// src, which doOpen always replaces with the resolver-expanded
	// clone; without this override, sq inspect would always show the
	// resolved value, leaking placeholder targets. The writer layer's
	// expand decorator (see expand_writer.go) then applies --expand
	// expansion centrally, so the displayed value is the stored
	// template by default, or the expanded value when --expand is set.
	// SecretsResolved is carried so the decorator skips re-expanding an
	// already-resolved location (e.g. a stdin source), matching the
	// guard on the source/group/collection expand paths.
	srcMeta.Location = src.Location
	srcMeta.SecretsResolved = src.SecretsResolved

	// This is a bit hacky, but it works... if not "--verbose", then just zap
	// the DBVars, as we usually don't want to see those
	if !OptVerbose.Get(src.Options) {
		srcMeta.DBProperties = nil
	}

	return ru.Writers.Metadata.SourceMetadata(srcMeta, !overviewOnly)
}

// errBinaryFormatToTerminal returns a guard error when fm is a binary image
// format (png-erd) bound for a terminal without a file target: writing PNG
// bytes to a TTY would corrupt the terminal. It returns nil for any other
// format, when a file output target (-o/--output) is set, or when stdout is
// not a terminal (a pipe, redirect, or file). svg-erd is plain-text image
// markup and needs no such guard.
func errBinaryFormatToTerminal(fm format.Format, fileOutputSet, stdoutIsTerminal bool) error {
	if fm != format.PNGERD || fileOutputSet || !stdoutIsTerminal {
		return nil
	}
	return errz.Errorf(
		"%s is a binary image format and would corrupt the terminal; "+
			"write it to a file with -o/--output (e.g. -o schema.png)", format.PNGERD)
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

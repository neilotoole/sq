package cli

import (
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/diff"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/output/tablew"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

var OptDiffNumLines = options.NewInt(
	"diff.lines",
	&options.Flag{Name: "unified", Short: 'U'},
	3,
	"Generate diffs with <n> lines of context",
	`Generate diffs with <n> lines of context, where n >= 0.`,
	options.TagOutput,
)

var OptDiffHunkMaxSize = options.NewInt(
	"diff.hunk.max-size",
	nil,
	10000,
	"Maximum size of individual diff hunks",
	`Maximum size of individual diff hunks. A hunk is a segment of a diff that
contains differing lines, as well as non-differing context lines before and
after the difference. A hunk must be loaded into memory in its entirety; this
setting prevents excessive memory usage. If a hunk would exceed this limit, it
is split into multiple hunks; this still produces a well-formed diff.`,
	options.TagOutput,
)

var OptDiffDataFormat = format.NewOpt(
	"diff.data.format",
	&options.Flag{Name: "format", Short: 'f'},
	format.Text,
	func(f format.Format) error {
		switch f { //nolint:exhaustive
		case format.Text, format.CSV, format.TSV,
			format.JSON, format.JSONA, format.JSONL,
			format.Markdown, format.HTML, format.XML, format.YAML:
			return nil
		default:
			return errz.Errorf("diff does not support output format {%s}", f)
		}
	},
	"Output format (json, csv…) when comparing data",
	`Specify the output format to use when comparing table data.

Available formats:

  text, csv, tsv,
  json, jsona, jsonl,
  markdown, html, xml, yaml`,
)

// diffFormats contains fewer formats than those in format.All.
// That's because some of them are not text-based, e.g. XLSX,
// and thus cause trouble with the text/line-based diff functionality.
var diffFormats = []format.Format{
	format.Text, format.CSV, format.TSV,
	format.JSON, format.JSONA, format.JSONL,
	format.Markdown, format.HTML, format.XML, format.YAML,
}

var allDiffElementsFlags = []string{
	flag.DiffAll,
	flag.DiffOverview,
	flag.DiffSchema,
	flag.DiffDBProps,
	flag.DiffRowCount,
	flag.DiffData,
}

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff @HANDLE1[.TABLE] @HANDLE2[.TABLE] [--data]",
		Short: "BETA: Compare sources, or tables",
		Long: `BETA: Compare metadata, or row data, of sources and tables.

CAUTION: This feature is in beta testing. Please report any issues:

  https://github.com/neilotoole/sq/issues/new/choose

When comparing sources ("source diff"), the default behavior is to diff the
source overview, schema, and table row counts. Table row data is not compared.

When comparing tables ("table diff"), the default is to diff table schema and
row counts. Table row data is not compared.

Use flags to specify the elements you want to compare. The available
elements are:

  --overview   source metadata, without schema (source diff only)
  --dbprops    database/server properties (source diff only)
  --schema     schema structure, for database or individual table
  --counts     show row counts when using --schema
  --data       row data values
  --all        all of the above

Flag --data diffs the values of each row in the compared tables. Use with
caution with large tables.

Use --format with --data to specify the format to render the diff records.
Line-based formats (e.g. "text" or "jsonl") are often the most ergonomic,
although "yaml" may be preferable for comparing column values. The
available formats are:

  text, csv, tsv,
  json, jsona, jsonl,
  markdown, html, xml, yaml

The default format can be changed via:

  $ sq config set diff.data.format FORMAT

The --format flag only applies with data diffs (--data). Metadata diffs are
always output in YAML.

Note that --overview and --dbprops only apply to source diffs, not table diffs.

Flag --unified (-U) controls the number of lines to show surrounding a diff.
The default (3) can be changed via:

  $ sq config set diff.lines N`,
		Args: cobra.ExactArgs(2),
		ValidArgsFunction: (&handleTableCompleter{
			handleRequired: true,
			max:            2,
		}).complete,
		RunE: execDiff,
		Example: `
  Metadata diff
  -------------

  # Diff sources (compare default elements).
  $ sq diff @prod/sakila @staging/sakila

  # As above, but show 7 lines surrounding each diff.
  $ sq diff @prod/sakila @staging/sakila -U7

  # Diff sources, but only compare source overview.
  $ sq diff @prod/sakila @staging/sakila --overview

  # Diff sources, but only DB properties.
  $ sq diff @prod/sakila @staging/sakila --dbprops

  # Compare source overview, and DB properties.
  $ sq diff @prod/sakila @staging/sakila -OB

  # Diff sources, but only compare schema.
  $ sq diff @prod/sakila @staging/sakila --schema

  # Compare schema table structure, and row counts.
  $ sq diff @prod/sakila @staging/sakila --SN

  # Compare everything, including table data. Caution: can be slow.
  $ sq diff @prod/sakila @staging/sakila --all

  # Compare metadata of actor table in prod vs staging.
  $ sq diff @prod/sakila.actor @staging/sakila.actor

  Row data diff
  -------------

  # Compare data in the actor tables.
  $ sq diff @prod/sakila.actor @staging/sakila.actor --data

  # Compare data in the actor tables, but output in JSONL.
  $ sq diff @prod/sakila.actor @staging/sakila.actor --data --format jsonl

  # Compare data in all tables and views. Caution: may be slow.
  $ sq diff @prod/sakila @staging/sakila --data`,
	}

	addOptionFlag(cmd.Flags(), OptDiffNumLines)
	addOptionFlag(cmd.Flags(), OptDiffDataFormat)

	cmd.Flags().BoolP(flag.DiffOverview, flag.DiffOverviewShort, false, flag.DiffOverviewUsage)
	cmd.Flags().BoolP(flag.DiffDBProps, flag.DiffDBPropsShort, false, flag.DiffDBPropsUsage)
	cmd.Flags().BoolP(flag.DiffSchema, flag.DiffSchemaShort, false, flag.DiffSchemaUsage)
	cmd.Flags().BoolP(flag.DiffRowCount, flag.DiffRowCountShort, false, flag.DiffRowCountUsage)
	cmd.Flags().BoolP(flag.DiffData, flag.DiffDataShort, false, flag.DiffDataUsage)
	cmd.Flags().BoolP(flag.DiffAll, flag.DiffAllShort, false, flag.DiffAllUsage)

	// If flag.DiffAll is provided, no other diff elements flag can be provided.
	nonAllFlags := lo.Drop(allDiffElementsFlags, 0)
	for i := range nonAllFlags {
		cmd.MarkFlagsMutuallyExclusive(flag.DiffAll, nonAllFlags[i])
	}

	panicOn(cmd.RegisterFlagCompletionFunc(
		OptDiffDataFormat.Flag().Name,
		completeStrings(-1, stringz.Strings(diffFormats)...),
	))

	addOptionFlag(cmd.Flags(), driver.OptIngestCache)
	return cmd
}

// execDiff compares sources or tables.
func execDiff(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)

	handle1, table1, err := source.ParseTableHandle(args[0])
	if err != nil {
		return errz.Wrapf(err, "invalid input (1st arg): %s", args[0])
	}

	handle2, table2, err := source.ParseTableHandle(args[1])
	if err != nil {
		return errz.Wrapf(err, "invalid input (2nd arg): %s", args[1])
	}

	o, err := getOptionsFromCmd(cmd)
	if err != nil {
		return err
	}

	f := OptDiffDataFormat.Get(o)
	recwFn := getRecordWriterFunc(f)
	if recwFn == nil {
		// Shouldn't happen
		lg.From(cmd).Warn("No record writer impl for format", "format", f)
		recwFn = tablew.NewRecordWriter
	}

	diffCfg := &diff.Config{
		Lines:          OptDiffNumLines.Get(o),
		HunkMaxSize:    OptDiffHunkMaxSize.Get(o),
		RecordWriterFn: recwFn,
	}

	if diffCfg.Lines < 0 {
		return errz.Errorf("number of lines to show must be >= 0")
	}

	switch {
	case table1 == "" && table2 == "":
		elems := getDiffSourceElements(cmd)
		return diff.ExecSourceDiff(ctx, ru, diffCfg, elems, handle1, handle2)
	case table1 == "" || table2 == "":
		return errz.Errorf("invalid args: both must be either @HANDLE or @HANDLE.TABLE")
	default:
		elems := getDiffTableElements(cmd)
		return diff.ExecTableDiff(ctx, ru, diffCfg, elems, handle1, table1, handle2, table2)
	}
}

func getDiffSourceElements(cmd *cobra.Command) *diff.Elements {
	if !isAnyDiffElementsFlagChanged(cmd) {
		// Default
		return &diff.Elements{
			Overview:     true,
			DBProperties: false,
			Schema:       true,
			RowCount:     true,
			Data:         false,
		}
	}

	if cmdFlagChanged(cmd, flag.DiffAll) {
		return &diff.Elements{
			Overview:     true,
			DBProperties: true,
			Schema:       true,
			RowCount:     true,
			Data:         true,
		}
	}

	return &diff.Elements{
		Overview:     cmdFlagIsSetTrue(cmd, flag.DiffOverview),
		DBProperties: cmdFlagIsSetTrue(cmd, flag.DiffDBProps),
		Schema:       cmdFlagIsSetTrue(cmd, flag.DiffSchema),
		RowCount:     cmdFlagIsSetTrue(cmd, flag.DiffRowCount),
		Data:         cmdFlagIsSetTrue(cmd, flag.DiffData),
	}
}

func getDiffTableElements(cmd *cobra.Command) *diff.Elements {
	if !isAnyDiffElementsFlagChanged(cmd) {
		// Default
		return &diff.Elements{
			Schema:   true,
			RowCount: true,
		}
	}

	if cmdFlagChanged(cmd, flag.DiffAll) {
		return &diff.Elements{
			Schema:   true,
			RowCount: true,
			Data:     true,
		}
	}

	return &diff.Elements{
		Schema:   cmdFlagIsSetTrue(cmd, flag.DiffSchema),
		RowCount: cmdFlagIsSetTrue(cmd, flag.DiffRowCount),
		Data:     cmdFlagIsSetTrue(cmd, flag.DiffData),
	}
}

func isAnyDiffElementsFlagChanged(cmd *cobra.Command) bool {
	for _, name := range allDiffElementsFlags {
		if cmdFlagChanged(cmd, name) {
			return true
		}
	}
	return false
}

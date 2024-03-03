package cli

import (
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/csvw"
	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/diff"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tuning"
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

var OptDiffStopAfter = options.NewInt(
	"diff.stop",
	&options.Flag{Name: "stop", Short: 'n'},
	3,
	"Stop after <n> differences",
	`Stop after <n> differences are found. If n <= 0, no limit is applied.`,
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

Use flags to specify the modes you want to compare. The available modes are:

  --overview   source metadata, without schema (source diff only)
  --dbprops    database/server properties (source diff only)
  --schema     schema structure, for database or individual table
  --counts     show row counts when using --schema
  --data       row data values
  --all        all of the above

Flag --data diffs the values of each row in the compared tables, until the stop
limit is reached. Use the --stop (-n) flag or the diff.stop config option to
specify the stop limit. The default is 3.

Use --format with --data to specify the format to render the diff records.
Line-based formats (e.g. "text" or "jsonl") are often the most ergonomic,
although "yaml" may be preferable for comparing column values. The available
formats are:

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

  $ sq config set diff.lines N

Exit status is 0 if inputs are the same, 1 if different, 2 on any error.`,
		Args: cobra.ExactArgs(2),
		ValidArgsFunction: (&handleTableCompleter{
			handleRequired: true,
			max:            2,
		}).complete,
		RunE: execDiff,
		Example: `  Metadata diff
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

  # Compare data in the actor tables, stopping at the first difference.
  $ sq diff @prod/sakila.actor @staging/sakila.actor --data --stop 1

  # Compare data in the actor tables, but output in JSONL.
  $ sq diff @prod/sakila.actor @staging/sakila.actor --data --format jsonl

  # Compare data in all tables and views. Caution: may be slow.
  $ sq diff @prod/sakila @staging/sakila --data --stop 0`,
	}

	addOptionFlag(cmd.Flags(), OptDiffNumLines)
	addOptionFlag(cmd.Flags(), OptDiffStopAfter)
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
func execDiff(cmd *cobra.Command, args []string) (err error) {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)

	var foundDiffs bool
	defer func() {
		// From GNU diff help:
		// > Exit status is 0 if inputs are the same, 1 if different, 2 if trouble.
		switch {
		case err != nil:
			err = errz.WithExitCode(err, 2)
		case foundDiffs:
			// We want to exit 1 if diffs were found.
			err = errz.WithExitCode(errz.ErrNoMsg, 1)
		}
	}()

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

	src1, err := ru.Config.Collection.Get(handle1)
	if err != nil {
		return err
	}
	src2, err := ru.Config.Collection.Get(handle2)
	if err != nil {
		return err
	}

	numLines := OptDiffNumLines.Get(o)
	if numLines < 0 {
		return errz.Errorf("number of lines to show must be >= 0")
	}

	diffCfg := &diff.Config{
		Run:         ru,
		Lines:       numLines,
		StopAfter:   OptDiffStopAfter.Get(o),
		HunkMaxSize: OptDiffHunkMaxSize.Get(o),
		Printing:    ru.Writers.PrOut.Clone(),
		Colors:      ru.Writers.PrOut.Diff.Clone(),
		Concurrency: tuning.OptErrgroupLimit.Get(options.FromContext(ctx)),
	}

	if diffCfg.RecordHunkWriter, err = getDiffRecordWriter(
		OptDiffDataFormat.Get(o),
		ru.Writers.PrOut,
		numLines,
	); err != nil {
		return err
	}

	switch {
	case table1 == "" && table2 == "":
		diffCfg.Modes = getDiffSourceElements(cmd)
		foundDiffs, err = diff.ExecSourceDiff(ctx, diffCfg, src1, src2)
	case table1 == "" || table2 == "":
		return errz.Errorf("invalid args: both must be either @HANDLE or @HANDLE.TABLE")
	default:
		diffCfg.Modes = getDiffTableElements(cmd)
		foundDiffs, err = diff.ExecTableDiff(ctx, diffCfg, src1, table1, src2, table2)
	}

	return err
}

func getDiffSourceElements(cmd *cobra.Command) *diff.Modes {
	if !isAnyDiffElementsFlagChanged(cmd) {
		// Default
		return &diff.Modes{
			Overview:     true,
			DBProperties: false,
			Schema:       true,
			RowCount:     true,
			Data:         false,
		}
	}

	if cmdFlagChanged(cmd, flag.DiffAll) {
		return &diff.Modes{
			Overview:     true,
			DBProperties: true,
			Schema:       true,
			RowCount:     true,
			Data:         true,
		}
	}

	return &diff.Modes{
		Overview:     cmdFlagIsSetTrue(cmd, flag.DiffOverview),
		DBProperties: cmdFlagIsSetTrue(cmd, flag.DiffDBProps),
		Schema:       cmdFlagIsSetTrue(cmd, flag.DiffSchema),
		RowCount:     cmdFlagIsSetTrue(cmd, flag.DiffRowCount),
		Data:         cmdFlagIsSetTrue(cmd, flag.DiffData),
	}
}

func getDiffTableElements(cmd *cobra.Command) *diff.Modes {
	if !isAnyDiffElementsFlagChanged(cmd) {
		// Default
		return &diff.Modes{
			Schema:   true,
			RowCount: true,
		}
	}

	if cmdFlagChanged(cmd, flag.DiffAll) {
		return &diff.Modes{
			Schema:   true,
			RowCount: true,
			Data:     true,
		}
	}

	return &diff.Modes{
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

func getDiffRecordWriter(f format.Format, pr *output.Printing, lines int) (diff.RecordHunkWriter, error) {
	if f == format.CSV {
		// Currently we've only implemented an "optimized" (and I say that loosely)
		// diff writer for CSV. There's no technical reason the others can't be
		// implemented; just haven't gotten around to it yet.
		return csvw.NewDiffWriter(pr), nil
	}

	// All the rest of the formats have to use the adapter.
	recWriterFn := getRecordWriterFunc(f)
	if recWriterFn == nil {
		// Shouldn't happen
		return nil, errz.Errorf("no diff record writer impl for format: %s", f)
	}

	return diff.NewRecordHunkWriterAdapter(pr, recWriterFn, lines), nil
}

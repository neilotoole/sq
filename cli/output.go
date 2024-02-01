package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	colorable "github.com/mattn/go-colorable"
	wordwrap "github.com/mitchellh/go-wordwrap"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/csvw"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/output/htmlw"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/cli/output/markdownw"
	"github.com/neilotoole/sq/cli/output/raww"
	"github.com/neilotoole/sq/cli/output/tablew"
	"github.com/neilotoole/sq/cli/output/xlsxw"
	"github.com/neilotoole/sq/cli/output/xmlw"
	"github.com/neilotoole/sq/cli/output/yamlw"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/timez"
)

var (
	OptPrintHeader = options.NewBool(
		"header",
		"",
		false,
		0,
		true,
		"Print header row",
		`Controls whether a header row is printed. This applies only to certain formats,
such as "text" or "csv".`,
		options.TagOutput,
	)

	OptFormat = format.NewOpt(
		"format",
		"format",
		'f',
		format.Text,
		nil,
		"Specify output format",
		`Specify the output format. Some formats are only implemented for a subset of
sq's commands. If the specified format is not available for a particular
command, sq falls back to "text". Available formats:

  text, csv, tsv, xlsx,
  json, jsona, jsonl,
  markdown, html, xlsx, xml, yaml, raw`,
	)

	OptErrorFormat = format.NewOpt(
		"error.format",
		"",
		0,
		format.Text,
		func(f format.Format) error {
			if f == format.Text || f == format.JSON {
				return nil
			}

			return errz.Errorf("option {error.format} allows only %q or %q", format.Text, format.JSON)
		},
		"Error output format",
		fmt.Sprintf(`The format to output errors in. Allowed formats are %q or %q.`, format.Text, format.JSON),
	)

	OptVerbose = options.NewBool(
		"verbose",
		"",
		false,
		'v',
		false,
		"Print verbose output",
		`Print verbose output.`,
		options.TagOutput,
	)

	OptMonochrome = options.NewBool(
		"monochrome",
		"",
		false,
		'M',
		false,
		"Don't print color output",
		`Don't print color output.`,
		options.TagOutput,
	)

	OptProgress = options.NewBool(
		"progress",
		"no-progress",
		true,
		0,
		true,
		"Progress bar for long-running operations",
		`Progress bar for long-running operations.`,
		options.TagOutput,
	)

	OptProgressDelay = options.NewDuration(
		"progress.delay",
		"",
		0,
		time.Second*2,
		"Progress bar render delay",
		`Delay before showing a progress bar.`,
	)

	OptDebugTrackMemory = options.NewDuration(
		"debug.stats.frequency",
		"",
		0,
		0,
		"Memory usage sampling interval.",
		`Memory usage sampling interval. If non-zero, peak memory usage is periodically
sampled, and reported on exit. If zero, memory usage sampling is disabled.`,
	)

	OptCompact = options.NewBool(
		"compact",
		"",
		false,
		'c',
		false,
		"Compact instead of pretty-printed output",
		`Compact instead of pretty-printed output.`,
		options.TagOutput,
	)

	OptTuningFlushThreshold = options.NewInt(
		"tuning.flush-threshold",
		"",
		0,
		1000,
		"Output writer buffer flush threshold in bytes",
		`Size in bytes after which output writers should flush any internal buffer.
Generally, it is not necessary to fiddle this knob.`,
		options.TagTuning,
	)

	timeLayoutsList = "Predefined values:\n" + stringz.IndentLines(
		wordwrap.WrapString(strings.Join(timez.NamedLayouts(), ", "), 64),
		"  ")

	OptDatetimeFormat = options.NewString(
		"format.datetime",
		"",
		0,
		"RFC3339",
		nil,
		"Timestamp format: constant such as RFC3339 or a strftime format",
		`Timestamp format. This can be one of several predefined constants such as
"RFC3339" or "Unix", or a strftime format such as "%Y-%m-%d %H:%M:%S".

`+timeLayoutsList,
		options.TagOutput,
	)

	OptDatetimeFormatAsNumber = options.NewBool(
		"format.datetime.number",
		"",
		false,
		0,
		true,
		"Render numeric datetime value as number instead of string",
		`Render numeric datetime value as number instead of string, if possible. If
format.datetime renders a numeric value (e.g. a Unix timestamp such as
"1591843854"), that value is typically rendered as a string. For some output
formats, such as JSON, it can be useful to instead render the value as a naked
number instead of a string. Note that this option is no-op if the rendered value
is not an integer.

  format.datetime.number=false
  [{"first_name":"PENELOPE","last_update":"1591843854"}]
  format.datetime.number=true
  [{"first_name":"PENELOPE","last_update":1591843854}]
`,
		options.TagOutput,
	)

	OptDateFormat = options.NewString(
		"format.date",
		"",
		0,
		"DateOnly",
		nil,
		"Date format: constant such as DateOnly or a strftime format",
		`Date format. This can be one of several predefined constants such as "DateOnly"
or "Unix", or a strftime format such as "%Y-%m-%d". Note that date values are
sometimes programmatically indistinguishable from datetime values. In that
situation, use format.datetime instead.

`+timeLayoutsList,
		options.TagOutput,
	)

	OptDateFormatAsNumber = options.NewBool(
		"format.date.number",
		"",
		false,
		0,
		true,
		"Render numeric date value as number instead of string",
		`Render numeric date value as number instead of string, if possible. If
format.date renders a numeric value (e.g. a year such as "1979"), that value is
typically rendered as a string. For some output formats, such as JSON, it can be
useful to instead render the value as a naked number instead of a string. Note
that this option is no-op if the rendered value is not an integer.

  format.date.number=false
  [{"first_name":"PENELOPE","birth_year":"1979"}]
  format.date.number=true
  [{"first_name":"PENELOPE","birth_year":1979}]
`,
		options.TagOutput,
	)

	OptTimeFormat = options.NewString(
		"format.time",
		"",
		0,
		"TimeOnly",
		nil,
		"Time format: constant such as TimeOnly or a strftime format",
		`Time format. This can be one of several predefined constants such as "TimeOnly"
or "Unix", or a strftime format such as "%Y-%m-%d". Note that time values are
sometimes programmatically indistinguishable from datetime values. In that
situation, use format.datetime instead.

`+timeLayoutsList,
		options.TagOutput,
	)

	OptTimeFormatAsNumber = options.NewBool(
		"format.time.number",
		"",
		false,
		0,
		true,
		"Render numeric time value as number instead of string",
		`
Render numeric time value as number instead of string, if possible. If format.time
renders a numeric value (e.g. "59"), that value is typically rendered as a string.
For some output formats, such as JSON, it can be useful to instead render the
value as a naked number instead of a string. Note that this option is no-op if
the rendered value is not an integer.

  format.time.number=false
  [{"first_name":"PENELOPE","favorite_minute":"59"}]
  format.time.number=true
  [{"first_name":"PENELOPE","favorite_minute":59}]
`,
		options.TagOutput,
	)
)

// newWriters returns an output.Writers instance configured per defaults and/or
// flags from cmd. The returned out2/errOut2 values may differ
// from the out/errOut args (e.g. decorated to support colorization).
func newWriters(cmd *cobra.Command, clnup *cleanup.Cleanup, o options.Options,
	out, errOut io.Writer,
) (w *output.Writers, out2, errOut2 io.Writer) {
	var pr *output.Printing
	pr, out2, errOut2 = getPrinting(cmd, clnup, o, out, errOut)
	log := logFrom(cmd)

	// Package tablew has writer impls for each of the writer interfaces,
	// so we use its Writers as the baseline. Later we check the format
	// flags and set the various writer fields depending upon which
	// writers the format implements.
	w = &output.Writers{
		Printing: pr,
		Record:   tablew.NewRecordWriter(out2, pr),
		Metadata: tablew.NewMetadataWriter(out2, pr),
		Source:   tablew.NewSourceWriter(out2, pr),
		Ping:     tablew.NewPingWriter(out2, pr),
		Error:    tablew.NewErrorWriter(errOut2, pr),
		Version:  tablew.NewVersionWriter(out2, pr),
		Config:   tablew.NewConfigWriter(out2, pr),
	}

	if OptErrorFormat.Get(o) == format.JSON {
		// This logic works because the only supported values are text and json.
		w.Error = jsonw.NewErrorWriter(log, errOut2, pr)
	}

	// Invoke getFormat to see if the format was specified
	// via config or flag.
	fm := getFormat(cmd, o)

	//nolint:exhaustive
	switch fm {
	case format.JSON:
		// No format specified, use JSON
		w.Metadata = jsonw.NewMetadataWriter(out2, pr)
		w.Source = jsonw.NewSourceWriter(out2, pr)
		w.Version = jsonw.NewVersionWriter(out2, pr)
		w.Ping = jsonw.NewPingWriter(out2, pr)
		w.Config = jsonw.NewConfigWriter(out2, pr)

	case format.Text:
		// Don't delete this case, it's actually needed due to
		// the slightly odd logic that determines format.

	case format.TSV:
		w.Ping = csvw.NewPingWriter(out2, csvw.Tab)

	case format.CSV:
		w.Ping = csvw.NewPingWriter(out2, csvw.Comma)

	case format.YAML:
		w.Config = yamlw.NewConfigWriter(out2, pr)
		w.Metadata = yamlw.NewMetadataWriter(out2, pr)
		w.Source = yamlw.NewSourceWriter(out2, pr)
		w.Version = yamlw.NewVersionWriter(out2, pr)
	default:
	}

	recwFn := getRecordWriterFunc(fm)
	if recwFn == nil {
		// We can still continue, because w.Record was already set above.
		log.Warn("No record writer impl for format", "format", fm)
	} else {
		w.Record = recwFn(out2, pr)
	}

	return w, out2, errOut2
}

// getRecordWriterFunc returns a func that creates a new output.RecordWriter
// for format f. If no matching format, nil is returned.
func getRecordWriterFunc(f format.Format) output.NewRecordWriterFunc {
	switch f {
	case format.Text:
		return tablew.NewRecordWriter
	case format.CSV:
		return csvw.NewCommaRecordWriter
	case format.TSV:
		return csvw.NewTabRecordWriter
	case format.JSON:
		return jsonw.NewStdRecordWriter
	case format.JSONA:
		return jsonw.NewArrayRecordWriter
	case format.JSONL:
		return jsonw.NewObjectRecordWriter
	case format.HTML:
		return htmlw.NewRecordWriter
	case format.Markdown:
		return markdownw.NewRecordWriter
	case format.XML:
		return xmlw.NewRecordWriter
	case format.XLSX:
		return xlsxw.NewRecordWriter
	case format.YAML:
		return yamlw.NewRecordWriter
	case format.Raw:
		return raww.NewRecordWriter
	default:
		return nil
	}
}

// outputters is a container for the various output writers.
// We need to refactor getPrinting to return a struct like this.
type outputters struct { //nolint:unused
	// outPr is the printing config for out.
	outPr *output.Printing

	// out is the output writer that should be used for stdout output.
	out io.Writer

	// ogOut is the original out, which probably was os.Stdin.
	// It's referenced here for special cases.
	ogOut io.Writer

	// errOutPr is the printing config for errOut.
	errOutPr *output.Printing

	// errOut is the output writer that should be used for stderr output.
	errOut io.Writer

	// ogErrOut is the original errOut, which probably was os.Stderr.
	// It's referenced here for special cases.
	ogErrOut io.Writer
}

// getPrinting returns a Printing instance and
// colorable or non-colorable writers. It is permissible
// for the cmd arg to be nil. The caller should use the returned
// io.Writer instances instead of the supplied writers, as they
// may be decorated for dealing with color, etc.
// The supplied opts must already have flags merged into it
// via getOptionsFromCmd.
//
// Be cautious making changes to getPrinting. This function must
// be absolutely bulletproof, as it's called by all commands, as well
// as by the error handling mechanism. So, be sure to always check
// for nil cmd, nil cmd.Context, etc.
//
// REVISIT: getPrinting should be refactored to return [*outputters].
func getPrinting(cmd *cobra.Command, clnup *cleanup.Cleanup, opts options.Options,
	out, errOut io.Writer,
) (pr *output.Printing, out2, errOut2 io.Writer) {
	pr = output.NewPrinting()

	pr.FormatDatetime = timez.FormatFunc(OptDatetimeFormat.Get(opts))
	pr.FormatDatetimeAsNumber = OptDatetimeFormatAsNumber.Get(opts)
	pr.FormatTime = timez.FormatFunc(OptTimeFormat.Get(opts))
	pr.FormatTimeAsNumber = OptTimeFormatAsNumber.Get(opts)
	pr.FormatDate = timez.FormatFunc(OptDateFormat.Get(opts))
	pr.FormatDateAsNumber = OptDateFormatAsNumber.Get(opts)

	pr.ExcelDatetimeFormat = xlsxw.OptDatetimeFormat.Get(opts)
	pr.ExcelDateFormat = xlsxw.OptDateFormat.Get(opts)
	pr.ExcelTimeFormat = xlsxw.OptTimeFormat.Get(opts)

	pr.Verbose = OptVerbose.Get(opts)
	pr.FlushThreshold = OptTuningFlushThreshold.Get(opts)
	pr.Compact = OptCompact.Get(opts)

	switch {
	case cmdFlagChanged(cmd, flag.Header):
		pr.ShowHeader, _ = cmd.Flags().GetBool(flag.Header)
	case cmdFlagChanged(cmd, flag.NoHeader):
		b, _ := cmd.Flags().GetBool(flag.NoHeader)
		pr.ShowHeader = !b
	case opts != nil:
		pr.ShowHeader = OptPrintHeader.Get(opts)
	}

	colorize := !OptMonochrome.Get(opts)

	if cmdFlagChanged(cmd, flag.Output) {
		// We're outputting to a file, thus no color.
		colorize = false
	}

	if !colorize {
		color.NoColor = true
		pr.EnableColor(false)
		out2 = out
		errOut2 = errOut

		if cmd != nil && cmd.Context() != nil && OptProgress.Get(opts) && isTerminal(errOut) {
			progColors := progress.DefaultColors()
			progColors.EnableColor(false)
			ctx := cmd.Context()
			renderDelay := OptProgressDelay.Get(opts)
			pb := progress.New(ctx, errOut2, renderDelay, progColors)
			clnup.Add(pb.Stop)
			// On first write to stdout, we remove the progress widget.
			out2 = ioz.NotifyOnceWriter(out2, pb.Stop)
			cmd.SetContext(progress.NewContext(ctx, pb))
		}

		return pr, out2, errOut2
	}

	// We do want to colorize
	if !isColorTerminal(out) {
		// But out can't be colorized.

		// FIXME: This disables colorization for both out and errOut, even
		// if errOut is a color terminal.
		//
		//  $ sq db dump > local.pg.dump
		//  sq: db dump: @ew/local/cloud: exit status 1: pg_dump: ...
		//
		// FIXME: e.g. above, the error message is not colorized, even
		// though it should be.

		color.NoColor = true
		pr.EnableColor(false)
		out2, errOut2 = out, errOut
		return pr, out2, errOut2
	}

	// out can be colorized.
	color.NoColor = false
	pr.EnableColor(true)
	out2 = colorable.NewColorable(out.(*os.File))

	// Check if we can colorize errOut
	if isColorTerminal(errOut) {
		errOut2 = colorable.NewColorable(errOut.(*os.File))
	} else {
		// errOut2 can't be colorized, but since we're colorizing
		// out, we'll apply the non-colorable filter to errOut.
		errOut2 = colorable.NewNonColorable(errOut)
	}

	if cmd != nil && cmd.Context() != nil && OptProgress.Get(opts) && isTerminal(errOut) {
		progColors := progress.DefaultColors()
		progColors.EnableColor(isColorTerminal(errOut))

		ctx := cmd.Context()
		renderDelay := OptProgressDelay.Get(opts)
		pb := progress.New(ctx, errOut2, renderDelay, progColors)
		clnup.Add(pb.Stop)

		// On first write to stdout, we remove the progress widget.
		out2 = ioz.NotifyOnceWriter(out2, pb.Stop)

		cmd.SetContext(progress.NewContext(ctx, pb))
	}

	return pr, out2, errOut2
}

func getFormat(cmd *cobra.Command, o options.Options) format.Format {
	var fm format.Format

	switch {
	case cmdFlagChanged(cmd, flag.TSV):
		fm = format.TSV
	case cmdFlagChanged(cmd, flag.CSV):
		fm = format.CSV
	case cmdFlagChanged(cmd, flag.XLSX):
		fm = format.XLSX
	case cmdFlagChanged(cmd, flag.XML):
		fm = format.XML
	case cmdFlagChanged(cmd, flag.Raw):
		fm = format.Raw
	case cmdFlagChanged(cmd, flag.HTML):
		fm = format.HTML
	case cmdFlagChanged(cmd, flag.Markdown):
		fm = format.Markdown
	case cmdFlagChanged(cmd, flag.Text):
		fm = format.Text
	case cmdFlagChanged(cmd, flag.JSONL):
		fm = format.JSONL
	case cmdFlagChanged(cmd, flag.JSONA):
		fm = format.JSONA
	case cmdFlagChanged(cmd, flag.JSON):
		fm = format.JSON
	case cmdFlagChanged(cmd, flag.YAML):
		fm = format.YAML
	default:
		// no format flag, use the config value
		fm = OptFormat.Get(o)
	}
	return fm
}

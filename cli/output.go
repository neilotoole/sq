package cli

import (
	"context"
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
	"github.com/neilotoole/sq/cli/output/erdimgw"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/output/htmlw"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/cli/output/markdownw"
	"github.com/neilotoole/sq/cli/output/mermaidw"
	"github.com/neilotoole/sq/cli/output/raww"
	"github.com/neilotoole/sq/cli/output/sqlw"
	"github.com/neilotoole/sq/cli/output/tablew"
	"github.com/neilotoole/sq/cli/output/xlsxw"
	"github.com/neilotoole/sq/cli/output/xmlw"
	"github.com/neilotoole/sq/cli/output/yamlw"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/debugz"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/scannerz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/termz"
	"github.com/neilotoole/sq/libsq/core/timez"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/files"
)

var (
	OptPrintHeader = options.NewBool(
		"header",
		nil,
		true,
		"Print header row",
		`Controls whether a header row is printed. This applies only to certain formats,
such as "text" or "csv".`,
		options.TagOutput,
	)

	OptFormat = format.NewOpt(
		"format",
		&options.Flag{Short: 'f'},
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

	// OptFormatDecimal controls how decimal values render in output formats that
	// distinguish a number from a string (JSON, YAML). "string" (the default)
	// emits a quoted string, precision-safe beyond float64 range; "number" emits
	// a bare number, convenient for jq-style consumers. See issue #846.
	OptFormatDecimal = options.NewString(
		"format.decimal",
		&options.Flag{Name: "format.decimal"},
		"string",
		func(s string) error {
			switch s {
			case "string", "number":
				return nil
			default:
				return errz.Errorf("option {format.decimal} allows only %q or %q", "string", "number")
			}
		},
		"Render decimal as string or number (JSON, YAML)",
		`Controls how decimal values render in output formats that distinguish a number
from a string (JSON and YAML). "string" (the default) emits a quoted string,
which is precision-safe for values beyond float64 range. "number" emits a bare
number, which is convenient for jq-style consumers but lossy on read for very
large values. XLSX always stores decimals adaptively and is unaffected; all-text
formats such as CSV are no-ops.

  format.decimal=string
  [{"total":"20100"}]
  format.decimal=number
  [{"total":20100}]
`,
		options.TagOutput,
	)

	OptErrorFormat = format.NewOpt(
		"error.format",
		nil,
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

	OptErrorStack = options.NewBool(
		"error.stack",
		&options.Flag{Short: 'E'},
		false,
		"Print error stack trace to stderr",
		`Print error stack trace to stderr. This only applies when error.format is
"text"; when error.format is "json", the stack trace is always printed.`,
		options.TagOutput,
	)

	// OptErrorFormatTextVerbose controls how much of an SLQ parse error is
	// shown in text error output. It is config-only (no CLI flag).
	OptErrorFormatTextVerbose = options.NewBool(
		"error.format.text.verbose",
		nil,
		true,
		"Show full multi-line text error reporting",
		`When true (default), syntax errors shown in "text" error format include the
full report: the offending span highlighted in the original query, plus any
"did you mean" suggestion. When false, only the first summary line is shown
(e.g. "sq: syntax error at line 1, col 10: unexpected 'mx'").

This only applies when error.format is "text"; "json" error output always
includes the full structured parse_error.`,
		options.TagOutput,
	)

	OptVerbose = options.NewBool(
		"verbose",
		&options.Flag{Short: 'v'},
		false,
		"Print verbose output",
		`Print verbose output.`,
		options.TagOutput,
	)

	OptMonochrome = options.NewBool(
		"monochrome",
		&options.Flag{Short: 'M'},
		false,
		"Don't print color output",
		`Don't print color output.`,
		options.TagOutput,
	)

	OptDebugTrackMemory = options.NewDuration(
		"debug.stats.frequency",
		nil,
		0,
		"Memory usage sampling interval.",
		`Memory usage sampling interval. If non-zero, peak memory usage is periodically
sampled, and reported on exit. If zero, memory usage sampling is disabled.`,
	)

	OptCompact = options.NewBool(
		"compact",
		&options.Flag{Short: 'c'},
		false,
		"Compact instead of pretty-printed output",
		`Compact instead of pretty-printed output.`,
		options.TagOutput,
	)

	// OptHTMLEmbedAssets controls whether sq inspect's HTML output inlines assets.
	OptHTMLEmbedAssets = options.NewBool(
		"format.html.embed-assets",
		nil,
		false,
		"Embed assets (Mermaid.js) in HTML output for offline use",
		`When true, sq inspect's HTML output inlines all assets — notably the
Mermaid.js library — so the document is fully self-contained and renders
offline. When false (default), Mermaid.js is loaded from a CDN, producing a
much smaller file that requires internet access to render the diagram.`,
		options.TagOutput,
	)

	timeLayoutsList = "Predefined values:\n" + scannerz.IndentLines(
		context.Background(),
		wordwrap.WrapString(strings.Join(timez.NamedLayouts(), ", "), 64),
		"  ",
	)

	OptDatetimeFormat = options.NewString(
		"format.datetime",
		nil,
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
		nil,
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
		nil,
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
		nil,
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
		nil,
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
		nil,
		true,
		"Render numeric time value as number instead of string",
		`Render numeric time value as number instead of string, if possible. If format.time
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

var OptProgress = options.NewBool(
	"progress",
	&options.Flag{
		Name:   "no-progress",
		Invert: true,
		Usage:  "Don't show progress bar",
	},
	true,
	"Show progress bar",
	`Show progress bar for long-running operations.`,
	options.TagOutput,
)

var OptProgressDelay = options.NewDuration(
	"progress.delay",
	nil,
	time.Second*2,
	"Progress bar render delay",
	`Delay before showing a progress bar.`,
	options.TagOutput,
)

var OptProgressMaxBars = options.NewInt(
	"progress.max-bars",
	nil,
	progress.DefaultMaxBars,
	"Max concurrent progress bars shown in terminal",
	`Limit the number of progress bars shown concurrently in the terminal. When the
threshold is reached, further progress bars are grouped into a single group bar.
If zero, no progress bar is rendered.`,

	options.TagOutput,
)

// newWriters returns an output.Writers instance configured per defaults and/or
// flags from ru.Cmd. The returned writers in [outputConfig] may differ from
// ru.Stdout / ru.Stderr (e.g. decorated to support colorization).
func newWriters(ru *run.Run, o options.Options) (w *output.Writers, outCfg *outputConfig) {
	cmd, fs, clnup := ru.Cmd, ru.Files, ru.Cleanup
	stdout, stderr := ru.Stdout, ru.Stderr

	// Invoke getFormat to see if the format was specified
	// via config or flag.
	fm := getFormat(cmd, o)
	outCfg = getOutputConfig(cmd, fs, clnup, fm, o, stdout, stderr)
	log := lg.From(cmd)

	// Package tablew has writer impls for each of the writer interfaces,
	// so we use its Writers as the baseline. Later we check the format
	// flags and set the various writer fields depending upon which
	// writers the format implements.
	w = &output.Writers{
		PrOut:        outCfg.outPr,
		PrErr:        outCfg.errOutPr,
		Record:       tablew.NewRecordWriter(outCfg.out, outCfg.outPr),
		RecordInsert: tablew.NewRecordInsertWriter(outCfg.out, outCfg.outPr),
		StmtExec:     tablew.NewStmtExecWriter(outCfg.out, outCfg.outPr),
		Metadata:     tablew.NewMetadataWriter(outCfg.out, outCfg.outPr),
		Source:       tablew.NewSourceWriter(outCfg.out, outCfg.outPr),
		Ping:         tablew.NewPingWriter(outCfg.out, outCfg.outPr),
		Error: tablew.NewErrorWriter(outCfg.errOut, outCfg.errOutPr,
			OptErrorStack.Get(o), OptErrorFormatTextVerbose.Get(o)),
		Version: tablew.NewVersionWriter(outCfg.out, outCfg.outPr),
		Config:  tablew.NewConfigWriter(outCfg.out, outCfg.outPr),
		Keyring: tablew.NewKeyringWriter(outCfg.out, outCfg.outPr),
		SQL:     sqlw.NewTextWriter(outCfg.out, outCfg.outPr),
	}

	if OptErrorFormat.Get(o) == format.JSON {
		// This logic works because the only supported values are text and json.
		w.Error = jsonw.NewErrorWriter(log, outCfg.errOut, outCfg.errOutPr)
	}

	//nolint:exhaustive
	switch fm {
	case format.JSON:
		w.RecordInsert = jsonw.NewRecordInsertWriter(outCfg.out, outCfg.outPr)
		w.StmtExec = jsonw.NewStmtExecWriter(outCfg.out, outCfg.outPr)
		w.Metadata = jsonw.NewMetadataWriter(outCfg.out, outCfg.outPr)
		w.Source = jsonw.NewSourceWriter(outCfg.out, outCfg.outPr)
		w.Version = jsonw.NewVersionWriter(outCfg.out, outCfg.outPr)
		w.Ping = jsonw.NewPingWriter(outCfg.out, outCfg.outPr)
		w.Config = jsonw.NewConfigWriter(outCfg.out, outCfg.outPr)
		w.Keyring = jsonw.NewKeyringWriter(outCfg.out, outCfg.outPr)
		w.SQL = sqlw.NewJSONWriter(outCfg.out, outCfg.outPr)

	case format.JSONL:
		w.SQL = sqlw.NewJSONLWriter(outCfg.out, outCfg.outPr)

	case format.Text:
		// Don't delete this case, it's actually needed due to
		// the slightly odd logic that determines format.

	case format.TSV:
		w.Ping = csvw.NewPingWriter(outCfg.out, csvw.Tab)

	case format.CSV:
		w.Ping = csvw.NewPingWriter(outCfg.out, csvw.Comma)

	case format.YAML:
		w.Config = yamlw.NewConfigWriter(outCfg.out, outCfg.outPr)
		w.Metadata = yamlw.NewMetadataWriter(outCfg.out, outCfg.outPr)
		w.Source = yamlw.NewSourceWriter(outCfg.out, outCfg.outPr)
		w.Version = yamlw.NewVersionWriter(outCfg.out, outCfg.outPr)
		w.SQL = sqlw.NewYAMLWriter(outCfg.out, outCfg.outPr)

	case format.Markdown:
		w.Metadata = markdownw.NewMetadataWriter(outCfg.out, outCfg.outPr)

	case format.HTML:
		w.Metadata = htmlw.NewMetadataWriter(outCfg.out, outCfg.outPr, OptHTMLEmbedAssets.Get(o))

	case format.MermaidERD:
		w.Metadata = mermaidw.NewMetadataWriter(outCfg.out, outCfg.outPr)

	case format.PNGERD:
		w.Metadata = erdimgw.NewPNGMetadataWriter(outCfg.out, outCfg.outPr)

	case format.SVGERD:
		w.Metadata = erdimgw.NewSVGMetadataWriter(outCfg.out, outCfg.outPr)
	default:
	}

	recwFn := getRecordWriterFunc(fm)
	if recwFn == nil {
		// We can still continue, because w.Record was already set above.
		log.Warn("No record writer impl for format", "format", fm)
	} else {
		w.Record = recwFn(outCfg.out, outCfg.outPr)
	}

	if cmd != nil {
		// Decorate the writers that print source locations so that the
		// --expand flag is honored centrally, in the writer layer, much
		// as Printing.Redact enforces redaction once for every writer.
		// (Redaction is a Printing field the format writers consult;
		// expansion is a cli-side decorator instead, because it performs
		// fallible resolver I/O that shouldn't be duplicated into every
		// writer impl. See expand_writer.go for that rationale.) Any
		// command that prints a location gets --expand for free; the
		// decorators no-op when the flag is unset.
		w.Source = &expandSourceWriter{w: w.Source, expander: expander{cmd: cmd, ru: ru}}
		w.Ping = &expandPingWriter{w: w.Ping, expander: expander{cmd: cmd, ru: ru}}
		w.Metadata = &expandMetadataWriter{w: w.Metadata, expander: expander{cmd: cmd, ru: ru}}
	}

	return w, outCfg
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
	case format.MermaidERD, format.PNGERD, format.SVGERD:
		// mermaid-erd, png-erd, and svg-erd are metadata-only (sq inspect)
		// ERD formats; they have no record writer, so callers fall back to
		// text for record output.
		return nil
	default:
		return nil
	}
}

// outputConfig is a container for the various output writers.
type outputConfig struct {
	// outPr is the printing config for out.
	outPr *output.Printing

	// out is the output writer that should be used for stdout output.
	out io.Writer

	// stdout is the original stdout, which probably was os.Stdin.
	// It's referenced here for special cases.
	stdout io.Writer

	// errOutPr is the printing config for errOut.
	errOutPr *output.Printing

	// errOut is the output writer that should be used for stderr output.
	errOut io.Writer

	// stderr is the original errOut, which probably was os.Stderr.
	// It's referenced here for special cases.
	stderr io.Writer
}

// getOutputConfig returns the configured output writers for cmd. Generally
// speaking, the caller should use the outputConfig.out and outputConfig.errOut
// writers for program output, as they are decorated appropriately for dealing
// with colorization, progress bars, etc. In very rare cases, such as calling
// out to an external program (e.g. pg_dump), the original outputConfig.stdout
// and outputConfig.stderr may be used.
//
// The supplied opts must already have flags merged into it via getOptionsFromCmd.
//
// If the progress bar is enabled and possible (stderr is TTY etc.), then cmd's
// context is decorated with via [progress.NewContext].
//
// Be VERY cautious about making changes to getOutputConfig. This function must
// be absolutely bulletproof, as it's called by all commands, as well as by the
// error handling mechanism. So, be sure to always check for nil: any of the
// args could be nil, or their fields could be nil. Check EVERYTHING for nil.
//
// The returned outputConfig and all of its fields are guaranteed to be non-nil.
//
// See also: [OptMonochrome], [OptProgress], newWriters.
func getOutputConfig(cmd *cobra.Command, fs *files.Files, clnup *cleanup.Cleanup,
	fm format.Format, o options.Options, stdout, stderr io.Writer,
) (outCfg *outputConfig) {
	if o == nil {
		o = options.Options{}
	}

	var ctx context.Context
	if cmd != nil {
		ctx = cmd.Context()
	}

	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	outCfg = &outputConfig{stdout: stdout, stderr: stderr}

	pr := output.NewPrinting()
	if fs != nil {
		pr.NewBufferFn = fs.NewBuffer
	}

	pr.FormatDatetime = timez.FormatFunc(OptDatetimeFormat.Get(o))
	pr.FormatDatetimeAsNumber = OptDatetimeFormatAsNumber.Get(o)
	pr.FormatTime = timez.FormatFunc(OptTimeFormat.Get(o))
	pr.FormatTimeAsNumber = OptTimeFormatAsNumber.Get(o)
	pr.FormatDate = timez.FormatFunc(OptDateFormat.Get(o))
	pr.FormatDateAsNumber = OptDateFormatAsNumber.Get(o)
	pr.DecimalAsNumber = OptFormatDecimal.Get(o) == "number"

	pr.ExcelDatetimeFormat = xlsxw.OptDatetimeFormat.Get(o)
	pr.ExcelDateFormat = xlsxw.OptDateFormat.Get(o)
	pr.ExcelTimeFormat = xlsxw.OptTimeFormat.Get(o)

	pr.Verbose = OptVerbose.Get(o)
	pr.FlushThreshold = int(tuning.OptFlushThreshold.Get(o).Bytes()) //nolint:gosec // ignore overflow concern
	pr.Compact = OptCompact.Get(o)
	pr.Redact = !secret.OptSecretsReveal.Get(o)

	switch {
	case cmdFlagChanged(cmd, flag.Header):
		pr.ShowHeader, _ = cmd.Flags().GetBool(flag.Header)
	case cmdFlagChanged(cmd, flag.NoHeader):
		b, _ := cmd.Flags().GetBool(flag.NoHeader)
		pr.ShowHeader = !b
	case o != nil:
		pr.ShowHeader = OptPrintHeader.Get(o)
	}

	var (
		prog       *progress.Progress
		noProg     = !OptProgress.Get(o) || OptProgressMaxBars.Get(o) < 1
		forceProg  = debugz.OptProgressDebugForce.Get(o)
		progColors = progress.DefaultColors()
		monochrome = OptMonochrome.Get(o)
	)

	if forceProg {
		noProg = false
	}

	if monochrome {
		color.NoColor = true
		pr.EnableColor(false)
		progColors.EnableColor(false)
	} else {
		color.NoColor = false
		pr.EnableColor(true)
		progColors.EnableColor(true)
	}

	outCfg.outPr = pr.Clone()
	outCfg.errOutPr = pr.Clone()
	pr = nil //nolint:wastedassign // Make sure we don't accidentally use pr again

	switch {
	case forceProg, termz.IsColorTerminal(stderr) && !monochrome:
		// Either forceProg is true, or stderr is a color terminal and we're
		// colorizing, thus we enable progress if allowed and if ctx is non-nil.
		var colorize bool
		if f, ok := stderr.(*os.File); ok {
			outCfg.errOut = colorable.NewColorable(f)
			colorize = true
		} else {
			outCfg.errOut = colorable.NewNonColorable(stderr)
		}

		outCfg.errOutPr.EnableColor(colorize)
		if ctx != nil && (forceProg || !noProg) {
			progColors.EnableColor(colorize)
			prog = progress.New(ctx, outCfg.errOut, OptProgressMaxBars.Get(o), OptProgressDelay.Get(o), progColors)
		}
	case termz.IsTerminal(stderr):
		// stderr is a terminal, and won't have color output, but we still enable
		// progress, if allowed and ctx is non-nil.
		//
		// But... slightly weirdly, we still need to wrap stderr in a colorable, or
		// else the progress bar won't render correctly. But it's not a problem,
		// because we'll just disable the colors directly.
		outCfg.errOut = colorable.NewColorable(stderr.(*os.File))
		outCfg.errOutPr.EnableColor(false)
		if ctx != nil && !noProg {
			progColors.EnableColor(false)
			prog = progress.New(ctx, outCfg.errOut, OptProgressMaxBars.Get(o), OptProgressDelay.Get(o), progColors)
		}
	default:
		// stderr is a not a terminal at all. No color, no progress.
		outCfg.errOut = colorable.NewNonColorable(stderr)
		outCfg.errOutPr.EnableColor(false)
		progColors.EnableColor(false)
		prog = nil // Set to nil just to be explicit.
	}

	switch {
	case cmdFlagChanged(cmd, flag.FileOutput) || fm == format.Raw || fm == format.XLSX ||
		fm == format.PNGERD || fm == format.SVGERD:
		// For file, raw, XLSX, or ERD-image output, we don't decorate stdout
		// with any colorable decorator. XLSX and png-erd are binary and must
		// not be modified; svg-erd is plain image markup that wants no ANSI
		// color processing either.
		outCfg.out = stdout
		outCfg.outPr.EnableColor(false)
	case termz.IsColorTerminal(stdout) && !monochrome:
		// stdout is a color terminal and we're colorizing.
		outCfg.out = colorable.NewColorable(stdout.(*os.File))
		outCfg.outPr.EnableColor(true)
	case termz.IsTerminal(stderr):
		// stdout is a terminal, but won't be colorized.
		outCfg.out = colorable.NewNonColorable(stdout)
		outCfg.outPr.EnableColor(false)
	default:
		// stdout is a not a terminal at all. No color.
		outCfg.out = colorable.NewNonColorable(stdout)
		outCfg.outPr.EnableColor(false)
	}

	if !noProg && prog != nil && cmd != nil && ctx != nil {
		// The progress bar is enabled.

		// Be sure to stop the progress bar eventually.
		clnup.Add(prog.Stop)

		// Also, hide the progress bar as soon as bytes are written
		// to out, because we don't want the progress bar to
		// corrupt the terminal output.
		outCfg.out = prog.HideOnWriter(outCfg.out)
		cmd.SetContext(progress.NewContext(ctx, prog))
	}

	return outCfg
}

func getFormat(cmd *cobra.Command, o options.Options) format.Format {
	var fm format.Format

	switch {
	case cmd == nil:
		fm = OptFormat.Get(o)
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

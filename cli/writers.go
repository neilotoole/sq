package cli

import (
	"io"
	"os"

	"github.com/neilotoole/sq/libsq/core/timefmt"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/cli/output/format"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/csvw"
	"github.com/neilotoole/sq/cli/output/htmlw"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/cli/output/markdownw"
	"github.com/neilotoole/sq/cli/output/raww"
	"github.com/neilotoole/sq/cli/output/tablew"
	"github.com/neilotoole/sq/cli/output/xlsxw"
	"github.com/neilotoole/sq/cli/output/xmlw"
	"github.com/neilotoole/sq/cli/output/yamlw"
	"github.com/spf13/cobra"
)

var (
	OptPrintHeader = options.NewBool(
		"header",
		0,
		true,
		"Print header row",
		`Controls whether a header row is printed. This applies only
to certain formats, such as "text" or "csv".`,
		"format",
	)
	OptFormat = NewFormatOpt(
		"format",
		format.Text,
		"Output format",
		`Specify the output format. Some formats are only implemented
for a subset of sq's commands. If the specified format is not available for
a particular command, sq falls back to 'text'. Available formats:
  text, csv, tsv, xlsx,
  json, jsona, jsonl,
  markdown, html, xml, yaml, raw`,
	)

	OptVerbose = options.NewBool(
		"verbose",
		'v',
		false,
		"Print verbose output",
		`Print verbose output.`,
		"format",
	)

	OptMonochrome = options.NewBool(
		"monochrome",
		'M',
		false,
		"Don't print color output",
		`Don't print color output.`,
		"format",
	)

	OptCompact = options.NewBool(
		"compact",
		'c',
		false,
		"Compact instead of pretty-printed output",
		`Compact instead of pretty-printed output.`,
		"format",
	)

	OptTuningFlushThreshold = options.NewInt(
		"tuning.flush-threshold",
		0,
		1000,
		"Output writer buffer flush threshold in bytes",
		`Size in bytes after which output writers should flush any internal buffer.
Generally, it is not necessary to fiddle this knob.`,
	)

	OptDatetimeFormat = options.NewString(
		"format.datetime",
		0,
		"RFC3339",
		"Timestamp format: constant such as RFC3339 or a strftime format",
		`Timestamp format. This can be one of several predefined constants such
as "RFC3339" or "Unix", or a strftime format such as "%Y-%m-%d %H:%M:%S".
Available predefined values are:
  ANSIC, DateOnly, DateTime, TimeOnly, ISO8601, RFC1123, RFC1123Z,
  RFC3339, RFC3339Milli, RFC3339MilliZ, RFC3339Nano, RFC3339Z,
  RFC822, RFC8222Z, RFC850, Unix, UnixDate, UnixMicro, UnixMilli, UnixNano`,
	)

	OptDateFormat = options.NewString(
		"format.date",
		0,
		"DateOnly",
		"Date format: constant such as DateOnly or a strftime format",
		`Date format. This can be one of several predefined constants such
as "DateOnly" or "Unix", or a strftime format such as "%Y-%m-%d".
Available predefined values are:
  ANSIC, DateOnly, DateTime, TimeOnly, ISO8601, RFC1123, RFC1123Z,
  RFC3339, RFC3339Milli, RFC3339MilliZ, RFC3339Nano, RFC3339Z,
  RFC822, RFC8222Z, RFC850, Unix, UnixDate, UnixMicro, UnixMilli, UnixNano`,
	)
	OptTimeFormat = options.NewString(
		"format.time",
		0,
		"TimeOnly",
		"Time format: constant such as TimeOnly or a strftime format",
		`Time format. This can be one of several predefined constants such
as "TimeOnly" or "Unix", or a strftime format such as "%Y-%m-%d".
Available predefined values are:
  ANSIC, DateOnly, DateTime, TimeOnly, ISO8601, RFC1123, RFC1123Z,
  RFC3339, RFC3339Milli, RFC3339MilliZ, RFC3339Nano, RFC3339Z,
  RFC822, RFC8222Z, RFC850, Unix, UnixDate, UnixMicro, UnixMilli, UnixNano`,
	)
)

// writers is a container for the various output writers.
type writers struct {
	pr *output.Printing

	recordw  output.RecordWriter
	metaw    output.MetadataWriter
	srcw     output.SourceWriter
	errw     output.ErrorWriter
	pingw    output.PingWriter
	versionw output.VersionWriter
	configw  output.ConfigWriter
}

// newWriters returns a writers instance configured per defaults and/or
// flags from cmd. The returned out2/errOut2 values may differ
// from the out/errOut args (e.g. decorated to support colorization).
func newWriters(cmd *cobra.Command, opts options.Options, out, errOut io.Writer,
) (w *writers, out2, errOut2 io.Writer) {
	var pr *output.Printing
	pr, out2, errOut2 = getPrinting(cmd, opts, out, errOut)
	log := logFrom(cmd)

	// Package tablew has writer impls for each of the writer interfaces,
	// so we use its writers as the baseline. Later we check the format
	// flags and set the various writer fields depending upon which
	// writers the format implements.
	w = &writers{
		pr:       pr,
		recordw:  tablew.NewRecordWriter(out2, pr),
		metaw:    tablew.NewMetadataWriter(out2, pr),
		srcw:     tablew.NewSourceWriter(out2, pr),
		pingw:    tablew.NewPingWriter(out2, pr),
		errw:     tablew.NewErrorWriter(errOut2, pr),
		versionw: tablew.NewVersionWriter(out2, pr),
		configw:  tablew.NewConfigWriter(out2, pr),
	}

	// Invoke getFormat to see if the format was specified
	// via config or flag.
	fm := getFormat(cmd, opts) // FIXME: is this still needed, or use standard opts mechanism?

	//nolint:exhaustive
	switch fm {
	default:
		// No format specified, use JSON
		w.recordw = jsonw.NewStdRecordWriter(out2, pr)
		w.metaw = jsonw.NewMetadataWriter(out2, pr)
		w.srcw = jsonw.NewSourceWriter(out2, pr)
		w.errw = jsonw.NewErrorWriter(log, errOut2, pr)
		w.versionw = jsonw.NewVersionWriter(out2, pr)
		w.pingw = jsonw.NewPingWriter(out2, pr)
		w.configw = jsonw.NewConfigWriter(out2, pr)

	case format.Text:
	// Table is the base format, already set above, no need to do anything.

	case format.TSV:
		w.recordw = csvw.NewRecordWriter(out2, pr, csvw.Tab)
		w.pingw = csvw.NewPingWriter(out2, csvw.Tab)

	case format.CSV:
		w.recordw = csvw.NewRecordWriter(out2, pr, csvw.Comma)
		w.pingw = csvw.NewPingWriter(out2, csvw.Comma)

	case format.XML:
		w.recordw = xmlw.NewRecordWriter(out2, pr)

	case format.XLSX:
		w.recordw = xlsxw.NewRecordWriter(out2, pr.ShowHeader)

	case format.Raw:
		w.recordw = raww.NewRecordWriter(out2, pr)

	case format.HTML:
		w.recordw = htmlw.NewRecordWriter(out2, pr)

	case format.Markdown:
		w.recordw = markdownw.NewRecordWriter(out2, pr)

	case format.JSONA:
		w.recordw = jsonw.NewArrayRecordWriter(out2, pr)

	case format.JSONL:
		w.recordw = jsonw.NewObjectRecordWriter(out2, pr)

	case format.YAML:
		w.configw = yamlw.NewConfigWriter(out2, pr)
		w.metaw = yamlw.NewMetadataWriter(out2, pr)
		w.srcw = yamlw.NewSourceWriter(out2, pr)
	}

	return w, out2, errOut2
}

// getPrinting returns a Printing instance and
// colorable or non-colorable writers. It is permissible
// for the cmd arg to be nil. The caller should use the returned
// io.Writer instances instead of the supplied writers, as they
// may be decorated for dealing with color, etc.
// The supplied opts must already have flags merged into it
// via getOptionsFromCmd.
func getPrinting(cmd *cobra.Command, opts options.Options, out, errOut io.Writer,
) (pr *output.Printing, out2, errOut2 io.Writer) {
	pr = output.NewPrinting()

	timestampLayout := OptDatetimeFormat.Get(opts)
	pr.FormatDatetime = timefmt.FormatFunc(timestampLayout)

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
		return pr, out2, errOut2
	}

	// We do want to colorize
	if !isColorTerminal(out) {
		// But out can't be colorized.
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

	logFrom(cmd).Debug("Constructed output.Printing", lga.Val, pr)

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

var _ options.Opt = FormatOpt{}

// NewFormatOpt returns a new FormatOpt instance.
func NewFormatOpt(key string, defaultVal format.Format, usage, doc string) FormatOpt {
	opt := options.NewBaseOpt(key, 0, usage, doc)
	return FormatOpt{BaseOpt: opt, defaultVal: defaultVal}
}

// FormatOpt is an options.Opt for format.Format.
type FormatOpt struct {
	options.BaseOpt
	defaultVal format.Format
}

// Process implements options.Processor. It converts matching
// string values in o into format.Format. If no match found,
// the input arg is returned unchanged. Otherwise, a clone is
// returned.
func (op FormatOpt) Process(o options.Options) (options.Options, error) {
	if o == nil {
		return nil, nil
	}

	key := op.Key()
	v, ok := o[key]
	if !ok || v == nil {
		return o, nil
	}

	// v should be a string
	switch v := v.(type) {
	case string:
		// continue below
	case format.Format:
		return o, nil
	default:
		return nil, errz.Errorf("option {%s} should be {%T} or {%T} but got {%T}: %v",
			key, format.Format(""), "", v, v)
	}

	var s string
	s, ok = v.(string)
	if !ok {
		return nil, errz.Errorf("option {%s} should be {%T} but got {%T}: %v",
			key, s, v, v)
	}

	var f format.Format
	if err := f.UnmarshalText([]byte(s)); err != nil {
		return nil, errz.Wrapf(err, "option {%s} is not a valid {%T}", key, f)
	}

	o = o.Clone()
	o[key] = f
	return o, nil
}

// GetAny implements options.Opt.
func (op FormatOpt) GetAny(o options.Options) any {
	return op.Get(o)
}

// Get returns op's value in o. If o is nil, or no value
// is set, op's default value is returned.
func (op FormatOpt) Get(o options.Options) format.Format {
	if o == nil {
		return op.defaultVal
	}

	v, ok := o[op.Key()]
	if !ok {
		return op.defaultVal
	}

	var f format.Format
	f, ok = v.(format.Format)
	if !ok {
		return op.defaultVal
	}

	return f
}

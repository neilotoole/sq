package cli

import (
	"io"
	"os"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/neilotoole/sq/cli/config"
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
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/spf13/cobra"
)

// writers is a container for the various output writers.
type writers struct {
	fm *output.Formatting

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
func newWriters(cmd *cobra.Command, opts config.Options, out,
	errOut io.Writer,
) (w *writers, out2, errOut2 io.Writer) {
	var fm *output.Formatting
	fm, out2, errOut2 = getWriterFormatting(cmd, &opts, out, errOut)
	log := lg.FromContext(cmd.Context())

	// Package tablew has writer impls for each of the writer interfaces,
	// so we use its writers as the baseline. Later we check the format
	// flags and set the various writer fields depending upon which
	// writers the format implements.
	w = &writers{
		fm:       fm,
		recordw:  tablew.NewRecordWriter(out2, fm),
		metaw:    tablew.NewMetadataWriter(out2, fm),
		srcw:     tablew.NewSourceWriter(out2, fm),
		pingw:    tablew.NewPingWriter(out2, fm),
		errw:     tablew.NewErrorWriter(errOut2, fm),
		versionw: tablew.NewVersionWriter(out2, fm),
		configw:  tablew.NewConfigWriter(out2, fm),
	}

	// Invoke getFormat to see if the format was specified
	// via config or flag.
	format := getFormat(cmd, opts)

	switch format { //nolint:exhaustive
	default:
		// No format specified, use JSON
		w.recordw = jsonw.NewStdRecordWriter(out2, fm)
		w.metaw = jsonw.NewMetadataWriter(out2, fm)
		w.srcw = jsonw.NewSourceWriter(out2, fm)
		w.errw = jsonw.NewErrorWriter(log, errOut2, fm)
		w.versionw = jsonw.NewVersionWriter(out2, fm)
		w.pingw = jsonw.NewPingWriter(out2, fm)
		w.configw = jsonw.NewConfigWriter(out2, fm)

	case config.FormatTable:
	// Table is the base format, already set above, no need to do anything.

	case config.FormatTSV:
		w.recordw = csvw.NewRecordWriter(out2, fm.ShowHeader, csvw.Tab)
		w.pingw = csvw.NewPingWriter(out2, csvw.Tab)

	case config.FormatCSV:
		w.recordw = csvw.NewRecordWriter(out2, fm.ShowHeader, csvw.Comma)
		w.pingw = csvw.NewPingWriter(out2, csvw.Comma)

	case config.FormatXML:
		w.recordw = xmlw.NewRecordWriter(out2, fm)

	case config.FormatXLSX:
		w.recordw = xlsxw.NewRecordWriter(out2, fm.ShowHeader)

	case config.FormatRaw:
		w.recordw = raww.NewRecordWriter(out2)

	case config.FormatHTML:
		w.recordw = htmlw.NewRecordWriter(out2)

	case config.FormatMarkdown:
		w.recordw = markdownw.NewRecordWriter(out2)

	case config.FormatJSONA:
		w.recordw = jsonw.NewArrayRecordWriter(out2, fm)

	case config.FormatJSONL:
		w.recordw = jsonw.NewObjectRecordWriter(out2, fm)

	case config.FormatYAML:
		w.configw = yamlw.NewConfigWriter(out2, fm)
		w.metaw = yamlw.NewMetadataWriter(out2, fm)
	}

	return w, out2, errOut2
}

// getWriterFormatting returns a Formatting instance and
// colorable or non-colorable writers. It is permissible
// for the cmd arg to be nil.
func getWriterFormatting(cmd *cobra.Command, opts *config.Options,
	out, errOut io.Writer,
) (fm *output.Formatting, out2, errOut2 io.Writer) {
	fm = output.NewFormatting()

	if cmdFlagChanged(cmd, flag.Pretty) {
		fm.Pretty, _ = cmd.Flags().GetBool(flag.Pretty)
	}

	if cmdFlagChanged(cmd, flag.Verbose) {
		fm.Verbose, _ = cmd.Flags().GetBool(flag.Verbose)
	}

	if cmdFlagChanged(cmd, flag.Header) {
		fm.ShowHeader, _ = cmd.Flags().GetBool(flag.Header)
	} else if opts != nil {
		fm.ShowHeader = opts.Header
	}

	// TODO: Should get this default value from config
	colorize := true

	if cmdFlagChanged(cmd, flag.Output) {
		// We're outputting to a file, thus no color.
		colorize = false
	} else if cmdFlagChanged(cmd, flag.Monochrome) {
		if mono, _ := cmd.Flags().GetBool(flag.Monochrome); mono {
			colorize = false
		}
	}

	if !colorize {
		color.NoColor = true // TODO: shouldn't rely on package-level var
		fm.EnableColor(false)
		out2 = out
		errOut2 = errOut
		return fm, out2, errOut2
	}

	// We do want to colorize
	if !isColorTerminal(out) {
		// But out can't be colorized.
		color.NoColor = true
		fm.EnableColor(false)
		out2, errOut2 = out, errOut
		return fm, out2, errOut2
	}

	// out can be colorized.
	color.NoColor = false
	fm.EnableColor(true)
	out2 = colorable.NewColorable(out.(*os.File))

	// Check if we can colorize errOut
	if isColorTerminal(errOut) {
		errOut2 = colorable.NewColorable(errOut.(*os.File))
	} else {
		// errOut2 can't be colorized, but since we're colorizing
		// out, we'll apply the non-colorable filter to errOut.
		errOut2 = colorable.NewNonColorable(errOut)
	}

	return fm, out2, errOut2
}

func getFormat(cmd *cobra.Command, defaults config.Options) config.Format {
	var format config.Format

	switch {
	// cascade through the format flags in low-to-high order of precedence.
	case cmdFlagChanged(cmd, flag.TSV):
		format = config.FormatTSV
	case cmdFlagChanged(cmd, flag.CSV):
		format = config.FormatCSV
	case cmdFlagChanged(cmd, flag.XLSX):
		format = config.FormatXLSX
	case cmdFlagChanged(cmd, flag.XML):
		format = config.FormatXML
	case cmdFlagChanged(cmd, flag.Raw):
		format = config.FormatRaw
	case cmdFlagChanged(cmd, flag.HTML):
		format = config.FormatHTML
	case cmdFlagChanged(cmd, flag.Markdown):
		format = config.FormatMarkdown
	case cmdFlagChanged(cmd, flag.Table):
		format = config.FormatTable
	case cmdFlagChanged(cmd, flag.JSONL):
		format = config.FormatJSONL
	case cmdFlagChanged(cmd, flag.JSONA):
		format = config.FormatJSONA
	case cmdFlagChanged(cmd, flag.JSON):
		format = config.FormatJSON
	case cmdFlagChanged(cmd, flag.YAML):
		format = config.FormatYAML
	default:
		// no format flag, use the config value
		format = defaults.Format
	}
	return format
}

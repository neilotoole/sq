package cmd

import (
	"github.com/neilotoole/sq/cmd/config"
	"github.com/neilotoole/sq/cmd/out"
	"github.com/neilotoole/sq/cmd/out/raw"
	"github.com/neilotoole/sq/cmd/out/table"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/drvr/sqlh"
	"github.com/spf13/cobra"

	"sync"

	"runtime"

	"github.com/labstack/gommon/color"
	"github.com/neilotoole/sq/cmd/out/csv"
	"github.com/neilotoole/sq/cmd/out/json"
	"github.com/neilotoole/sq/cmd/out/xlsx"
	"github.com/neilotoole/sq/cmd/out/xml"
)

// Writer is the interface that integrates all application output. The xyzWriter
// interfaces are separated out because not every format can or should support
// every application output (e.g. RawWriter.Help() is not useful).
type Writer interface {
	out.RecordWriter
	out.MetadataWriter
	out.SourceWriter
	out.ErrorWriter
	out.HelpWriter
}

// wi caches the writer instance (as returned by getWriter).
var wi *writer
var wiMu sync.Mutex

// writer implements cmd.Writer interface.
type writer struct {
	cmd   *cobra.Command
	recw  out.RecordWriter
	metaw out.MetadataWriter
	srcw  out.SourceWriter
	errw  out.ErrorWriter
	helpw out.HelpWriter
}

func (w *writer) Records(records []*sqlh.Record) error {
	return w.recw.Records(records)
}
func (w *writer) Close() error {
	return w.recw.Close()
}
func (w *writer) Metadata(meta *drvr.SourceMetadata) error {
	return w.metaw.Metadata(meta)
}
func (w *writer) SourceSet(srcs *drvr.SourceSet, active *drvr.Source) error {
	return w.srcw.SourceSet(srcs, active)
}
func (w *writer) Source(src *drvr.Source) error {
	return w.srcw.Source(src)
}
func (w *writer) Error(err error) {
	w.errw.Error(err)
}
func (w *writer) Help(help string) error {
	return w.helpw.Help(help)
}

// getWriter returns a cmd.Writer as configured by the flags on cmd. It is permissible
// for cmd to be nil, in which case a default Writer is returned.
func getWriter(cmd *cobra.Command) Writer {

	wiMu.Lock()
	defer wiMu.Unlock()
	if wi != nil {
		return wi
	}

	if runtime.GOOS == "windows" {
		// TODO: at some point need to look into handling windows color support
		color.Disable()
	}

	if cmd == nil {
		// this shouldn't happen, but let's play it safe
		tblw := table.NewWriter(cfg.Options.Header)
		wi = &writer{cmd: cmd, recw: tblw, metaw: tblw, srcw: tblw, errw: tblw, helpw: tblw}
		return wi
	}

	// we need to determine --header here because the writer/format constructor
	// functions (e.g. table.NewWriter()) currently require it.
	hasHeader := false
	switch {
	case cmdFlagChanged(cmd, FlagHeader):
		hasHeader = true
	case cmdFlagChanged(cmd, FlagNoHeader):
		hasHeader = false
	default:
		// get the default --header value from config
		hasHeader = cfg.Options.Header
	}

	// table.NewWriter implements all sq's writer interfaces, so we set
	// that as default. Later we check the format flags and set the
	// various wi fields depending upon what functionality the format
	// implements.
	tblw := table.NewWriter(hasHeader)
	w := &writer{cmd: cmd, recw: tblw, metaw: tblw, srcw: tblw, errw: tblw, helpw: tblw}

	var format config.Format

	switch {
	// cascade through the format flags in low-to-high order of precedence.
	case cmdFlagChanged(cmd, FlagTSV):
		format = config.FormatTSV
	case cmdFlagChanged(cmd, FlagCSV):
		format = config.FormatCSV
	case cmdFlagChanged(cmd, FlagXLSX):
		format = config.FormatXLSX
	case cmdFlagChanged(cmd, FlagXML):
		format = config.FormatXML
	case cmdFlagChanged(cmd, FlagRaw):
		format = config.FormatRaw
	case cmdFlagChanged(cmd, FlagTable):
		format = config.FormatTable
	case cmdFlagChanged(cmd, FlagJSON):
		format = config.FormatJSON
	default:
		// no format flag, use the config value (which itself defaults to JSON)
		format = cfg.Options.Format
	}

	switch format {
	case config.FormatTSV:
		w.recw = csv.NewWriter(hasHeader, '\t')
	case config.FormatCSV:
		w.recw = csv.NewWriter(hasHeader, ',')
	case config.FormatXML:
		w.recw = xml.NewWriter()
	case config.FormatXLSX:
		w.recw = xlsx.NewWriter(hasHeader)
	case config.FormatRaw:
		w.recw = raw.NewWriter()
	case config.FormatTable:
		tw := table.NewWriter(hasHeader)
		w.recw = tw
		w.metaw = tw
	default:
		jw := json.NewWriter()
		w.recw = jw
		w.metaw = jw
	}

	wi = w
	return wi
}

// cmdFlagChanged returns true if cmd has the named flag and it has been changed.
func cmdFlagChanged(cmd *cobra.Command, name string) bool {
	flag := cmd.Flag(name)
	if flag == nil {
		return false
	}

	return flag.Changed
}

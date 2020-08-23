// Package xmlw implements output writers for XML.
package xmlw

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/fatih/color"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/cli/output"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// recordWriter implements output.RecordWriter.
type recordWriter struct {
	out io.Writer
	fm  *output.Formatting

	recMeta sqlz.RecordMeta

	// outBuf holds output prior to flushing.
	outBuf *bytes.Buffer

	// recWritten indicates that at least one record has been written
	// to outBuf.
	recsWritten bool

	elemColor *color.Color

	tplRecStart   string
	tplRecEnd     string
	tplFieldStart []string
	tplFieldEnd   []string

	fieldPrintFns []func(w io.Writer, a ...interface{})
}

const (
	decl            = "<?xml version=\"1.0\"?>"
	recsElementName = "records"
	recElemName     = "record"
)

// NewRecordWriter returns an output.RecordWriter instance for XML.
func NewRecordWriter(out io.Writer, fm *output.Formatting) output.RecordWriter {
	return &recordWriter{out: out, fm: fm, elemColor: fm.Key.Add(color.Faint)}
}

// Open implements output.RecordWriter.
func (w *recordWriter) Open(recMeta sqlz.RecordMeta) error {
	w.recMeta = recMeta

	var indent, newline string
	if w.fm.Pretty {
		indent = w.fm.Indent
		newline = "\n"
	}

	w.fieldPrintFns = make([]func(w io.Writer, a ...interface{}), len(recMeta))

	w.tplFieldStart = make([]string, len(recMeta))
	w.tplFieldEnd = make([]string, len(recMeta))

	w.tplRecStart = "\n" + indent + w.elemColor.Sprintf("<%s>", recElemName)
	w.tplRecEnd = newline + indent + w.elemColor.Sprintf("</%s>", recElemName)

	for i, name := range recMeta.Names() {
		elementName := stringz.SanitizeAlphaNumeric(name, '_')
		w.tplFieldStart[i] = newline + indent + indent + w.elemColor.Sprintf("<%s>", elementName)
		w.tplFieldEnd[i] = w.elemColor.Sprintf("</%s>", elementName)

		if w.fm.IsMonochrome() {
			w.fieldPrintFns[i] = monoPrint
			continue
		}

		switch w.recMeta[i].Kind() {
		default:
			w.fieldPrintFns[i] = monoPrint
		case kind.Datetime, kind.Date, kind.Time:
			w.fieldPrintFns[i] = w.fm.Datetime.FprintFunc()
		case kind.Int, kind.Decimal, kind.Float:
			w.fieldPrintFns[i] = w.fm.Number.FprintFunc()
		case kind.Bool:
			w.fieldPrintFns[i] = w.fm.Bool.FprintFunc()
		case kind.Bytes:
			w.fieldPrintFns[i] = w.fm.Bytes.FprintFunc()
		case kind.Text:
			w.fieldPrintFns[i] = w.fm.String.FprintFunc()
		}
	}

	w.outBuf = &bytes.Buffer{}
	w.outBuf.WriteString(w.fm.Faint.Sprint(decl))
	return nil
}

// monoPrint delegates to fmt.Fprint, for
// monochrome (non-color) printing.
func monoPrint(w io.Writer, a ...interface{}) {
	_, _ = fmt.Fprint(w, a...)
}

// Flush implements output.RecordWriter.
func (w *recordWriter) Flush() error {
	_, err := w.outBuf.WriteTo(w.out) // resets buf
	return errz.Err(err)
}

// Close implements output.RecordWriter.
func (w *recordWriter) Close() error {
	w.outBuf.WriteByte('\n')

	if w.recsWritten {
		w.outBuf.WriteString(w.elemColor.Sprintf("</%s>", recsElementName))
	} else {
		// empty element: <records />
		w.outBuf.WriteString(w.elemColor.Sprintf("<%s />", recsElementName))
	}

	w.outBuf.WriteByte('\n')

	return w.Flush()
}

// WriteRecords implements output.RecordWriter.
// Note that (by design) the XML element is omitted for any nil value
// in a record.
func (w *recordWriter) WriteRecords(recs []sqlz.Record) error {
	if len(recs) == 0 {
		return nil
	}

	if !w.recsWritten {
		w.outBuf.WriteByte('\n')
		w.outBuf.WriteString(w.elemColor.Sprintf("<%s>", recsElementName))
		w.recsWritten = true
	}

	var err error
	for _, rec := range recs {
		err = w.writeRecord(rec)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *recordWriter) writeRecord(rec sqlz.Record) error {
	var err error
	tmpBuf := &bytes.Buffer{}

	w.outBuf.WriteString(w.tplRecStart)

	for i, val := range rec {
		if val == nil {
			continue // omit the element if val is nil
		}

		w.outBuf.WriteString(w.tplFieldStart[i])

		switch val := val.(type) {
		default:
			// should never happen
			err = xml.EscapeText(tmpBuf, []byte(fmt.Sprintf("%v", val)))
			if err != nil {
				return errz.Err(err)
			}
			w.fieldPrintFns[i](w.outBuf, tmpBuf.String())
			tmpBuf.Reset()
		case nil:
			// should never happen
		case *string:
			err = xml.EscapeText(tmpBuf, []byte(*val))
			if err != nil {
				return errz.Err(err)
			}
			w.fieldPrintFns[i](w.outBuf, tmpBuf.String())
			tmpBuf.Reset()
		case *[]byte:
			w.fieldPrintFns[i](w.outBuf, base64.StdEncoding.EncodeToString(*val))
		case *bool:
			w.fieldPrintFns[i](w.outBuf, strconv.FormatBool(*val))
		case *int64:
			w.fieldPrintFns[i](w.outBuf, strconv.FormatInt(*val, 10))
		case *float64:
			w.fieldPrintFns[i](w.outBuf, stringz.FormatFloat(*val))
		case *time.Time:
			switch w.recMeta[i].Kind() {
			default:
				w.fieldPrintFns[i](w.outBuf, val.Format(stringz.DatetimeFormat))
			case kind.Time:
				w.fieldPrintFns[i](w.outBuf, val.Format(stringz.TimeFormat))
			case kind.Date:
				w.fieldPrintFns[i](w.outBuf, val.Format(stringz.DateFormat))
			}
		}

		w.outBuf.WriteString(w.tplFieldEnd[i])
	}

	w.outBuf.WriteString(w.tplRecEnd)

	if w.outBuf.Len() > output.FlushThreshold {
		return w.Flush()
	}

	return nil
}

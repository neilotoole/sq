// Package xmlw implements output writers for XML.
package xmlw

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// recordWriter implements output.RecordWriter.
type recordWriter struct {
	out io.Writer
	pr  *output.Printing

	// outBuf holds output prior to flushing.
	outBuf *bytes.Buffer

	elemColor *color.Color

	tplRecStart string
	tplRecEnd   string

	recMeta record.Meta

	tplFieldStart []string
	tplFieldEnd   []string

	fieldPrintFns []func(w io.Writer, a ...any)

	// recWritten indicates that at least one record has been written
	// to outBuf.
	recsWritten bool
}

const (
	decl            = "<?xml version=\"1.0\"?>"
	recsElementName = "records"
	recElemName     = "record"
)

var _ output.NewRecordWriterFunc = NewRecordWriter

// NewRecordWriter returns an output.RecordWriter instance for XML.
func NewRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return &recordWriter{out: out, pr: pr, elemColor: pr.Key.Add(color.Faint)}
}

// Open implements output.RecordWriter.
func (w *recordWriter) Open(_ context.Context, recMeta record.Meta) error {
	w.recMeta = recMeta

	var indent, newline string
	if !w.pr.Compact {
		indent = w.pr.Indent
		newline = "\n"
	}

	w.fieldPrintFns = make([]func(w io.Writer, a ...any), len(recMeta))

	w.tplFieldStart = make([]string, len(recMeta))
	w.tplFieldEnd = make([]string, len(recMeta))

	w.tplRecStart = "\n" + indent + w.elemColor.Sprintf("<%s>", recElemName)
	w.tplRecEnd = newline + indent + w.elemColor.Sprintf("</%s>", recElemName)

	for i, name := range recMeta.MungedNames() {
		elementName := stringz.SanitizeAlphaNumeric(name, '_')
		w.tplFieldStart[i] = newline + indent + indent + w.elemColor.Sprintf("<%s>", elementName)
		w.tplFieldEnd[i] = w.elemColor.Sprintf("</%s>", elementName)

		if w.pr.IsMonochrome() {
			w.fieldPrintFns[i] = monoPrint
			continue
		}

		switch w.recMeta[i].Kind() { //nolint:exhaustive // ignore kind.Unknown and kind.Null
		default:
			w.fieldPrintFns[i] = monoPrint
		case kind.Datetime, kind.Date, kind.Time:
			w.fieldPrintFns[i] = w.pr.Datetime.FprintFunc()
		case kind.Int, kind.Decimal, kind.Float:
			w.fieldPrintFns[i] = w.pr.Number.FprintFunc()
		case kind.Bool:
			w.fieldPrintFns[i] = w.pr.Bool.FprintFunc()
		case kind.Bytes:
			w.fieldPrintFns[i] = w.pr.Bytes.FprintFunc()
		case kind.Text:
			w.fieldPrintFns[i] = w.pr.String.FprintFunc()
		}
	}

	w.outBuf = &bytes.Buffer{}
	w.outBuf.WriteString(w.pr.Faint.Sprint(decl))
	return nil
}

// monoPrint delegates to fmt.Fprint, for
// monochrome (non-color) printing.
func monoPrint(w io.Writer, a ...any) {
	_, _ = fmt.Fprint(w, a...)
}

// Flush implements output.RecordWriter.
func (w *recordWriter) Flush(context.Context) error {
	_, err := w.outBuf.WriteTo(w.out) // resets buf
	return errz.Err(err)
}

// Close implements output.RecordWriter.
func (w *recordWriter) Close(ctx context.Context) error {
	w.outBuf.WriteByte('\n')

	if w.recsWritten {
		w.outBuf.WriteString(w.elemColor.Sprintf("</%s>", recsElementName))
	} else {
		// empty element: <records />
		w.outBuf.WriteString(w.elemColor.Sprintf("<%s />", recsElementName))
	}

	w.outBuf.WriteByte('\n')

	return w.Flush(ctx)
}

// WriteRecords implements output.RecordWriter.
// Note that (by design) the XML element is omitted for any nil value
// in a record.
func (w *recordWriter) WriteRecords(ctx context.Context, recs []record.Record) error {
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
		err = w.writeRecord(ctx, rec)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *recordWriter) writeRecord(ctx context.Context, rec record.Record) error {
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
		case string:
			err = xml.EscapeText(tmpBuf, []byte(val))
			if err != nil {
				return errz.Err(err)
			}
			w.fieldPrintFns[i](w.outBuf, tmpBuf.String())
			tmpBuf.Reset()
		case []byte:
			w.fieldPrintFns[i](w.outBuf, base64.StdEncoding.EncodeToString(val))
		case bool:
			w.fieldPrintFns[i](w.outBuf, strconv.FormatBool(val))
		case int64:
			w.fieldPrintFns[i](w.outBuf, strconv.FormatInt(val, 10))
		case float64:
			w.fieldPrintFns[i](w.outBuf, stringz.FormatFloat(val))
		case decimal.Decimal:
			w.fieldPrintFns[i](w.outBuf, stringz.FormatDecimal(val))
		case time.Time:
			switch w.recMeta[i].Kind() { //nolint:exhaustive
			default:
				w.fieldPrintFns[i](w.outBuf, w.pr.FormatDatetime(val))
			case kind.Time:
				w.fieldPrintFns[i](w.outBuf, w.pr.FormatTime(val))
			case kind.Date:
				w.fieldPrintFns[i](w.outBuf, w.pr.FormatDate(val))
			}
		}

		w.outBuf.WriteString(w.tplFieldEnd[i])
	}

	w.outBuf.WriteString(w.tplRecEnd)

	if w.outBuf.Len() > w.pr.FlushThreshold {
		return w.Flush(ctx)
	}

	return nil
}

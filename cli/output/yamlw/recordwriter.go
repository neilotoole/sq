package yamlw

import (
	"bytes"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/fatih/color"
	goccy "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
)

var _ output.NewRecordWriterFunc = NewRecordWriter

// NewRecordWriter returns an output.RecordWriter that writes YAML.
func NewRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return &recordWriter{
		out: out,
		pr:  pr,
	}
}

type recordWriter struct {
	mu         sync.Mutex
	out        io.Writer
	pr         *output.Printing
	recMeta    record.Meta
	fieldNames []string
	buf        *bytes.Buffer
	enc        *goccy.Encoder
	clrs       []*color.Color
	keys       []string
	null       string
}

// Open implements output.RecordWriter.
func (w *recordWriter) Open(recMeta record.Meta) error {
	w.recMeta = recMeta
	w.fieldNames = w.recMeta.MungedNames()
	w.buf = &bytes.Buffer{}
	w.enc = goccy.NewEncoder(io.Discard, decimalMarshaler)
	w.clrs = make([]*color.Color, len(w.recMeta))
	w.keys = make([]string, len(w.recMeta))
	w.null = w.pr.Null.Sprint("null")

	var (
		node ast.Node
		err  error
	)

	// Generate the field keys and colors
	for i := range w.recMeta {
		node, err = w.enc.EncodeToNode(w.recMeta[i].MungedName())
		if err != nil {
			// Shouldn't happen
			return errz.Wrapf(err, "yaml: failed to encode field name: %s", w.recMeta[i].MungedName())
		}
		if node == nil {
			// Also shouldn't happen
			return errz.Errorf("yaml: failed to encode field name: %s: encoded to nil", w.recMeta[i].MungedName())
		}

		w.keys[i] = w.pr.Key.Sprint(node.String())

		// Set up the colors
		switch w.recMeta[i].Kind() {
		case kind.Bytes:
			w.clrs[i] = w.pr.Bytes
		case kind.Null:
			w.clrs[i] = w.pr.Null
		case kind.Text:
			w.clrs[i] = w.pr.String
		case kind.Bool:
			w.clrs[i] = w.pr.Bool
		case kind.Datetime, kind.Date, kind.Time:
			w.clrs[i] = w.pr.Datetime
		case kind.Int, kind.Float, kind.Decimal:
			w.clrs[i] = w.pr.Number
		case kind.Unknown:
			w.clrs[i] = w.pr.Normal
		default:
			w.clrs[i] = w.pr.Normal
		}
	}

	return nil
}

// WriteRecords implements output.RecordWriter.
func (w *recordWriter) WriteRecords(recs []record.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(recs) == 0 {
		return nil
	}

	var (
		err  error
		node ast.Node

		buf  = w.buf
		clrs = w.clrs
		keys = w.keys
	)

	for i, rec := range recs {
		buf.WriteString("- ")

		for j := range rec {
			if j > 0 {
				buf.WriteString("\n  ")
			}
			buf.WriteString(keys[j])
			buf.WriteString(": ")
			val := rec[j]

			if val == nil {
				buf.WriteString(w.null)
				continue
			}

			if tm, ok := val.(time.Time); ok {
				if tm.IsZero() {
					buf.WriteString(w.null)
					continue
				}

				var s string
				s, err = w.renderTime(w.recMeta[j], tm)
				if err != nil {
					return err
				}
				buf.WriteString(s)
				continue
			}

			node, err = w.enc.EncodeToNode(val)
			if err != nil {
				return errz.Wrapf(err, "yaml: failed to encode result: [%d].%s",
					i, w.recMeta[j].MungedName())
			}

			if node == nil {
				// Shouldn't happen
				buf.WriteString(w.null)
			} else {
				buf.WriteString(clrs[j].Sprint(node.String()))
			}
		}

		buf.WriteRune('\n')
	}

	return nil
}

// Flush implements output.RecordWriter.
func (w *recordWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.buf.WriteTo(w.out)
	return errz.Err(err)
}

// Close implements output.RecordWriter.
func (w *recordWriter) Close() error {
	return w.Flush()
}

// renderTime renders the *time.Time val into a fully-rendered string
// ready for writing out.
func (w *recordWriter) renderTime(fieldMeta *record.FieldMeta, val any) (string, error) {
	if val == nil {
		return w.null, nil
	}

	var (
		pr            = w.pr
		timeFormatter func(time.Time) string
		asNumber      bool
		isNumber      bool
		tm            time.Time
		ok            bool
	)

	if tm, ok = val.(time.Time); !ok {
		return "", errz.Errorf("unexpected value type: expected %T, but got %T", tm, val)
	}

	if tm.IsZero() {
		return w.null, nil
	}

	switch fieldMeta.Kind() { //nolint:exhaustive
	case kind.Datetime:
		timeFormatter = pr.FormatDatetime
		asNumber = pr.FormatDatetimeAsNumber
	case kind.Date:
		timeFormatter = pr.FormatDate
		asNumber = pr.FormatDateAsNumber
	case kind.Time:
		timeFormatter = pr.FormatTime
		asNumber = pr.FormatTimeAsNumber
	default:
		// Shouldn't happen
		return "", errz.Errorf("unexpected data kind: expected a time-like value, but got {%s}: %v",
			fieldMeta.Kind(), val)
	}

	v := timeFormatter(tm)
	if _, err := strconv.ParseInt(v, 10, 64); err == nil {
		isNumber = true
	}

	switch {
	case isNumber && asNumber:
		return w.pr.Datetime.Sprint(v), nil
	case isNumber:
		// If the value is a number, but asNumber is false, we want to render it
		// as a string? Not sure about the desired behavior.

		// We could surround the number in quotes, e.g.
		//
		//  return w.pr.Datetime.Sprintf("%q", v), nil
		//
		// But is that what we want?
		// We'll just leave it naked for now, and await user feedback.
		return w.pr.Datetime.Sprint(v), nil
	default:
		// It's not a number, it's some kind of non-numeric date/time value.
	}

	node, err := w.enc.EncodeToNode(v)
	if err != nil {
		return "", errz.Err(err)
	}

	return w.pr.Datetime.Sprint(node.String()), nil
}

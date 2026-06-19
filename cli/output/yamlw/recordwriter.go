package yamlw

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/fatih/color"
	goccy "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/shopspring/decimal"

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
	out        io.Writer
	pr         *output.Printing
	buf        *bytes.Buffer
	enc        *goccy.Encoder
	null       string
	recMeta    record.Meta
	fieldNames []string
	clrs       []*color.Color
	byValue    []bool
	keys       []string
	mu         sync.Mutex
}

// Open implements output.RecordWriter.
func (w *recordWriter) Open(_ context.Context, recMeta record.Meta) error {
	w.recMeta = recMeta
	w.fieldNames = w.recMeta.MungedNames()
	w.buf = &bytes.Buffer{}
	w.enc = goccy.NewEncoder(io.Discard, newDecimalMarshaler(w.pr.DecimalAsNumber))
	w.clrs = make([]*color.Color, len(w.recMeta))
	w.byValue = make([]bool, len(w.recMeta))
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
			// The declared kind doesn't determine the color; resolve it from
			// the Go value type at write time, mirroring the JSON encoder. See #851.
			w.clrs[i] = w.pr.Normal
			w.byValue[i] = true
		default:
			w.clrs[i] = w.pr.Normal
		}
	}

	return nil
}

// colorForValue returns the color to apply to val, dispatching on the Go value
// type. It mirrors the JSON encoder's value-type dispatch (see jsonw.encodeAny)
// and is used for columns whose declared kind is kind.Unknown, where the column
// color can't be determined from metadata alone. See #851.
func colorForValue(pr *output.Printing, val any) *color.Color {
	switch val.(type) {
	case int64, float64, decimal.Decimal:
		return pr.Number
	case bool:
		return pr.Bool
	case []byte:
		return pr.Bytes
	case string:
		return pr.String
	default:
		// time.Time is intercepted before this point, and nil is rendered as
		// null; anything else falls back to no color.
		return pr.Normal
	}
}

// WriteRecords implements output.RecordWriter.
func (w *recordWriter) WriteRecords(ctx context.Context, recs []record.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(recs) == 0 {
		return nil
	}

	var (
		err  error
		node ast.Node

		buf     = w.buf
		clrs    = w.clrs
		byValue = w.byValue
		keys    = w.keys
	)

	for i, rec := range recs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
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

			// goccy renders a []byte as a YAML sequence of byte ints. Instead
			// encode it as a base64 string, matching the JSON output. A nil
			// slice is a NULL value, rendered as null like the JSON encoder
			// (a typed-nil []byte does not satisfy the val == nil check above).
			// See #851.
			if b, ok := val.([]byte); ok {
				if b == nil {
					buf.WriteString(w.null)
					continue
				}
				val = base64.StdEncoding.EncodeToString(b)
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
				clr := clrs[j]
				if byValue[j] {
					// Resolve the color from the original record value, which
					// may have been transformed above (e.g. []byte to base64).
					clr = colorForValue(w.pr, rec[j])
				}
				buf.WriteString(clr.Sprint(node.String()))
			}
		}

		buf.WriteRune('\n')
	}

	return nil
}

// Flush implements output.RecordWriter.
func (w *recordWriter) Flush(context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.buf.WriteTo(w.out)
	return errz.Err(err)
}

// Close implements output.RecordWriter.
func (w *recordWriter) Close(ctx context.Context) error {
	return w.Flush(ctx)
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

	if isGoccyTimestamp(v) {
		return w.pr.Datetime.Sprint(v), nil
	}

	// For safety, we'll use the goccy encoder to render the value.
	node, err := w.enc.EncodeToNode(v)
	if err != nil {
		return "", errz.Err(err)
	}

	return w.pr.Datetime.Sprint(node.String()), nil
}

// isGoccyTimestamp returns true if one of the timestamp values that goccy has
// decided it wants to enquote. This goccy change occurred in v0.15.9.
//
// See:
//   - Issue: https://github.com/goccy/go-yaml/issues/430
//   - PR: https://github.com/goccy/go-yaml/pull/515
//
// However, sq does NOT want to enquote these values. This function is a
// workaround (copied from goccy) to determine if val one of these values. If
// it is, renderTime renders the value directly.
//
// We probably don't need to go to all of this trouble in renderTime, and could
// just render the value directly as a string, but if the rendered output is
// weird (e.g. includes newlines?), it's better to play it safe and use the
// goccy encoder's EncodeToNode method.

func isGoccyTimestamp(val string) bool {
	for _, format := range goccyTimestampFormats {
		if _, err := time.Parse(format, val); err == nil {
			return true
		}
	}
	return false
}

// This is a subset of the formats permitted by the regular expression
// defined at http://yaml.org/type/timestamp.html. Note that time.Parse
// cannot handle: "2001-12-14 21:59:43.10 -5" from the examples.
var goccyTimestampFormats = []string{
	time.RFC3339Nano,
	"2006-01-02t15:04:05.999999999Z07:00", // RFC3339Nano with lower-case "t".
	time.DateTime,
	time.DateOnly,

	// Not in examples, but to preserve backward compatibility by quoting time values.
	"15:4",
}

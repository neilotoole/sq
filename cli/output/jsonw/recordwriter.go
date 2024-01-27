package jsonw

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw/internal"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/record"
)

var _ output.NewRecordWriterFunc = NewStdRecordWriter

// NewStdRecordWriter returns a record writer that outputs each
// record as a JSON object that is an element of JSON array. This is
// to say, standard JSON. For example:
//
//	[
//	  {
//	    "actor_id": 1,
//	    "first_name": "PENELOPE",
//	    "last_name": "GUINESS",
//	    "last_update": "2020-06-11T02:50:54Z"
//	  },
//	  {
//	    "actor_id": 2,
//	    "first_name": "NICK",
//	    "last_name": "WAHLBERG",
//	    "last_update": "2020-06-11T02:50:54Z"
//	  }
//	]
func NewStdRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return &stdWriter{
		out: out,
		pr:  pr,
	}
}

// stdWriter outputs records in standard JSON format.
type stdWriter struct {
	err error
	out io.Writer
	pr  *output.Printing

	// outBuf is used to hold output prior to flushing.
	outBuf *bytes.Buffer

	tpl *stdTemplate

	// b is used as a buffer by writeRecord
	b []byte

	recMeta     record.Meta
	encodeFns   []func(b []byte, v any) ([]byte, error)
	mu          sync.Mutex
	recsWritten bool
}

// Open implements output.RecordWriter.
func (w *stdWriter) Open(_ context.Context, recMeta record.Meta) error {
	if w.err != nil {
		return w.err
	}

	w.outBuf = &bytes.Buffer{}
	w.recMeta = recMeta

	if len(recMeta) == 0 {
		// This can happen, e.g. with postgres for an empty table
		w.tpl = &stdTemplate{
			header: []byte("["),
			footer: []byte("]"),
		}
	} else {
		w.encodeFns = getFieldEncoders(recMeta, w.pr)

		tpl, err := newStdTemplate(recMeta, w.pr)
		if err != nil {
			w.err = err
			return err
		}

		w.tpl = tpl
	}

	w.outBuf.Write(w.tpl.header)

	return nil
}

// WriteRecords implements output.RecordWriter.
func (w *stdWriter) WriteRecords(_ context.Context, recs []record.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}

	var err error
	for i := range recs {
		err = w.writeRecord(recs[i])
		if err != nil {
			w.err = err
			return err
		}
	}
	return nil
}

func (w *stdWriter) writeRecord(rec record.Record) error {
	var err error

	if w.recsWritten {
		// we need to add the separator
		w.b = append(w.b, w.tpl.recSep...)
	} else {
		// This is the first record: we'll need the separator next time.
		w.recsWritten = true
	}

	for i := range w.recMeta {
		w.b = append(w.b, w.tpl.recTpl[i]...)
		w.b, err = w.encodeFns[i](w.b, rec[i])
		if err != nil {
			return errz.Err(err)
		}
	}

	w.b = append(w.b, w.tpl.recTpl[len(w.recMeta)]...)
	w.outBuf.Write(w.b)
	w.b = w.b[:0]

	if w.outBuf.Len() > w.pr.FlushThreshold {
		return w.doFlush()
	}

	return nil
}

// Flush implements output.RecordWriter.
func (w *stdWriter) Flush(context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.doFlush()
}

func (w *stdWriter) doFlush() error {
	if w.err != nil {
		return w.err
	}
	if _, err := w.outBuf.WriteTo(w.out); err != nil {
		return errz.Err(err)
	}
	return nil
}

// Close implements output.RecordWriter.
func (w *stdWriter) Close(ctx context.Context) error {
	if w.err != nil {
		return w.err
	}

	if w.recsWritten && !w.pr.Compact {
		w.outBuf.WriteRune('\n')
	}

	w.outBuf.Write(w.tpl.footer)
	return w.Flush(ctx)
}

// stdTemplate holds the various parts of the output template
// used by stdWriter.
type stdTemplate struct {
	header []byte
	recTpl [][]byte
	recSep []byte
	footer []byte
}

func newStdTemplate(recMeta record.Meta, pr *output.Printing) (*stdTemplate, error) {
	tpl := make([][]byte, len(recMeta)+1)
	clrs := internal.NewColors(pr)
	pnc := newPunc(pr)

	fieldNames := make([][]byte, len(recMeta))

	var err error
	for i := range recMeta {
		fieldNames[i], err = encodeString(nil, recMeta[i].MungedName(), false)
		if err != nil {
			return nil, errz.Err(err)
		}
		if !pr.IsMonochrome() {
			fieldNames[i] = clrs.AppendKey(nil, fieldNames[i])
		}
	}

	if pr.Compact {
		tpl[0] = append(tpl[0], pnc.lBrace...)
		tpl[0] = append(tpl[0], fieldNames[0]...)
		tpl[0] = append(tpl[0], pnc.colon...)

		for i := 1; i < len(fieldNames); i++ {
			tpl[i] = append(tpl[i], pnc.comma...)
			tpl[i] = append(tpl[i], fieldNames[i]...)
			tpl[i] = append(tpl[i], pnc.colon...)
		}

		tpl[len(recMeta)] = append(tpl[len(recMeta)], pnc.rBrace...)

		stdTpl := &stdTemplate{recTpl: tpl}

		stdTpl.header = append(stdTpl.header, pnc.lBracket...)
		stdTpl.footer = append(stdTpl.footer, pnc.rBracket...)
		stdTpl.footer = append(stdTpl.footer, '\n')
		stdTpl.recSep = append(stdTpl.recSep, pnc.comma...)

		return stdTpl, nil
	}

	// Else we're doing pretty printing
	tpl[0] = append(tpl[0], []byte("\n"+pr.Indent)...)
	tpl[0] = append(tpl[0], pnc.lBrace...)
	tpl[0] = append(tpl[0], '\n')
	tpl[0] = append(tpl[0], strings.Repeat(pr.Indent, 2)...)

	tpl[0] = append(tpl[0], fieldNames[0]...)
	tpl[0] = append(tpl[0], pnc.colon...)
	tpl[0] = append(tpl[0], ' ')

	for i := 1; i < len(fieldNames); i++ {
		tpl[i] = append(tpl[i], pnc.comma...)
		tpl[i] = append(tpl[i], '\n')
		tpl[i] = append(tpl[i], strings.Repeat(pr.Indent, 2)...)
		tpl[i] = append(tpl[i], fieldNames[i]...)
		tpl[i] = append(tpl[i], pnc.colon...)
		tpl[i] = append(tpl[i], ' ')
	}

	last := len(recMeta)

	tpl[last] = append(tpl[last], '\n')
	tpl[last] = append(tpl[last], pr.Indent...)
	tpl[last] = append(tpl[last], pnc.rBrace...)

	stdTpl := &stdTemplate{recTpl: tpl}
	stdTpl.header = append(stdTpl.header, pnc.lBracket...)
	stdTpl.footer = append(stdTpl.footer, pnc.rBracket...)
	stdTpl.footer = append(stdTpl.footer, '\n')
	stdTpl.recSep = append(stdTpl.recSep, pnc.comma...)
	return stdTpl, nil
}

var _ output.NewRecordWriterFunc = NewObjectRecordWriter

// NewObjectRecordWriter writes out each record as a JSON object
// on its own line. For example:
//
//	{"actor_id": 1, "first_name": "PENELOPE", "last_name": "GUINESS", "last_update": "2020-06-11T02:50:54Z"}
//	{"actor_id": 2, "first_name": "NICK", "last_name": "WAHLBERG", "last_update": "2020-06-11T02:50:54Z"}
func NewObjectRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return &lineRecordWriter{
		out:      out,
		pr:       pr,
		newTplFn: newJSONObjectsTemplate,
	}
}

var _ output.NewRecordWriterFunc = NewArrayRecordWriter

// NewArrayRecordWriter returns a RecordWriter that outputs each
// record as a JSON array on its own line. For example:
//
//	[1, "PENELOPE", "GUINESS", "2020-06-11T02:50:54Z"]
//	[2, "NICK", "WAHLBERG", "2020-06-11T02:50:54Z"]
func NewArrayRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return &lineRecordWriter{
		out:      out,
		pr:       pr,
		newTplFn: newJSONArrayTemplate,
	}
}

// lineRecordWriter is an output.RecordWriter that outputs each
// record as its own line. This type is used to generate
// both the "json array" and "json lines" output formats.
//
// For each element of a record, there is an encoding function in
// w.encodeFns. So, there's len(rec) encode fns. The elements of w.tpl
// surround the output of each encode func. Therefore there
// are len(rec)+1 tpl elements.
type lineRecordWriter struct {
	err error
	out io.Writer
	pr  *output.Printing

	// outBuf holds the output of the writer prior to flushing.
	outBuf *bytes.Buffer

	// newTplFn is invoked during open to get the "template" for
	// generating output.
	newTplFn func(record.Meta, *output.Printing) ([][]byte, error)

	recMeta record.Meta

	// tpl is a slice of []byte, where len(tpl) == len(recMeta) + 1.
	tpl [][]byte

	// encodeFns holds an encoder func for each element of the record.
	encodeFns []func(b []byte, v any) ([]byte, error)
	mu        sync.Mutex
}

// Open implements output.RecordWriter.
func (w *lineRecordWriter) Open(_ context.Context, recMeta record.Meta) error {
	if w.err != nil {
		return w.err
	}

	w.outBuf = &bytes.Buffer{}
	w.recMeta = recMeta

	w.tpl, w.err = w.newTplFn(recMeta, w.pr)
	if w.err != nil {
		return w.err
	}
	w.encodeFns = getFieldEncoders(recMeta, w.pr)
	return nil
}

// WriteRecords implements output.RecordWriter.
func (w *lineRecordWriter) WriteRecords(ctx context.Context, recs []record.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}

	var err error
	for i := range recs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		err = w.writeRecord(ctx, recs[i])
		if err != nil {
			w.err = err
			return err
		}
	}
	return nil
}

func (w *lineRecordWriter) writeRecord(ctx context.Context, rec record.Record) error {
	var err error
	b := make([]byte, 0, 10)

	for i := range w.recMeta {
		b = append(b, w.tpl[i]...)
		b, err = w.encodeFns[i](b, rec[i])
		if err != nil {
			return errz.Err(err)
		}
	}

	b = append(b, w.tpl[len(w.recMeta)]...)
	w.outBuf.Write(b)

	if w.outBuf.Len() > w.pr.FlushThreshold {
		return w.Flush(ctx)
	}

	return nil
}

// Flush implements output.RecordWriter.
func (w *lineRecordWriter) Flush(context.Context) error {
	if w.err != nil {
		return w.err
	}
	_, err := w.outBuf.WriteTo(w.out)
	if err != nil {
		return errz.Err(err)
	}
	return nil
}

// Close implements output.RecordWriter.
func (w *lineRecordWriter) Close(ctx context.Context) error {
	if w.err != nil {
		return w.err
	}

	return w.Flush(ctx)
}

func newJSONObjectsTemplate(recMeta record.Meta, pr *output.Printing) ([][]byte, error) {
	tpl := make([][]byte, len(recMeta)+1)
	clrs := internal.NewColors(pr)
	pnc := newPunc(pr)

	fieldNames := make([][]byte, len(recMeta))
	var err error
	for i := range recMeta {
		fieldNames[i], err = encodeString(nil, recMeta[i].MungedName(), false)
		if err != nil {
			return nil, errz.Err(err)
		}
		if !pr.IsMonochrome() {
			fieldNames[i] = clrs.AppendKey(nil, fieldNames[i])
		}
	}

	tpl[0] = append(tpl[0], pnc.lBrace...)
	tpl[0] = append(tpl[0], fieldNames[0]...)
	tpl[0] = append(tpl[0], pnc.colon...)
	if !pr.Compact {
		tpl[0] = append(tpl[0], ' ')
	}

	for i := 1; i < len(fieldNames); i++ {
		tpl[i] = append(tpl[i], pnc.comma...)
		if !pr.Compact {
			tpl[i] = append(tpl[i], ' ')
		}

		tpl[i] = append(tpl[i], fieldNames[i]...)

		tpl[i] = append(tpl[i], pnc.colon...)
		if !pr.Compact {
			tpl[i] = append(tpl[i], ' ')
		}
	}

	tpl[len(recMeta)] = append(tpl[len(recMeta)], pnc.rBrace...)
	tpl[len(recMeta)] = append(tpl[len(recMeta)], '\n')

	return tpl, nil
}

func newJSONArrayTemplate(recMeta record.Meta, pr *output.Printing) ([][]byte, error) {
	tpl := make([][]byte, len(recMeta)+1)
	pnc := newPunc(pr)

	tpl[0] = append(tpl[0], pnc.lBracket...)

	for i := 1; i < len(recMeta); i++ {
		tpl[i] = append(tpl[i], pnc.comma...)
		if !pr.Compact {
			tpl[i] = append(tpl[i], ' ')
		}
	}

	tpl[len(recMeta)] = append(tpl[len(recMeta)], pnc.rBracket...)
	tpl[len(recMeta)] = append(tpl[len(recMeta)], '\n')

	return tpl, nil
}

package testh

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

var (
	recSinkCache = map[string]*RecordSink{}
	recSinkMu    sync.Mutex
)

// RecordsFromTbl returns a cached copy of all records from handle.tbl.
// The function performs a "SELECT * FROM tbl" and caches (in a package
// variable) the returned recs and recMeta for subsequent calls. Thus
// if the underlying data source records are modified, the returned records
// may be inconsistent.
//
// This function effectively exists to speed up testing times.
func RecordsFromTbl(tb testing.TB, handle, tbl string) (recMeta record.Meta, recs []record.Record) {
	recSinkMu.Lock()
	defer recSinkMu.Unlock()

	key := fmt.Sprintf("#rec_sink__%s__%s", handle, tbl)
	sink, ok := recSinkCache[key]
	if !ok {
		th := New(tb)
		th.Log = lg.Discard()
		src := th.Source(handle)
		var err error
		sink, err = th.QuerySQL(src, nil, "SELECT * FROM "+tbl)
		require.NoError(tb, err)
		recSinkCache[key] = sink
	}

	// Make copies so that the caller can mutate their records
	// without it affecting other callers
	recMeta = make(record.Meta, len(sink.RecMeta))

	// Don't need to make a deep copy of each FieldMeta because
	// the type is effectively immutable
	copy(recMeta, sink.RecMeta)

	recs = record.CloneSlice(sink.Recs)
	return recMeta, recs
}

// NewRecordMeta builds a new record.Meta instance for testing.
func NewRecordMeta(colNames []string, colKinds []kind.Kind) record.Meta {
	recMeta := make(record.Meta, len(colNames))
	for i := range colNames {
		knd := colKinds[i]
		ct := &record.ColumnTypeData{
			Name:             colNames[i],
			HasNullable:      true,
			Nullable:         true,
			DatabaseTypeName: sqlite3.DBTypeForKind(knd),
			ScanType:         KindScanType(knd),
			Kind:             knd,
		}

		recMeta[i] = record.NewFieldMeta(ct, ct.Name)
	}

	return recMeta
}

// KindScanType returns the default scan type for kind. The returned
// type is typically a sql.NullType.
func KindScanType(knd kind.Kind) reflect.Type {
	switch knd { //nolint:exhaustive
	default:
		return sqlz.RTypeNullString

	case kind.Text, kind.Decimal:
		return sqlz.RTypeNullString

	case kind.Int:
		return sqlz.RTypeNullInt64

	case kind.Bool:
		return sqlz.RTypeNullBool

	case kind.Float:
		return sqlz.RTypeNullFloat64

	case kind.Bytes:
		return sqlz.RTypeBytes

	case kind.Datetime:
		return sqlz.RTypeNullTime

	case kind.Date:
		return sqlz.RTypeNullTime

	case kind.Time:
		return sqlz.RTypeNullTime
	}
}

// RecordSink is an impl of output.RecordWriter that
// captures invocations of that interface.
type RecordSink struct {
	mu sync.Mutex

	// RecMeta holds the recMeta received via Open.
	RecMeta record.Meta

	// Recs holds the records received via WriteRecords.
	Recs []record.Record

	// Closed tracks the times Close was invoked.
	Closed []time.Time

	// Flushed tracks the times Flush was invoked.
	Flushed []time.Time
}

// Result returns the first (and only) value returned from
// a query like "SELECT COUNT(*) FROM actor". It is effectively
// the same as RecordSink.Recs[0][0]. The function will panic
// if there is no appropriate result.
func (r *RecordSink) Result() any {
	if len(r.Recs) == 0 || len(r.RecMeta) == 0 {
		panic("record sink has no data")
	}

	if len(r.RecMeta) != 1 {
		panic(fmt.Sprintf("record sink data should have 1 cold, but got %d", len(r.RecMeta)))
	}

	if len(r.Recs) != 1 {
		panic(fmt.Sprintf("record sink should have 1 record, but got %d", len(r.Recs)))
	}

	return r.Recs[0][0]
}

// Open implements libsq.RecordWriter.
func (r *RecordSink) Open(_ context.Context, recMeta record.Meta) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.RecMeta = recMeta
	return nil
}

// WriteRecords implements libsq.RecordWriter.
func (r *RecordSink) WriteRecords(_ context.Context, recs []record.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Recs = append(r.Recs, recs...)
	return nil
}

// Flush implements libsq.RecordWriter.
func (r *RecordSink) Flush(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Flushed = append(r.Flushed, time.Now())
	return nil
}

// Close implements libsq.RecordWriter.
func (r *RecordSink) Close(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Closed = append(r.Closed, time.Now())
	return nil
}

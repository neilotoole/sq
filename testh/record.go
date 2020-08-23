package testh

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/neilotoole/lg"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// RecordSink is a testing impl of output.RecordWriter that
// captures invocations of that interface.
type RecordSink struct {
	mu sync.Mutex

	// RecMeta holds the recMeta received via Open.
	RecMeta sqlz.RecordMeta

	// Recs holds the records received via WriteRecords.
	Recs []sqlz.Record

	// Closed tracks the times Close was invoked.
	Closed []time.Time

	// Flushed tracks the times Flush was invoked.
	Flushed []time.Time
}

// Open implements libsq.RecordWriter.
func (r *RecordSink) Open(recMeta sqlz.RecordMeta) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.RecMeta = recMeta
	return nil
}

// WriteRecords implements libsq.RecordWriter.
func (r *RecordSink) WriteRecords(recs []sqlz.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Recs = append(r.Recs, recs...)
	return nil
}

// Flush implements libsq.RecordWriter.
func (r *RecordSink) Flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Flushed = append(r.Flushed, time.Now())
	return nil
}

// Close implements libsq.RecordWriter.
func (r *RecordSink) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Closed = append(r.Closed, time.Now())
	return nil
}

var recSinkCache = map[string]*RecordSink{}
var recSinkMu sync.Mutex

// RecordsFromTbl returns a cached copy of all records from handle.tbl.
// The function performs a "SELECT * FROM tbl" and caches (in a package
// variable) the returned recs and recMeta for subsequent calls. Thus
// if the underlying data source records are modified, the returned records
// may be inconsistent.
//
// This function effectively exists to speed up testing times.
func RecordsFromTbl(tb testing.TB, handle, tbl string) (recMeta sqlz.RecordMeta, recs []sqlz.Record) {
	recSinkMu.Lock()
	defer recSinkMu.Unlock()

	key := fmt.Sprintf("#rec_sink__%s__%s", handle, tbl)
	sink, ok := recSinkCache[key]
	if !ok {
		th := New(tb)
		th.Log = lg.Discard()
		src := th.Source(handle)
		var err error
		sink, err = th.QuerySQL(src, "SELECT * FROM "+tbl)
		require.NoError(tb, err)
		recSinkCache[key] = sink
	}

	// Make copies so that the caller can mutate their records
	// without it affecting other callers
	recMeta = make(sqlz.RecordMeta, len(sink.RecMeta))
	for i := range sink.RecMeta {
		// Don't need to make a deep copy of each FieldMeta because
		// the type is effectively immutable
		recMeta[i] = sink.RecMeta[i]
	}

	recs = CopyRecords(sink.Recs)
	return recMeta, recs
}

// NewRecordMeta builds a new RecordMeta instance for testing.
func NewRecordMeta(colNames []string, colKinds []kind.Kind) sqlz.RecordMeta {
	recMeta := make(sqlz.RecordMeta, len(colNames))
	for i := range colNames {
		knd := colKinds[i]
		ct := &sqlz.ColumnTypeData{
			Name:             colNames[i],
			HasNullable:      true,
			Nullable:         true,
			DatabaseTypeName: sqlite3.DBTypeForKind(knd),
			ScanType:         KindScanType(knd),
			Kind:             knd,
		}

		recMeta[i] = sqlz.NewFieldMeta(ct)
	}

	return recMeta
}

// CopyRecords returns a deep copy of recs.
func CopyRecords(recs []sqlz.Record) []sqlz.Record {
	if recs == nil {
		return recs
	}

	if len(recs) == 0 {
		return []sqlz.Record{}
	}

	r2 := make([]sqlz.Record, len(recs))
	for i := range recs {
		r2[i] = CopyRecord(recs[i])
	}
	return r2
}

// CopyRecord returns a deep copy of rec.
func CopyRecord(rec sqlz.Record) sqlz.Record {
	if rec == nil {
		return nil
	}

	if len(rec) == 0 {
		return sqlz.Record{}
	}

	r2 := make(sqlz.Record, len(rec))
	for i := range rec {
		val := rec[i]
		switch val := val.(type) {
		case nil:
			continue
		case *int64:
			v := *val
			r2[i] = &v
		case *bool:
			v := *val
			r2[i] = &v
		case *float64:
			v := *val
			r2[i] = &v
		case *string:
			v := *val
			r2[i] = &v
		case *[]byte:
			b := make([]byte, len(*val))
			copy(b, *val)
			r2[i] = &b
		case *time.Time:
			v := *val
			r2[i] = &v
		default:
			panic(fmt.Sprintf("field [%d] has unacceptable record value type %T", i, val))
		}
	}

	return r2
}

// KindScanType returns the default scan type for kind. The returned
// type is typically a sql.NullType.
func KindScanType(knd kind.Kind) reflect.Type {
	switch knd {
	default:
		return sqlz.RTypeNullString

	case kind.Text, kind.KindDecimal:
		return sqlz.RTypeNullString

	case kind.KindInt:
		return sqlz.RTypeNullInt64

	case kind.KindBool:
		return sqlz.RTypeNullBool

	case kind.KindFloat:
		return sqlz.RTypeNullFloat64

	case kind.KindBytes:
		return sqlz.RTypeBytes

	case kind.KindDatetime:
		return sqlz.RTypeNullTime

	case kind.KindDate:
		return sqlz.RTypeNullTime

	case kind.KindTime:
		return sqlz.RTypeNullTime
	}
}

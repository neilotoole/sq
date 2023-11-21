// Package record holds the record.Record type, which is the
// core type for holding query results.
package record

import (
	"bytes"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// Record is a []any row of field values returned from a query.
//
// In the codebase, we distinguish between a "Record" and
// a "ScanRow", although both are []any and are closely related.
//
// An instance of ScanRow is passed to the sql rows.Scan method, and
// its elements may include implementations of the sql.Scanner interface
// such as sql.NullString, sql.NullInt64 or even driver-specific types.
//
// A Record is typically built from a ScanRow, unwrapping and
// munging elements such that the Record only contains standard types:
//
//	nil, int64, float64, bool, string, []byte, time.Time
//
// It is an error for a Record to contain elements of any other type.
type Record []any

// Value is the idealized generic type. One day, we'd like to be able
// to do something like this the below.
// FIXME: Delete this type.
type Value interface {
	~int64 | ~float64 | ~bool | ~string | ~[]byte | time.Time | decimal.Decimal
}

// Valid checks that each element of the record vals is
// of an acceptable type. On the first unacceptable element,
// the index of that element and an error are returned. On
// success (-1, nil) is returned.
//
// These acceptable types are:
//
//	nil, int64, float64, decimal.Decimal, bool, string, []byte, time.Time
func Valid(rec Record) (i int, err error) {
	var val any
	for i, val = range rec {
		switch val := val.(type) {
		case nil, int64, float64, bool, string, []byte, time.Time, decimal.Decimal:
			continue
		default:
			return i, errz.Errorf("field [%d] has unacceptable record value type %T", i, val)
		}
	}

	return -1, nil
}

// Equal returns true if each element of a and b are equal values.
func Equal(a, b Record) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	case len(a) != len(b):
		return false
	}

	var ok bool
	for i := range a {
		switch va := a[i].(type) {
		case nil:
			if b[i] != nil {
				return false
			}
		case []byte:
			var vb []byte
			if vb, ok = b[i].([]byte); !ok {
				return false
			}
			if !bytes.Equal(va, vb) {
				return false
			}
		case int64, float64, bool, string, time.Time, decimal.Decimal:
			switch vb := b[i].(type) {
			case int64, float64, bool, string, time.Time, decimal.Decimal:
				if vb != va {
					return false
				}
			default:
				return false
			}
		default:
			return false
		}
	}

	return true
}

// CloneSlice returns a deep copy of recs.
func CloneSlice(recs []Record) []Record {
	if recs == nil {
		return recs
	}

	if len(recs) == 0 {
		return []Record{}
	}

	r2 := make([]Record, len(recs))
	for i := range recs {
		r2[i] = Clone(recs[i])
	}
	return r2
}

// Clone returns a deep copy of rec.
func Clone(rec Record) Record {
	if rec == nil {
		return nil
	}

	if len(rec) == 0 {
		return Record{}
	}

	r2 := make(Record, len(rec))
	for i := range rec {
		val := rec[i]
		switch val := val.(type) {
		case nil:
			continue
		case int64, bool, float64, string, time.Time, decimal.Decimal:
			r2[i] = val
		case []byte:
			b := make([]byte, len(val))
			copy(b, val)
			r2[i] = b
		default:
			panic(fmt.Sprintf("field [%d] has unacceptable record value type %T", i, val))
		}
	}

	return r2
}

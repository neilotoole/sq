// Package record holds the record.Record type, which is the
// core type for holding query results.
package record

import (
	"bytes"
	"time"

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

// Valid checks that each element of the record vals is
// of an acceptable type. On the first unacceptable element,
// the index of that element and an error are returned. On
// success (-1, nil) is returned.
//
// These acceptable types, per the stdlib sql pkg, are:
//
//	nil, int64, float64, bool, string, []byte, time.Time
func Valid(_ Meta, rec Record) (i int, err error) {
	// FIXME: Valid should check the values of rec to see if they match recMeta's kinds

	var val any
	for i, val = range rec {
		switch val := val.(type) {
		case nil, int64, float64, bool, string, []byte, time.Time:
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
		case int64, float64, bool, string, time.Time:
			switch vb := b[i].(type) {
			case int64, float64, bool, string, time.Time:
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

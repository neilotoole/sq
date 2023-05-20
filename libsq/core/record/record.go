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
//	nil, *int64, *float64, *bool, *string, *[]byte, *time.Time
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
//	nil, *int64, *float64, *bool, *string, *[]byte, *time.Time
func Valid(_ Meta, rec Record) (i int, err error) {
	// FIXME: Valid should check the values of rec to see if they match recMeta's kinds

	var val any
	for i, val = range rec {
		switch val := val.(type) {
		case nil, *int64, *float64, *bool, *string, *[]byte, *time.Time:
			continue
		default:
			return i, errz.Errorf("field [%d] has unacceptable record value type %T", i, val)
		}
	}

	return -1, nil
}

// Equal returns true if rec1 and rec2 contain
// the same values.
func Equal(a, b Record) bool { //nolint:gocognit
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	case len(a) != len(b):
		return false
	}

	var i int
	var va, vb any

	for i, va = range a {
		vb = b[i]

		if va == nil && vb == nil {
			continue
		}

		if va == nil || vb == nil {
			return false
		}

		switch va := va.(type) {
		case *string:
			vb, ok := vb.(*string)
			if !ok {
				return false
			}

			if *va != *vb {
				return false
			}
		case *bool:
			vb, ok := vb.(*bool)
			if !ok {
				return false
			}

			if *va != *vb {
				return false
			}
		case *int64:
			vb, ok := vb.(*int64)
			if !ok {
				return false
			}

			if *va != *vb {
				return false
			}
		case *float64:
			vb, ok := vb.(*float64)
			if !ok {
				return false
			}

			if *va != *vb {
				return false
			}
		case *time.Time:
			vb, ok := vb.(*time.Time)
			if !ok {
				return false
			}

			if *va != *vb {
				return false
			}
		case *[]byte:
			vb, ok := vb.(*[]byte)
			if !ok {
				return false
			}

			if !bytes.Equal(*va, *vb) {
				return false
			}
		default:
			// Shouldn't happen
			return false
		}
	}

	return true
}

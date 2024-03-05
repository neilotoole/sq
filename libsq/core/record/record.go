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

// Valid checks that each element of the record vals is of an acceptable type.
// On the first unacceptable element, the index of that element and an error are
// returned. On success (-1, nil) is returned.
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

// Equal returns true if each element of a and b are equal values. Note that
// [time.Time] values compare both time and location components.
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
		case int64, float64, bool, string:
			switch vb := b[i].(type) {
			case int64, float64, bool, string:
				if vb != va {
					return false
				}
			default:
				return false
			}
		case decimal.Decimal:
			var vb decimal.Decimal
			if vb, ok = b[i].(decimal.Decimal); !ok {
				return false
			}
			return va.Equal(vb)
		case time.Time:
			var vb time.Time
			if vb, ok = b[i].(time.Time); !ok {
				return false
			}

			return equalTimes(va, vb)
		default:
			return false
		}
	}

	return true
}

// equalTimes returns true if a and b have the same time value and location.
//
// We don't use [time.Time.Equal] directly because it doesn't compare locations.
// And using a == b alone is not reliable because of monotonic clock issues. See
// the docs for [time.Time.Equal] for more info.
func equalTimes(a, b time.Time) bool {
	if a == b { //nolint:revive // time-equal
		// The linter complains about == comparison, but it's ok, because we also
		// check using Time.Equal below.
		return true
	}

	if !a.Equal(b) {
		return false
	}

	locA, locB := a.Location(), b.Location()
	if locA == locB {
		return true
	}

	return locA.String() == locB.String()
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

// NewPair returns a new Pair of records. The value returned by [Pair.Equal] is
// calculated once in this constructor. Mutating the records after construction
// may make that value inaccurate.
func NewPair(row int, rec1, rec2 Record) Pair {
	return Pair{row: row, rec1: rec1, rec2: rec2, equal: Equal(rec1, rec2)}
}

// NewIdenticalPairs returns a slice of [record.Pair] where [Pair.Rec1] and
// [Pair.Rec2] are the same record. [Pair.Equal] returns true for each pair.
func NewIdenticalPairs(row int, recs ...Record) []Pair {
	pairs := make([]Pair, len(recs))
	for i := 0; i < len(recs); i++ {
		pairs[i] = Pair{row: row + i, rec1: recs[i], rec2: recs[i], equal: true}
	}
	return pairs
}

// Pair is a pair of records, typically used to represent matching records from
// two different sources, which are being compared. Either of the pair may be
// nil. The value return by [Pair.Equal] is calculated once at the time of
// construction; mutating the records after construction may make that value
// inaccurate.
type Pair struct {
	rec1, rec2 Record
	row        int
	equal      bool
}

// Equal returns true if the records were equal at the time of the Pair's
// construction. Mutating the records after construction may make this value
// inaccurate. Two nil records are considered equal.
//
// See: [record.Equal].
func (p Pair) Equal() bool  { return p.equal }
func (p Pair) Row() int     { return p.row }
func (p Pair) Rec1() Record { return p.rec1 }
func (p Pair) Rec2() Record { return p.rec2 }

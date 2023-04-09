package csv

import (
	"encoding/csv"
	"io"
	"strconv"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/shopspring/decimal"
)

// predictColKinds examines up to maxExamine records in readAheadRecs
// and those returned by r to guess the kind of each field.
// Any additional records read from r are appended to readAheadRecs.
//
// This func considers these candidate kinds, in order of
// precedence: kind.Int, kind.Bool, kind.Decimal.
//
// kind.Decimal is chosen over kind.Float due to its greater flexibility.
// NOTE: Currently kind.Time and kind.Datetime are not examined.
//
// If any field (string) value cannot be parsed into a particular kind, that
// kind is excluded from the list of candidate kinds. The first of any
// remaining candidate kinds for each field is returned, or kind.Text if
// no candidate kinds.
//
// Deprecated: Use kind.Detector.
func predictColKinds(expectFieldCount int, r *csv.Reader, readAheadRecs *[][]string,
	maxExamine int) ([]kind.Kind,
	error,
) {
	// FIXME: [legacy] this function should switch to using kind.Detector

	candidateKinds := newCandidateFieldKinds(expectFieldCount)
	var examineCount int

	// First, read any records from the readAheadRecs buffer
	for recIndex := 0; recIndex < len(*readAheadRecs) && examineCount < maxExamine; recIndex++ {
		for fieldIndex := 0; fieldIndex < expectFieldCount; fieldIndex++ {
			candidateKinds[fieldIndex] = excludeFieldKinds(candidateKinds[fieldIndex],
				(*readAheadRecs)[recIndex][fieldIndex])
		}
		examineCount++
	}

	// Next, continue to read from r until we reach maxExamine records.
	for ; examineCount < maxExamine; examineCount++ {
		rec, err := r.Read()
		if err == io.EOF { //nolint:errorlint
			break
		}
		if err != nil {
			return nil, errz.Err(err)
		}

		if len(rec) != expectFieldCount {
			// safety check
			return nil, errz.Errorf("expected %d fields in CSV record but got %d", examineCount, len(rec))
		}

		for fieldIndex, fieldValue := range rec {
			candidateKinds[fieldIndex] = excludeFieldKinds(candidateKinds[fieldIndex], fieldValue)
		}

		// Add the recently read record to readAheadRecs so that
		// it's not lost.
		*readAheadRecs = append(*readAheadRecs, rec)
	}

	resultKinds := make([]kind.Kind, expectFieldCount)
	for i := range resultKinds {
		switch len(candidateKinds[i]) {
		case 0:
			// If all candidate kinds have been excluded, kind.Text is
			// the fallback option.
			resultKinds[i] = kind.Text
		default:
			// If there's one or more candidate kinds remaining, pick the first
			// one available, as it should be the most specific kind.
			resultKinds[i] = candidateKinds[i][0]
		}
	}
	return resultKinds, nil
}

// newCandidateFieldKinds returns a new slice of kind.Kind containing
// potential kinds for a field/column. The kinds are in an order of
// precedence.
func newCandidateFieldKinds(n int) [][]kind.Kind {
	kinds := make([][]kind.Kind, n)
	for i := range kinds {
		k := []kind.Kind{
			kind.Int,
			kind.Bool,
			kind.Decimal,
		}
		kinds[i] = k
	}

	return kinds
}

// excludeFieldKinds returns a filter of fieldCandidateKinds, removing those
// kinds which fieldVal cannot be converted to.
func excludeFieldKinds(fieldCandidateKinds []kind.Kind, fieldVal string) []kind.Kind {
	var resultCandidateKinds []kind.Kind

	if fieldVal == "" {
		// If the field is empty string, this could indicate a NULL value
		// for any kind. That is, we don't exclude a candidate kind due
		// to empty string.
		return fieldCandidateKinds
	}

	for _, knd := range fieldCandidateKinds {
		var err error

		switch knd { //nolint:exhaustive
		case kind.Int:
			_, err = strconv.Atoi(fieldVal)
		case kind.Bool:
			_, err = strconv.ParseBool(fieldVal)
		case kind.Decimal:
			_, err = decimal.NewFromString(fieldVal)
		default:
		}

		if err == nil {
			resultCandidateKinds = append(resultCandidateKinds, knd)
		}
	}

	return resultCandidateKinds
}

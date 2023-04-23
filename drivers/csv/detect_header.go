package csv

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
)

// hasHeaderRow returns true if a header row is explicitly
// set in opts, or if detectHeaderRow detects that the first
// row of recs seems to be a header.
func hasHeaderRow(recs [][]string, opts options.Options) (bool, error) {
	if OptImportHeader.IsSet(opts) {
		return OptImportHeader.Get(opts), nil
	}

	return detectHeaderRow(recs)
}

// detectHeaderRow returns true if recs has a header row.
// The recs arg should be regularly shaped: each rec should
// have the same number of fields.
func detectHeaderRow(recs [][]string) (hasHeader bool, err error) {
	if len(recs) < 2 {
		// If zero records, obviously no header row.
		// If one record... well, is there any way of determining if
		// it's a header row or not? Probably best to treat it as a data row.
		return false, nil
	}

	firstRowHash, err := calcKindHash(recs[0:1])
	if err != nil {
		return false, err
	}

	remainderHash, err := calcKindHash(recs[1:])
	if err != nil {
		return false, err
	}

	if firstRowHash != remainderHash {
		return true, nil
	}

	return false, nil
}

// Hash generates a hash from the kinds returned by
// the detectors. The detectors should already have
// sampled data.
//
// TODO: move Hash to pkg libsq/core/kind?
func Hash(detectors []*kind.Detector) (h string, err error) {
	if len(detectors) == 0 {
		return "", errz.New("no kind detectors")
	}

	kinds := make([]kind.Kind, len(detectors))
	for i := range detectors {
		kinds[i], _, err = detectors[i].Detect()
		if err != nil {
			return "", err
		}
	}

	// TODO: use an actual hash function
	hash := strings.Builder{}
	for i := range kinds {
		if i > 0 {
			hash.WriteRune('|')
		}
		hash.WriteString(kinds[i].String())
	}

	h = hash.String()
	return h, nil
}

func calcKindHash(recs [][]string) (string, error) {
	if len(recs) == 0 || len(recs[0]) == 0 {
		return "", errz.New("no records")
	}

	fieldCount := len(recs[0])

	detectors := make([]*kind.Detector, len(recs[0]))
	for i := 0; i < fieldCount; i++ {
		detectors[i] = kind.NewDetector()
	}

	for i := range recs {
		for j := range recs[i] {
			detectors[j].Sample(recs[i][j])
		}
	}

	return Hash(detectors)
}

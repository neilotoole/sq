package csv

import (
	"context"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
)

// hasHeaderRow returns true if a header row is explicitly
// set in opts, or if detectHeaderRow detects that the first
// row of recs seems to be a header.
func hasHeaderRow(ctx context.Context, recs [][]string, opts options.Options) (bool, error) {
	if driver.OptIngestHeader.IsSet(opts) {
		b := driver.OptIngestHeader.Get(opts)
		lg.FromContext(ctx).Debug("CSV ingest header explicitly specified: skipping header detection",
			lga.Key, driver.OptIngestHeader.Key(),
			lga.Val, b)
		return b, nil
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

func calcKindHash(recs [][]string) (string, error) {
	if len(recs) == 0 || len(recs[0]) == 0 {
		return "", errz.New("no records")
	}

	fieldCount := len(recs[0])

	detectors := make([]*kind.Detector, len(recs[0]))
	for i := range fieldCount {
		detectors[i] = kind.NewDetector()
	}

	for i := range recs {
		for j := range recs[i] {
			detectors[j].Sample(recs[i][j])
		}
	}

	return kind.Hash(detectors)
}

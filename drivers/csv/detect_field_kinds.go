package csv

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
)

// detectColKinds detects the kinds of recs' columns.
//
// TODO: Do we need to return []kind.MungeFunc?
func detectColKinds(recs [][]string) ([]kind.Kind, []kind.MungeFunc, error) { //nolint:unparam
	if len(recs) == 0 || len(recs[0]) == 0 {
		return nil, nil, errz.New("no records")
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

	kinds := make([]kind.Kind, fieldCount)
	mungers := make([]kind.MungeFunc, fieldCount)

	var err error
	for i := range detectors {
		kinds[i], mungers[i], err = detectors[i].Detect()
		if err != nil {
			return nil, nil, err
		}
	}

	return kinds, mungers, nil
}

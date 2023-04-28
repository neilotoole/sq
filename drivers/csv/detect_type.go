package csv

import (
	"context"
	"encoding/csv"
	"errors"
	"io"

	"github.com/neilotoole/sq/cli/output/csvw"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/source"
)

var (
	_ source.DriverDetectFunc = DetectCSV
	_ source.DriverDetectFunc = DetectTSV
)

// DetectCSV implements source.DriverDetectFunc.
func DetectCSV(ctx context.Context, openFn source.FileOpenFunc) (detected source.DriverType, score float32,
	err error,
) {
	return detectType(ctx, TypeCSV, openFn)
}

// DetectTSV implements source.DriverDetectFunc.
func DetectTSV(ctx context.Context, openFn source.FileOpenFunc) (detected source.DriverType,
	score float32, err error,
) {
	return detectType(ctx, TypeTSV, openFn)
}

func detectType(ctx context.Context, typ source.DriverType,
	openFn source.FileOpenFunc,
) (detected source.DriverType, score float32, err error) {
	log := lg.From(ctx)
	var r io.ReadCloser
	r, err = openFn()
	if err != nil {
		return source.TypeNone, 0, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	delim := csvw.Comma
	if typ == TypeTSV {
		delim = csvw.Tab
	}

	cr := csv.NewReader(&crFilterReader{r: r})
	cr.Comma = delim
	cr.FieldsPerRecord = -1

	score = isCSV(ctx, cr)
	if score > 0 {
		return typ, score, nil
	}

	return source.TypeNone, 0, nil
}

const (
	scoreNo       float32 = 0
	scoreMaybe    float32 = 0.1
	scoreProbably float32 = 0.2
	// scoreYes is less than 1.0 because other detectors
	// (e.g. XLSX) can be more confident.
	scoreYes float32 = 0.9
)

// isCSV returns a score indicating the
// the confidence that cr is reading legitimate CSV, where
// a score <= 0 is not CSV, a score >= 1 is definitely CSV.
func isCSV(ctx context.Context, cr *csv.Reader) (score float32) {
	const (
		maxRecords int = 100
	)

	var recordCount, totalFieldCount int
	var avgFields float32

	for i := 0; i < maxRecords; i++ {
		select {
		case <-ctx.Done():
			return 0
		default:
		}

		rec, err := cr.Read()
		if err != nil {
			if errors.Is(err, io.EOF) && rec == nil {
				// This means end of data
				break
			}

			// It's a genuine error
			return scoreNo
		}
		totalFieldCount += len(rec)
		recordCount++
	}

	if recordCount == 0 {
		return scoreNo
	}

	avgFields = float32(totalFieldCount) / float32(recordCount)

	if recordCount == 1 {
		if avgFields <= 2 {
			return scoreMaybe
		}
		return scoreProbably
	}

	// recordCount >= 2
	switch {
	case avgFields <= 1:
		return scoreMaybe
	case avgFields <= 2:
		return scoreProbably
	default:
		// avgFields > 2
		return scoreYes
	}
}

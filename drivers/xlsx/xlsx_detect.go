package xlsx

import (
	"context"
	"io"
	"slices"

	"github.com/neilotoole/sq/libsq/core/kind"

	"github.com/xuri/excelize/v2"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/source"
)

var _ source.DriverDetectFunc = DetectXLSX

// DetectXLSX implements source.DriverDetectFunc, returning
// TypeXLSX and a score of 1.0 if valid XLSX.
func DetectXLSX(ctx context.Context, openFn source.FileOpenFunc) (detected source.DriverType, score float32,
	err error,
) {
	log := lg.FromContext(ctx)
	var r io.ReadCloser
	r, err = openFn()
	if err != nil {
		return source.TypeNone, 0, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	f, err := excelize.OpenReader(r)
	if err != nil {
		return source.TypeNone, 0, nil
	}

	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, f)

	return Type, 1.0, nil
}

func detectHeaderRow(ctx context.Context, sheet *xSheet) (hasHeader bool, err error) {
	if len(sheet.sampleRows) < 2 {
		// If zero records, obviously no header row.
		// If one record... well, is there any way of determining if
		// it's a header row or not? Probably best to treat it as a data row.
		return false, nil
	}

	kinds1, _, err := detectSheetColumnKinds(ctx, sheet, 0)
	if err != nil {
		return false, err
	}
	kinds2, _, err := detectSheetColumnKinds(ctx, sheet, 1)
	if err != nil {
		return false, err
	}

	if len(kinds1) == len(kinds2) {
		return !slices.Equal(kinds1, kinds2), nil
	}

	// The rows differ in length (ragged edges). Unfortunately this does
	// happen in the real world, so we must deal with it.
	lg.FromContext(ctx).Warn("Excel sheet has ragged edges", laSheet, sheet.name)

	length := min(len(kinds1), len(kinds2))
	kinds1 = kinds1[:length]
	kinds2 = kinds2[:length]

	return !slices.Equal(kinds1, kinds2), nil
}

// determineSampleColumnTypes returns the xlsx cell types for the sheet.
// It is assumed that sheet.sampleRows is already loaded.
func determineSampleColumnTypes(ctx context.Context, sheet *xSheet, rangeStart, rangeEnd int) ([]excelize.CellType, error) {
	rows, err := sheet.file.Rows(sheet.name)
	if err != nil {
		return nil, errw(err)
	}

	defer lg.WarnIfCloseError(lg.FromContext(ctx), msgCloseSheetIter, rows)

	var (
		cols  []string
		typ   excelize.CellType
		types []excelize.CellType
	)

	for rowi := rangeStart; rowi < rangeEnd && rowi < len(sheet.sampleRows); rowi++ {
		if rowi < rangeStart {
			continue
		}

		cols = sheet.sampleRows[rowi]
		if len(cols) > len(types) {
			types2 := make([]excelize.CellType, len(cols))
			if types != nil {
				copy(types2, types)
			}
			types = types2
		}

		for coli := range cols {
			typ = sheet.sampleTypes[rowi][coli]

			if types[coli] == 0 {
				types[coli] = typ
				continue
			}

			// Else, it already has a type
			if types[coli] == typ {
				// type matches, just continue
				continue
			}

			// It already has a type, and it's different from this cell's type,
			// so we default to string.
			types[coli] = excelize.CellTypeSharedString
		}
	}

	return types, nil
}

// detectSheetColumnKinds calculates the lowest-common-denominator kind
// for the cells of rows. The returned slice will have length
// equal to the longest row.
func detectSheetColumnKinds(ctx context.Context, sheet *xSheet, rangeStart int) ([]kind.Kind, []kind.MungeFunc, error) {
	rows := sheet.sampleRows

	if rangeStart > len(rows) {
		// Shouldn't happen
		return nil, nil, errz.Errorf("sheet {%s} is empty", sheet.name)
	}

	var detectors []*kind.Detector

	for i := rangeStart; i < len(rows); i++ {
		if isEmptyRow(rows[i]) {
			continue
		}

		for j := len(detectors); j < len(rows[i]); j++ {
			detectors = append(detectors, kind.NewDetector())
		}

		for j := range rows[i] {

			val := rows[i][j]
			detectors[j].Sample(val)
		}
	}

	kinds := make([]kind.Kind, len(detectors))
	mungeFns := make([]kind.MungeFunc, len(detectors))
	var err error

	for j := range detectors {
		if kinds[j], mungeFns[j], err = detectors[j].Detect(); err != nil {
			return nil, nil, err
		}
	}

	return kinds, mungeFns, nil
}

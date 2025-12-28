package xlsx

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"slices"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

var _ files.TypeDetectFunc = DetectXLSX

// DetectXLSX implements files.TypeDetectFunc, returning
// TypeXLSX and a score of 1.0 if valid XLSX.
//
// Detection works by parsing the file as a ZIP archive and checking
// for the presence of the "xl/" directory, which is the hallmark of
// an XLSX (Office Open XML Spreadsheet) file. This approach is more
// reliable than magic number detection because XLSX files are ZIP
// archives, and different tools create them with varying internal
// structures that can confuse magic-number-based detection.
func DetectXLSX(ctx context.Context, newRdrFn files.NewReaderFunc) (detected drivertype.Type, score float32,
	err error,
) {
	log := lg.FromContext(ctx)
	var r io.ReadCloser
	r, err = newRdrFn(ctx)
	if err != nil {
		return drivertype.None, 0, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	data, err := io.ReadAll(r)
	if err != nil {
		return drivertype.None, 0, errz.Err(err)
	}

	if len(data) < 4 {
		return drivertype.None, 0, nil
	}

	// Quick check for ZIP signature before attempting to parse.
	// ZIP files start with "PK\x03\x04".
	if data[0] != 'P' || data[1] != 'K' || data[2] != 0x03 || data[3] != 0x04 {
		return drivertype.None, 0, nil
	}

	zipRdr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		// Not a valid ZIP file
		return drivertype.None, 0, nil
	}

	// XLSX files contain an "xl/" directory with workbook data.
	// Check if any entry starts with "xl/".
	for _, f := range zipRdr.File {
		if strings.HasPrefix(f.Name, "xl/") {
			return drivertype.XLSX, 1.0, nil
		}
	}

	return drivertype.None, 0, nil
}

func detectHeaderRow(ctx context.Context, sheet *xSheet) (hasHeader bool, err error) {
	if len(sheet.sampleRows) < 2 {
		// If zero records, obviously no header row.
		// If one record... well, is there any way of determining if
		// it's a header row or not? Probably best to treat it as a data row.
		return false, nil
	}

	kinds1, _, err := detectSheetColumnKinds(sheet, 0)
	if err != nil {
		return false, err
	}
	kinds2, _, err := detectSheetColumnKinds(sheet, 1)
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

// detectSheetColumnKinds calculates the lowest-common-denominator kind
// for the columns of sheet. It also returns munge funcs for ingesting
// each column's data (the munge func may be nil for any column).
func detectSheetColumnKinds(sheet *xSheet, rangeStart int) ([]kind.Kind, []kind.MungeFunc, error) {
	rows := sheet.sampleRows

	if rangeStart > len(rows) {
		// Shouldn't happen
		return nil, nil, errz.Errorf("excel: sheet {%s} is empty", sheet.name)
	}

	var detectors []*kind.Detector

	for i := rangeStart; i < len(rows); i++ {
		if langz.IsSliceZeroed(rows[i]) {
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

package xlsx

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/tealeg/xlsx/v2"
	"golang.org/x/exp/slices"
)

func detectHeaderRow(sheet *xlsx.Sheet) (hasHeader bool, err error) {
	if len(sheet.Rows) < 2 {
		// If zero records, obviously no header row.
		// If one record... well, is there any way of determining if
		// it's a header row or not? Probably best to treat it as a data row.
		return false, nil
	}

	types1 := getCellColumnTypes(sheet, true)
	types2 := getCellColumnTypes(sheet, false)

	if len(types1) != len(types2) {
		// Can this happen?
		return false, errz.Errorf("sheet {%s} has ragged edges", sheet.Name)
	}

	if slices.Equal(types1, types2) {
		return false, nil
	}

	return true, nil
}

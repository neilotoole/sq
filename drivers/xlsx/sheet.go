package xlsx

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/xuri/excelize/v2"
)

const msgCloseSheetIter = "Close Excel sheet iterator"

type sheetIter struct {
	file *excelize.File
	name string
	rows *excelize.Rows
	rowi int
}

func newSheetIter(file *excelize.File, sheetName string) (*sheetIter, error) {
	rows, err := file.Rows(sheetName)
	if err != nil {
		return nil, errw(err)
	}

	return &sheetIter{
		file: file,
		name: sheetName,
		rows: rows,
		rowi: -1,
	}, nil
}

func (si *sheetIter) Close() error {
	return errw(si.rows.Close())
}

func (si *sheetIter) Next() bool {
	b := si.rows.Next()
	if b {
		si.rowi++
	}

	return b
}

// Count returns the row index of the iterator.
func (si *sheetIter) Count() int {
	return si.rowi
}

// Error returns any error encountered by the iterator.
func (si *sheetIter) Error() error {
	return si.rows.Error()
}

// Row returns next row as []string, as well as the type of each cell.
func (si *sheetIter) Row() (cols, vals []string, types []excelize.CellType, styles []int, err error) {
	if si.rowi < 0 {
		return nil, nil, nil, nil, errz.New("excel: sheet iterator: must call Next before Row")
	}

	cols, err = si.rows.Columns(excelize.Options{RawCellValue: false})
	if err != nil {
		return nil, nil, nil, styles, errw(err)
	}

	types = make([]excelize.CellType, len(cols))
	styles = make([]int, len(cols))
	vals = make([]string, len(cols))

	var cell string
	for i := range cols {
		cell = cellName(i, si.rowi)

		if vals[i], err = si.file.GetCellValue(si.name, cell, excelize.Options{RawCellValue: false}); err != nil {
			return nil, nil, nil, nil, errw(err)
		}

		types[i], err = si.file.GetCellType(si.name, cell)
		if err != nil {
			return nil, nil, nil, nil, errw(err)
		}

		if styles[i], err = si.file.GetCellStyle(si.name, cell); err != nil {
			return nil, nil, nil, nil, errw(err)
		}

	}

	// convertRowDates(context.Background(), si.file, si.name, si.rowi, vals)
	// convertDates(context.Background(), si.file, si.name, [][]string{vals})

	return cols, vals, types, styles, nil
}

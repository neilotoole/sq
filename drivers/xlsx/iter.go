package xlsx

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/xuri/excelize/v2"
)

const msgCloseRowIter = "Close Excel row iterator"

// rowIter wraps excelize.Rows. Originally the iter had more functionality,
// but has since been slimmed down. It's possible that we may get rid
// of it entirely and use excelize.Rows directly.
type rowIter struct {
	file *excelize.File
	name string
	rows *excelize.Rows
	rowi int
}

func newRowIter(file *excelize.File, sheetName string) (*rowIter, error) {
	rows, err := file.Rows(sheetName)
	if err != nil {
		return nil, errw(err)
	}

	return &rowIter{
		file: file,
		name: sheetName,
		rows: rows,
		rowi: -1,
	}, nil
}

func (ri *rowIter) Close() error {
	return errw(ri.rows.Close())
}

func (ri *rowIter) Next() bool {
	b := ri.rows.Next()
	if b {
		ri.rowi++
	}

	return b
}

// Count returns the row index of the iterator.
func (ri *rowIter) Count() int {
	return ri.rowi
}

// Error returns any error encountered by the iterator.
func (ri *rowIter) Error() error {
	return ri.rows.Error()
}

// Row returns next row as []string, as well as the type of each cell.
func (ri *rowIter) Row() (cols, vals []string, types []excelize.CellType, styles []int, err error) {
	if ri.rowi < 0 {
		return nil, nil, nil, nil, errz.New("excel: sheet iterator: must call Next before Row")
	}

	cols, err = ri.rows.Columns(excelize.Options{RawCellValue: false})
	if err != nil {
		return nil, nil, nil, styles, errw(err)
	}

	types = make([]excelize.CellType, len(cols))
	styles = make([]int, len(cols))
	vals = make([]string, len(cols))

	var cell string
	for i := range cols {
		cell = cellName(i, ri.rowi)

		if vals[i], err = ri.file.GetCellValue(ri.name, cell, excelize.Options{RawCellValue: false}); err != nil {
			return nil, nil, nil, nil, errw(err)
		}

		types[i], err = ri.file.GetCellType(ri.name, cell)
		if err != nil {
			return nil, nil, nil, nil, errw(err)
		}

		if styles[i], err = ri.file.GetCellStyle(ri.name, cell); err != nil {
			return nil, nil, nil, nil, errw(err)
		}

	}

	return cols, vals, types, styles, nil
}

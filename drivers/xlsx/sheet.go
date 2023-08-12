package xlsx

import "github.com/xuri/excelize/v2"

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
	}, nil
}

func (si *sheetIter) Close() error {
	return errw(si.rows.Close())
}

func (si *sheetIter) Next() bool {
	return si.rows.Next()
}

// Count returns the row index of the iterator.
func (si *sheetIter) Count() int {
	return si.rowi
}

// Row returns next row as []string, as well as the type of each cell.
func (si *sheetIter) Row() ([]string, []excelize.CellType, error) {
	cols, err := si.rows.Columns()
	if err != nil {
		return nil, nil, errw(err)
	}

	types := make([]excelize.CellType, len(cols))

	var name string
	for i := range cols {
		name = cellName(i, si.rowi)
		types[i], err = si.file.GetCellType(si.name, name)
		if err != nil {
			return nil, nil, errw(err)
		}
	}

	si.rowi++
	return cols, types, nil
}

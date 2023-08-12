package xlsx

import (
	"fmt"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/tealeg/xlsx/v2"
)

func rowToRecord(log *slog.Logger, destColKinds []kind.Kind, row *xlsx.Row, sheetName string, rowIndex int) []any {
	vals := make([]any, len(destColKinds))
	for j, cell := range row.Cells {
		if j >= len(vals) {
			// log.Warn("Sheet %s[%d:%d]: skipping additional cells because there's more cells than expected (%d)",
			//	sheetName, rowIndex, j, len(destColKinds))
			continue
		}

		typ := cell.Type()
		switch typ { //nolint:exhaustive
		case xlsx.CellTypeBool:
			vals[j] = cell.Bool()
		case xlsx.CellTypeNumeric:
			if cell.IsTime() {
				t, err := cell.GetTime(false)
				if err != nil {
					// log.Warn("Sheet %s[%d:%d]: failed to get Excel time: %v", sheetName, rowIndex, j, err)
					vals[j] = nil
					continue
				}

				vals[j] = t
				continue
			}

			intVal, err := cell.Int64()
			if err == nil {
				vals[j] = intVal
				continue
			}
			floatVal, err := cell.Float()
			if err == nil {
				vals[j] = floatVal
				continue
			}

			if cell.Value == "" {
				vals[j] = nil
				continue
			}

			// it's not an int, it's not a float, it's not empty string;
			// just give up and make it a string.
			log.Warn("Failed to determine type of numeric cell",
				"sheet", sheetName,
				"cell", fmt.Sprintf("%d:%d", rowIndex, j),
				lga.Val, cell.Value,
			)

			vals[j] = cell.Value
			// FIXME: prob should return an error here?
		case xlsx.CellTypeString:
			if cell.Value == "" {
				if destColKinds[j] != kind.Text {
					vals[j] = nil
					continue
				}
			}

			vals[j] = cell.String()
		case xlsx.CellTypeDate:
			// TODO: parse into a time value here
			vals[j] = cell.Value
		default:
			if cell.Value == "" {
				vals[j] = nil
			} else {
				vals[j] = cell.Value
			}
		}
	}
	return vals
}

// isEmptyRow returns true if row is nil or has zero cells, or if
// every cell value is empty string.
func isEmptyRow(row *xlsx.Row) bool {
	if row == nil || len(row.Cells) == 0 {
		return true
	}

	for i := range row.Cells {
		if row.Cells[i].Value != "" {
			return false
		}
	}

	return true
}

// readCellValue reads the value of a cell, returning a value of
// type that most matches the sq kind.
func readCellValue(cell *xlsx.Cell) any {
	if cell == nil || cell.Value == "" {
		return nil
	}

	var val any

	switch cell.Type() { //nolint:exhaustive
	case xlsx.CellTypeBool:
		val = cell.Bool()
		return val
	case xlsx.CellTypeNumeric:
		if cell.IsTime() {
			t, err := cell.GetTime(false)
			if err == nil {
				return t
			}

			t, err = cell.GetTime(true)
			if err == nil {
				return t
			}

			// Otherwise we have an error, just return the value
			val, _ = cell.FormattedValue()
			return val
		}

		intVal, err := cell.Int64()
		if err == nil {
			val = intVal
			return val
		}

		floatVal, err := cell.Float()
		if err == nil {
			val = floatVal
			return val
		}

		val, _ = cell.FormattedValue()
		return val

	case xlsx.CellTypeString:
		val = cell.String()
	case xlsx.CellTypeDate:
		// TODO: parse into a time.Time value here?
		val, _ = cell.FormattedValue()
	default:
		val, _ = cell.FormattedValue()
	}

	return val
}

// getCellColumnTypes returns the xlsx cell types for the sheet, determined from
// the values of the first data row (after any header row).
func getCellColumnTypes(sheet *xlsx.Sheet, hasHeader bool) []xlsx.CellType {
	types := make([]*xlsx.CellType, getRowsMaxCellCount(sheet))
	firstDataRow := 0
	if hasHeader {
		firstDataRow = 1
	}

	for x := firstDataRow; x < len(sheet.Rows); x++ {
		for i, cell := range sheet.Rows[x].Cells {
			if types[i] == nil {
				typ := cell.Type()
				types[i] = &typ
				continue
			}

			// else, it already has a type
			if *types[i] == cell.Type() {
				// type matches, just continue
				continue
			}

			// it already has a type, and it's different from this cell's type
			typ := xlsx.CellTypeString
			types[i] = &typ
		}
	}

	// convert back to value types
	ret := make([]xlsx.CellType, len(types))
	for i, typ := range types {
		ret[i] = *typ
	}

	return ret
}

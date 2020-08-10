package xlsx

import (
	"context"
	"strings"
	"time"

	"github.com/tealeg/xlsx/v2"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/options"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/sqlz"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/sqlmodel"
	"github.com/neilotoole/sq/libsq/stringz"
)

// xlsxToScratch loads the data in xlFile into scratchDB.
func xlsxToScratch(ctx context.Context, log lg.Log, src *source.Source, xlFile *xlsx.File, scratchDB driver.Database) error {
	start := time.Now()
	log.Debugf("Beginning import from XLSX %s to %s (%s)...", src.Handle, scratchDB.Source().Handle, scratchDB.Source().RedactedLocation())

	hasHeader, _, err := options.HasHeader(src.Options)
	if err != nil {
		return err
	}

	tblDefs, err := buildTblDefsForSheets(ctx, log, xlFile.Sheets, hasHeader)
	if err != nil {
		return err
	}

	for _, tblDef := range tblDefs {
		err = scratchDB.SQLDriver().CreateTable(ctx, scratchDB.DB(), tblDef)
		if err != nil {
			return err
		}
	}

	log.Debugf("%d tables created (but not yet populated) in %s in %s",
		len(tblDefs), scratchDB.Source().Handle, time.Since(start))

	for i := range xlFile.Sheets {
		err = importSheetToTable(ctx, log, xlFile.Sheets[i], hasHeader, scratchDB, tblDefs[i])
		if err != nil {
			return err
		}
	}

	log.Debugf("%d sheets imported from %s to %s in %s",
		len(xlFile.Sheets), src.Handle, scratchDB.Source().Handle, time.Since(start))

	return nil
}

// importSheetToTable imports sheet's data to its scratch table.
// The scratch table must already exist.
func importSheetToTable(ctx context.Context, log lg.Log, sheet *xlsx.Sheet, hasHeader bool, scratchDB driver.Database, tblDef *sqlmodel.TableDef) error {
	startTime := time.Now()

	stmtExecer, err := scratchDB.SQLDriver().PrepareInsertStmt(ctx, scratchDB.DB(), tblDef.Name, tblDef.ColNames(), 1)
	if err != nil {
		return err
	}

	destColKinds := tblDef.ColKinds()

	for i, row := range sheet.Rows {
		if hasHeader && i == 0 {
			continue
		}

		rec := rowToRecord(log, destColKinds, row, sheet.Name, i)
		err = stmtExecer.Munge(rec)
		if err != nil {
			return err
		}

		_, err = stmtExecer.Exec(ctx, rec...)
		if err != nil {
			log.WarnIfCloseError(stmtExecer)
			return errz.Err(err)
		}
	}

	err = stmtExecer.Close()
	if err != nil {
		return errz.Err(err)
	}

	log.Debugf("Inserted %d rows from sheet %q into %s.%s in %s",
		len(sheet.Rows), sheet.Name, scratchDB.Source().Handle, tblDef.Name, time.Since(startTime))

	return nil
}

// buildTblDefsForSheets returns a TableDef for each sheet.
func buildTblDefsForSheets(ctx context.Context, log lg.Log, sheets []*xlsx.Sheet, hasHeader bool) ([]*sqlmodel.TableDef, error) {
	tblDefs := make([]*sqlmodel.TableDef, len(sheets))

	g, _ := errgroup.WithContext(ctx)
	for i := range sheets {
		i := i
		g.Go(func() error {
			tblDef, err := buildTblDefForSheet(log, sheets[i], hasHeader)
			if err != nil {
				return err
			}
			tblDefs[i] = tblDef
			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		return nil, err
	}

	return tblDefs, nil
}

// buildTblDefForSheet creates a table for the given sheet, and returns
// a model of the table, or an error.
func buildTblDefForSheet(log lg.Log, sheet *xlsx.Sheet, hasHeader bool) (*sqlmodel.TableDef, error) {
	numCells := getRowsMaxCellCount(sheet)
	if numCells == 0 {
		return nil, errz.Errorf("sheet %q has no columns", sheet.Name)
	}

	colNames := make([]string, numCells)
	colKinds := make([]sqlz.Kind, numCells)

	firstDataRow := 0

	if len(sheet.Rows) == 0 {
		// TODO: is this even reachable? That is, if sheet.Rows is empty,
		//  then sheet.cols (checked for above) will also be empty?

		// sheet has no rows
		for i := 0; i < numCells; i++ {
			colKinds[i] = sqlz.KindText
			colNames[i] = stringz.GenerateAlphaColName(i)
		}
	} else {
		// sheet is non-empty

		// Set up the column names
		if hasHeader {
			firstDataRow = 1
			headerCells := sheet.Rows[0].Cells
			for i := 0; i < numCells; i++ {
				colNames[i] = headerCells[i].Value
			}
		} else {
			for i := 0; i < numCells; i++ {
				colNames[i] = stringz.GenerateAlphaColName(i)
			}
		}

		// Set up the column types
		if firstDataRow >= len(sheet.Rows) {
			// the sheet contains only one row (the header row). Let's
			// explicitly set the column type none the less
			for i := 0; i < numCells; i++ {
				colKinds[i] = sqlz.KindText
			}
		} else {
			// we have at least one data row, let's get the column types
			colKinds = getKindsFromCells(sheet.Rows[firstDataRow].Cells)
		}
	}

	tblDef := &sqlmodel.TableDef{Name: sheet.Name}
	cols := make([]*sqlmodel.ColDef, len(colNames))
	for i, colName := range colNames {
		cols[i] = &sqlmodel.ColDef{Table: tblDef, Name: colName, Kind: colKinds[i]}
	}
	tblDef.Cols = cols
	log.Debugf("sheet %q: using col names [%q]", sheet.Name, strings.Join(colNames, ", "))

	return tblDef, nil
}

func rowToRecord(log lg.Log, destColKinds []sqlz.Kind, row *xlsx.Row, sheetName string, rowIndex int) []interface{} {
	vals := make([]interface{}, len(row.Cells))
	for j, cell := range row.Cells {
		typ := cell.Type()
		switch typ {
		case xlsx.CellTypeBool:
			vals[j] = cell.Bool()
		case xlsx.CellTypeNumeric:
			if cell.IsTime() {
				t, err := cell.GetTime(false)
				if err != nil {
					log.Warnf("sheet %s[%d:%d]: failed to get Excel time: %v", sheetName, rowIndex, j, err)
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
			log.Warnf("Failed to determine type of numeric cell [%s:%d:%d] from value: %q", sheetName, rowIndex, j, cell.Value)
			vals[j] = cell.Value
			// FIXME: prob should return an error here?
		case xlsx.CellTypeString:
			if cell.Value == "" {
				if destColKinds[j] != sqlz.KindText {
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

func getKindsFromCells(cells []*xlsx.Cell) []sqlz.Kind {
	vals := make([]sqlz.Kind, len(cells))
	for i, cell := range cells {
		typ := cell.Type()
		switch typ {
		case xlsx.CellTypeBool:
			vals[i] = sqlz.KindBool
		case xlsx.CellTypeNumeric:
			if cell.IsTime() {
				vals[i] = sqlz.KindDatetime
				continue
			}

			_, err := cell.Int64()
			if err == nil {
				vals[i] = sqlz.KindInt
				continue
			}
			_, err = cell.Float()
			if err == nil {
				vals[i] = sqlz.KindFloat
				continue
			}
			// it's not an int, it's not a float
			vals[i] = sqlz.KindDecimal

		case xlsx.CellTypeDate:
			// TODO: support time values here?
			vals[i] = sqlz.KindDatetime

		default:
			vals[i] = sqlz.KindText
		}
	}

	return vals
}

// getColNames returns column names for the sheet. If hasHeader is true and there's
// at least one row, the column names are the values of the first row. Otherwise
// an alphabetical sequence (A, B... Z, AA, AB) is generated.
func getColNames(sheet *xlsx.Sheet, hasHeader bool) []string {
	numCells := getRowsMaxCellCount(sheet)
	colNames := make([]string, numCells)

	if len(sheet.Rows) > 0 && hasHeader {
		row := sheet.Rows[0]
		for i := 0; i < numCells; i++ {
			colNames[i] = row.Cells[i].Value
		}
		return colNames
	}

	for i := 0; i < numCells; i++ {
		colNames[i] = stringz.GenerateAlphaColName(i)
	}

	return colNames
}

// getColTypes returns the xlsx column types for the sheet, determined from
// the values of the first data row (after any header row).
func getColTypes(sheet *xlsx.Sheet, hasHeader bool) []xlsx.CellType {
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

func cellTypeToString(typ xlsx.CellType) string {
	switch typ {
	case xlsx.CellTypeString:
		return "string"
	case xlsx.CellTypeStringFormula:
		return "formula"
	case xlsx.CellTypeNumeric:
		return "numeric"
	case xlsx.CellTypeBool:
		return "bool"
	case xlsx.CellTypeInline:
		return "inline"
	case xlsx.CellTypeError:
		return "error"
	case xlsx.CellTypeDate:
		return "date"
	}
	return "general"
}

// getRowsMaxCellCount returns the largest count of cells in
// in the rows of sheet.
func getRowsMaxCellCount(sheet *xlsx.Sheet) int {
	max := 0

	for _, row := range sheet.Rows {
		if len(row.Cells) > max {
			max = len(row.Cells)
		}
	}

	return max
}

package xlsx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"golang.org/x/exp/slog"

	"github.com/tealeg/xlsx/v2"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
)

// xlsxToScratch loads the data in xlFile into scratchDB.
func xlsxToScratch(ctx context.Context, src *source.Source, xlFile *xlsx.File, scratchDB driver.Database) error {
	log := lg.FromContext(ctx)
	start := time.Now()
	log.Debug("Beginning import from XLSX",
		lga.Src, src,
		lga.Target, scratchDB.Source())

	hasHeader := driver.OptIngestHeader.Get(src.Options)

	// TODO: Like the csv driver, the xlsx driver should detect
	// the presence of a header.
	tblDefs, err := buildTblDefsForSheets(ctx, xlFile.Sheets, hasHeader)
	if err != nil {
		return err
	}

	for _, tblDef := range tblDefs {
		if tblDef == nil {
			// tblDef can be nil if its sheet is empty (has no data).
			continue
		}
		err = scratchDB.SQLDriver().CreateTable(ctx, scratchDB.DB(), tblDef)
		if err != nil {
			return err
		}
	}

	log.Debug("Tables created (but not yet populated)",
		lga.Count, len(tblDefs),
		lga.Target, scratchDB.Source(),
		lga.Elapsed, time.Since(start))

	var imported, skipped int

	for i := range xlFile.Sheets {
		if tblDefs[i] == nil {
			// tblDef can be nil if its sheet is empty (has no data).
			skipped++
			continue
		}
		err = importSheetToTable(ctx, xlFile.Sheets[i], hasHeader, scratchDB, tblDefs[i])
		if err != nil {
			return err
		}
		imported++
	}

	log.Debug("Sheets imported",
		lga.Count, imported,
		"skipped", skipped,
		lga.From, src,
		lga.To, scratchDB.Source(),
		lga.Elapsed, time.Since(start),
	)

	return nil
}

// importSheetToTable imports sheet's data to its scratch table.
// The scratch table must already exist.
func importSheetToTable(ctx context.Context, sheet *xlsx.Sheet, hasHeader bool,
	scratchDB driver.Database, tblDef *sqlmodel.TableDef,
) error {
	log := lg.FromContext(ctx)
	startTime := time.Now()

	conn, err := scratchDB.DB().Conn(ctx)
	if err != nil {
		return errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDB, conn)

	drvr := scratchDB.SQLDriver()

	destColKinds := tblDef.ColKinds()

	batchSize := driver.MaxBatchRows(drvr, len(destColKinds))
	bi, err := driver.NewBatchInsert(ctx, drvr, conn, tblDef.Name, tblDef.ColNames(), batchSize)
	if err != nil {
		return err
	}

	for i, row := range sheet.Rows {
		if hasHeader && i == 0 {
			continue
		}

		if isEmptyRow(row) {
			continue
		}

		rec := rowToRecord(log, destColKinds, row, sheet.Name, i)
		err = bi.Munge(rec)
		if err != nil {
			close(bi.RecordCh)
			return err
		}

		select {
		case <-ctx.Done():
			close(bi.RecordCh)
			return ctx.Err()
		case err = <-bi.ErrCh:
			if err != nil {
				close(bi.RecordCh)
				return err
			}

			// The batch inserter successfully completed
			break
		case bi.RecordCh <- rec:
		}
	}

	close(bi.RecordCh) // Indicate that we're finished writing records

	err = <-bi.ErrCh // Wait for bi to complete
	if err != nil {
		return err
	}

	log.Debug("Inserted rows from sheet into table",
		lga.Count, bi.Written(),
		"sheet", sheet.Name,
		lga.Target, source.Target(scratchDB.Source(), tblDef.Name),
		lga.Elapsed, time.Since(startTime))

	return nil
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

// buildTblDefsForSheets returns a TableDef for each sheet. If the
// sheet is empty (has no data), the TableDef for that sheet will be nil.
func buildTblDefsForSheets(ctx context.Context, sheets []*xlsx.Sheet, hasHeader bool) ([]*sqlmodel.TableDef, error) {
	tblDefs := make([]*sqlmodel.TableDef, len(sheets))

	g, gCtx := errgroup.WithContext(ctx)
	for i := range sheets {
		i := i
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
			}

			tblDef, err := buildTblDefForSheet(gCtx, sheets[i], hasHeader)
			if err != nil {
				return err
			}
			tblDefs[i] = tblDef
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return tblDefs, nil
}

// buildTblDefForSheet creates a table for the given sheet, and returns
// a model of the table, or an error. If the sheet is empty, (nil,nil)
// is returned.
func buildTblDefForSheet(ctx context.Context, sheet *xlsx.Sheet, hasHeader bool) (*sqlmodel.TableDef, error) {
	log := lg.FromContext(ctx)
	maxCols := getRowsMaxCellCount(sheet)
	if maxCols == 0 {
		log.Warn("sheet is empty: skipping", "sheet", sheet.Name)
		return nil, nil //nolint:nilnil
	}

	colNames := make([]string, maxCols)
	colKinds := make([]kind.Kind, maxCols)

	firstDataRow := 0
	if len(sheet.Rows) == 0 {
		// TODO: is this even reachable? That is, if sheet.Rows is empty,
		//  then sheet.cols (checked for above) will also be empty?

		// sheet has no rows
		for i := 0; i < maxCols; i++ {
			colKinds[i] = kind.Text
			colNames[i] = stringz.GenerateAlphaColName(i, false)
		}
	} else {
		// sheet is non-empty

		// Set up the column names
		if hasHeader {
			firstDataRow = 1
			headerCells := sheet.Rows[0].Cells
			for i := 0; i < len(headerCells); i++ {
				colNames[i] = headerCells[i].Value
			}
		} else {
			for i := 0; i < maxCols; i++ {
				colNames[i] = stringz.GenerateAlphaColName(i, false)
			}
		}

		// Set up the column types
		if firstDataRow >= len(sheet.Rows) {
			// the sheet contains only one row (the header row). Let's
			// explicitly set the column type nonetheless
			for i := 0; i < maxCols; i++ {
				colKinds[i] = kind.Text
			}
		} else {
			// we have at least one data row, let's get the column types
			var err error
			colKinds, err = calcKindsForRows(firstDataRow, sheet.Rows)
			if err != nil {
				return nil, err
			}
		}
	}

	colNames, colKinds = syncColNamesKinds(colNames, colKinds)

	var err error
	if colNames, err = driver.MungeIngestColNames(ctx, colNames); err != nil {
		return nil, err
	}

	tblDef := &sqlmodel.TableDef{Name: sheet.Name}
	cols := make([]*sqlmodel.ColDef, len(colNames))
	for i, colName := range colNames {
		cols[i] = &sqlmodel.ColDef{Table: tblDef, Name: colName, Kind: colKinds[i]}
	}
	tblDef.Cols = cols
	log.Debug("Built table def",
		"sheet", sheet.Name,
		"cols", strings.Join(colNames, ", "))

	return tblDef, nil
}

// syncColNamesKinds ensures that column names and kinds are in
// a working state vis-a-vis each other. Notably if a colName is
// empty and its equivalent kind is kind.Null, that element
// is filtered out.
func syncColNamesKinds(colNames []string, colKinds []kind.Kind) (names []string, kinds []kind.Kind) {
	// Allow for the case of "phantom" columns. That is,
	// columns with entirely empty data.
	// Note: not sure if this scenario is now reachable
	if len(colKinds) < len(colNames) {
		colNames = colNames[0:len(colKinds)]
	}

	for i := range colNames {
		// Filter out the case where the column name is empty
		// and the kind is kind.Null or kind.Unknown.
		if colNames[i] == "" && (colKinds[i] == kind.Null || colKinds[i] == kind.Unknown) {
			continue
		}

		names = append(names, colNames[i])
		kinds = append(kinds, colKinds[i])
	}

	colNames = names
	colKinds = kinds

	// Check that we don't have any unnamed columns (empty header)
	for i := 0; i < len(colNames); i++ {
		if colNames[i] == "" {
			// Empty col name... possibly we should just throw
			// an error, but instead we'll try to generate a col name.
			colName := stringz.GenerateAlphaColName(i, false)
			for stringz.InSlice(colNames[0:i], colName) {
				// If colName already exists, just append an
				// underscore and try again.
				colName += "_"
			}
			colNames[i] = colName
		}
	}

	for i := range colKinds {
		if colKinds[i] == kind.Null || colKinds[i] == kind.Unknown {
			colKinds[i] = kind.Text
		}
	}

	return colNames, colKinds
}

func rowToRecord(log *slog.Logger, destColKinds []kind.Kind, row *xlsx.Row, sheetName string, rowIndex int) []any {
	vals := make([]any, len(destColKinds))
	for j, cell := range row.Cells {
		if j >= len(vals) {
			log.Warn("Sheet %s[%d:%d]: skipping additional cells because there's more cells than expected (%d)",
				sheetName, rowIndex, j, len(destColKinds))
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
					log.Warn("Sheet %s[%d:%d]: failed to get Excel time: %v", sheetName, rowIndex, j, err)
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

// calcKindsForRows calculates the lowest-common-denominator kind
// for the cells of rows. The returned slice will have length
// equal to the longest row.
func calcKindsForRows(firstDataRow int, rows []*xlsx.Row) ([]kind.Kind, error) {
	if firstDataRow > len(rows) {
		return nil, errz.Errorf("rows are empty")
	}

	var detectors []*kind.Detector

	for i := firstDataRow; i < len(rows); i++ {
		if isEmptyRow(rows[i]) {
			continue
		}

		for j := len(detectors); j < len(rows[i].Cells); j++ {
			detectors = append(detectors, kind.NewDetector())
		}

		for j := range rows[i].Cells {
			val := readCellValue(rows[i].Cells[j])
			detectors[j].Sample(val)
		}
	}

	kinds := make([]kind.Kind, len(detectors))

	for j := range detectors {
		knd, _, err := detectors[j].Detect()
		if err != nil {
			return nil, err
		}

		kinds[j] = knd
	}

	return kinds, nil
}

// getColNames returns column names for the sheet. If hasHeader is true and there's
// at least one row, the column names are the values of the first row. Otherwise
// an alphabetical sequence (A, B... Z, AA, AB) is generated.
func getColNames(sheet *xlsx.Sheet, hasHeader bool) []string {
	numCells := getRowsMaxCellCount(sheet)
	colNames := make([]string, numCells)

	if len(sheet.Rows) > 0 && hasHeader {
		row := sheet.Rows[0]
		for i := 0; i < len(row.Cells); i++ {
			colNames[i] = row.Cells[i].String()
		}
	}

	for i := range colNames {
		if colNames[i] == "" {
			colNames[i] = stringz.GenerateAlphaColName(i, false)
		}
	}

	return colNames
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
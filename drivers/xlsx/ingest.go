package xlsx

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"

	"golang.org/x/exp/slices"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
)

type xSheet struct {
	file        *excelize.File
	name        string
	sampleRows  [][]string
	sampleTypes [][]excelize.CellType
	maxCols     int
	rowsOnce    sync.Once
	rowsErr     error
}

func errw(err error) error {
	return errz.Wrap(err, "excel")
}

// cellName accepts zero-index cell coordinates, and returns the call name.
// For example, {0,0} returns "A1".
func cellName(col, row int) string {
	s, _ := excelize.ColumnNumberToName(col + 1)
	s += strconv.Itoa(row + 1)
	return s
}

func (xs *xSheet) loadSampleRows(ctx context.Context, sampleSize int) error {
	si, err := newSheetIter(xs.file, xs.name)
	if err != nil {
		return err
	}

	defer lg.WarnIfCloseError(lg.FromContext(ctx), msgCloseSheetIter, si)

	for si.Next() && si.Count() <= sampleSize {
		var cells []string
		var types []excelize.CellType

		if cells, types, err = si.Row(); err != nil {
			return err
		}

		xs.sampleRows = append(xs.sampleRows, cells)
		xs.sampleTypes = append(xs.sampleTypes, types)
		if len(cells) > xs.maxCols {
			xs.maxCols = len(cells)
		}
	}

	return nil
}

func hasSheet(xlFile *excelize.File, sheetName string) bool {
	return slices.Contains(xlFile.GetSheetList(), sheetName)
}

// ingestXLSX loads the data in xlFile into scratchDB.
// If includeSheetNames is non-empty, only the named sheets are ingested.
func ingestXLSX(ctx context.Context, src *source.Source, scratchDB driver.Database,
	xlFile *excelize.File, includeSheetNames []string,
) error {
	log := lg.FromContext(ctx)
	start := time.Now()
	log.Debug("Beginning import from XLSX",
		lga.Src, src,
		lga.Target, scratchDB.Source())

	var sheets []*xSheet
	if len(includeSheetNames) > 0 {
		for _, sheetName := range includeSheetNames {
			if !hasSheet(xlFile, sheetName) {
				return errz.Errorf("sheet {%s} not found", sheetName)
			}
			sheets = append(sheets, &xSheet{file: xlFile, name: sheetName})
		}
	} else {
		sheetNames := xlFile.GetSheetList()
		sheets = make([]*xSheet, len(sheetNames))
		for i := range sheetNames {
			sheets[i] = &xSheet{file: xlFile, name: sheetNames[i]}
		}
	}

	srcIngestHeader := getSrcIngestHeader(src.Options)
	sheetTbls, err := buildSheetTables(ctx, srcIngestHeader, sheets)
	if err != nil {
		return err
	}

	for _, sheetTbl := range sheetTbls {
		if sheetTbl == nil {
			// tblDef can be nil if its sheet is empty (has no data).
			continue
		}

		var db *sql.DB
		if db, err = scratchDB.DB(ctx); err != nil {
			return err
		}

		if err = scratchDB.SQLDriver().CreateTable(ctx, db, sheetTbl.def); err != nil {
			return err
		}
	}

	log.Debug("Tables created (but not yet populated)",
		lga.Count, len(sheetTbls),
		lga.Target, scratchDB.Source(),
		lga.Elapsed, time.Since(start))

	var imported, skipped int

	for i := range sheetTbls {
		if sheetTbls[i] == nil {
			// tblDef can be nil if its sheet is empty (has no data).
			skipped++
			continue
		}

		if err = importSheetToTable(ctx, scratchDB, sheetTbls[i]); err != nil {
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

// importSheetToTable imports the sheet data into the appropriate table
// in scratchDB. The scratch table must already exist.
func importSheetToTable(ctx context.Context, scratchDB driver.Database, sheetTbl *sheetTable) error {
	var (
		log       = lg.FromContext(ctx)
		startTime = time.Now()
		sheet     = sheetTbl.sheet
		hasHeader = sheetTbl.hasHeaderRow
		tblDef    = sheetTbl.def
	)

	db, err := scratchDB.DB(ctx)
	if err != nil {
		return err
	}

	conn, err := db.Conn(ctx)
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

	si, err := newSheetIter(sheetTbl.sheet.file, sheetTbl.sheet.name)
	if err != nil {
		return errw(err)
	}

	defer lg.WarnIfCloseError(log, msgCloseSheetIter, si)

	// for i, row := range sheet.rows {

	var cells []string
	var cellTypes []excelize.CellType

	i := -1
	for si.Next() {
		i++
		if hasHeader && i == 0 {
			continue
		}

		cells, cellTypes, err = si.Row()
		if err != nil {
			close(bi.RecordCh)
			return err
		}

		if isEmptyRow(cells) {
			continue
		}

		rec := rowToRecord(ctx, destColKinds, sheet.name, i, cells, cellTypes)
		if err = bi.Munge(rec); err != nil {
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
		laSheet, sheet.name,
		lga.Target, source.Target(scratchDB.Source(), tblDef.Name),
		lga.Elapsed, time.Since(startTime))

	return nil
}

func isEmptyRow(row []string) bool {
	if len(row) == 0 {
		return true
	}

	for i := range row {
		if row[i] != "" {
			return false
		}
	}

	return true
}

// buildSheetTables executes buildSheetTable for each sheet. If sheet is
// empty (has no data), the sheetTable element for that sheet will be nil.
func buildSheetTables(ctx context.Context, srcIngestHeader *bool, sheets []*xSheet) ([]*sheetTable, error) {
	sheetTbls := make([]*sheetTable, len(sheets))

	g, gCtx := errgroup.WithContext(ctx)
	for i := range sheets {
		i := i
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
			}

			sheetTbl, err := buildSheetTable(gCtx, srcIngestHeader, sheets[i])
			if err != nil {
				return err
			}
			sheetTbls[i] = sheetTbl
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return sheetTbls, nil
}

// getSrcIngestHeader returns nil if driver.OptIngestHeader is not set,
// and has the value of the opt if set.
func getSrcIngestHeader(o options.Options) *bool {
	if driver.OptIngestHeader.IsSet(o) {
		b := driver.OptIngestHeader.Get(o)
		return &b
	}

	return nil
}

// buildSheetTable constructs a table definition for the given sheet, and returns
// a model of the table, or an error. If the sheet is empty, (nil,nil)
// is returned. If srcIngestHeader is nil, the function attempts
// to detect if the sheet has a header row.
func buildSheetTable(ctx context.Context, srcIngestHeader *bool, sheet *xSheet) (*sheetTable, error) {
	sampleSize := driver.OptIngestSampleSize.Get(options.FromContext(ctx))
	if err := sheet.loadSampleRows(ctx, sampleSize); err != nil {
		return nil, err
	}

	var hasHeader bool
	if srcIngestHeader != nil {
		hasHeader = *srcIngestHeader
	} else {
		var err error
		if hasHeader, err = detectHeaderRow(ctx, sheet); err != nil {
			return nil, err
		}
	}

	maxCols := sheet.maxCols
	if maxCols == 0 {
		lg.FromContext(ctx).Warn("sheet is empty: skipping", laSheet, sheet.name)
		return nil, nil //nolint:nilnil
	}

	colNames := make([]string, maxCols)
	colKinds := make([]kind.Kind, maxCols)

	firstDataRow := 0
	if len(sheet.sampleRows) == 0 {
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
			headerCells := sheet.sampleRows[0]
			for i := 0; i < len(headerCells); i++ {
				colNames[i] = headerCells[i]
			}
		} else {
			for i := 0; i < maxCols; i++ {
				colNames[i] = stringz.GenerateAlphaColName(i, false)
			}
		}

		// Set up the column types
		if firstDataRow >= len(sheet.sampleRows) {
			// the sheet contains only one row (the header row). Let's
			// explicitly set the column type nonetheless
			for i := 0; i < maxCols; i++ {
				colKinds[i] = kind.Text
			}
		} else {
			// we have at least one data row, let's get the column types
			var err error
			colKinds, err = calcKindsForRows(firstDataRow, sheet)
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

	tblDef := &sqlmodel.TableDef{Name: sheet.name}
	cols := make([]*sqlmodel.ColDef, len(colNames))
	for i, colName := range colNames {
		cols[i] = &sqlmodel.ColDef{Table: tblDef, Name: colName, Kind: colKinds[i]}
	}
	tblDef.Cols = cols
	lg.FromContext(ctx).Debug("Built table def",
		laSheet, sheet.name,
		"cols", strings.Join(colNames, ", "))

	return &sheetTable{
		sheet:        sheet,
		def:          tblDef,
		hasHeaderRow: hasHeader,
	}, nil
}

// syncColNamesKinds ensures that column names and kinds are in
// a working state vis-à-vis each other. Notably if a colName is
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

func rowToRecord(ctx context.Context, destColKinds []kind.Kind, sheetName string,
	rowi int, cells []string, cellTypes []excelize.CellType,
) []any {
	log := lg.FromContext(ctx)

	vals := make([]any, len(destColKinds))
	for coli, str := range cells {
		if coli >= len(vals) {
			log.Warn(
				"Skipping additional cells because there's more cells than expected",
				laSheet, sheetName,
				lga.Col, fmt.Sprintf("%d:%d", rowi, coli),
				lga.Count, len(vals),
				lga.Expected, len(destColKinds),
			)
			continue
		}

		typ := cellTypes[coli]
		switch typ {
		case excelize.CellTypeBool:
			if b, err := stringz.ParseBool(str); err == nil {
				vals[coli] = b
				continue
			}

		case excelize.CellTypeNumber:
			if cells[coli] == "" {
				vals[coli] = nil
				continue
			}

			intVal, err := strconv.ParseInt(str, 10, 64)
			if err == nil {
				vals[coli] = intVal
				continue
			}
			//if cell.IsTime() {
			//	t, err := cell.GetTime(false)
			//	if err != nil {
			//		log.Warn("Sheet %s[%d:%d]: failed to get Excel time: %v", sheetName, rowIndex, j, err)
			//		vals[j] = nil
			//		continue
			//	}
			//
			//	vals[j] = t
			//	continue
			//}

			// floatVal, err := cell.Float()
			floatVal, err := strconv.ParseFloat(str, 64)
			if err == nil {
				vals[coli] = floatVal
				continue
			}

			// it's not an int, it's not a float, it's not empty string;
			// just give up and make it a string.
			log.Warn("Failed to determine type of numeric cell",
				laSheet, sheetName,
				"cell", fmt.Sprintf("%d:%d", rowi, coli),
				lga.Val, str,
			)

			vals[coli] = str
			// FIXME: prob should return an error here?
		case excelize.CellTypeInlineString, excelize.CellTypeSharedString,
			excelize.CellTypeFormula, excelize.CellTypeError:
			if str == "" {
				if destColKinds[coli] != kind.Text {
					vals[coli] = nil
					continue
				}
			}

			vals[coli] = str
		case excelize.CellTypeDate:
			// TODO: parse into a time value here?
			vals[coli] = str

		case excelize.CellTypeUnset:
			if str == "" {
				vals[coli] = nil
			} else {
				vals[coli] = str
			}
		default:
			if str == "" {
				vals[coli] = nil
			} else {
				vals[coli] = str
			}
		}
	}
	return vals
}

// calcKindsForRows calculates the lowest-common-denominator kind
// for the cells of rows. The returned slice will have length
// equal to the longest row.
func calcKindsForRows(firstDataRow int, sheet *xSheet) ([]kind.Kind, error) {
	rows := sheet.sampleRows

	if firstDataRow > len(rows) {
		return nil, errz.Errorf("rows are empty")
	}

	var detectors []*kind.Detector

	for i := firstDataRow; i < len(rows); i++ {
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

	for j := range detectors {
		knd, _, err := detectors[j].Detect()
		if err != nil {
			return nil, err
		}

		kinds[j] = knd
	}

	return kinds, nil
}

// sheetTable maps a sheet to a database table.
type sheetTable struct {
	sheet        *xSheet
	def          *sqlmodel.TableDef
	hasHeaderRow bool
}

func detectHeaderRow(ctx context.Context, sheet *xSheet) (hasHeader bool, err error) {
	sampleSize := driver.OptIngestSampleSize.Get(options.FromContext(ctx))

	if len(sheet.sampleRows) < 2 {
		// If zero records, obviously no header row.
		// If one record... well, is there any way of determining if
		// it's a header row or not? Probably best to treat it as a data row.
		return false, nil
	}

	types1, err := determineSampleColumnTypes(ctx, sheet, 0, sampleSize)
	if err != nil {
		return false, err
	}
	types2, err := determineSampleColumnTypes(ctx, sheet, 1, sampleSize)
	if err != nil {
		return false, err
	}

	if len(types1) != len(types2) {
		// Can this happen?
		return false, errz.Errorf("sheet {%s} has ragged edges", sheet.name)
	}

	if slices.Equal(types1, types2) {
		return false, nil
	}

	return true, nil
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
			types[coli] = excelize.CellTypeInlineString
		}
	}

	return types, nil
}

package xlsx

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"
	excelize "github.com/xuri/excelize/v2"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/loz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

const msgCloseRowIter = "Close Excel row iterator"

// sheetTable maps a sheet to a database table.
type sheetTable struct {
	sheet             *xSheet
	def               *sqlmodel.TableDef
	colIngestMungeFns []kind.MungeFunc
	hasHeaderRow      bool
}

// xSheet encapsulates access to a worksheet.
type xSheet struct {
	file       *excelize.File
	name       string
	sampleRows [][]string
	// sampleRowsMaxWidth is the width of the widest row in sampleRows.
	sampleRowsMaxWidth int
}

// loadSampleRows loads up to sampleSize rows, storing them to xSheet.sampleRows.
// Note that the row count may be less than sampleSize, if there aren't
// that many rows, or some rows are empty.
func (xs *xSheet) loadSampleRows(ctx context.Context, sampleSize int) error {
	iter, err := xs.file.Rows(xs.name)
	if err != nil {
		return err
	}

	defer lg.WarnIfCloseError(lg.FromContext(ctx), msgCloseRowIter, iter)

	var count int
	for iter.Next() {
		if count >= sampleSize {
			break
		}
		var cells []string
		if cells, err = iter.Columns(); err != nil {
			return err
		}

		if !loz.IsSliceZeroed(cells) {
			xs.sampleRows = append(xs.sampleRows, cells)
			if len(cells) > xs.sampleRowsMaxWidth {
				xs.sampleRowsMaxWidth = len(cells)
			}
		}

		count++
	}

	loz.AlignMatrixWidth(xs.sampleRows, "")

	return nil
}

// ingestXLSX loads the data in xfile into destGrip.
// If includeSheetNames is non-empty, only the named sheets are ingested.
func ingestXLSX(ctx context.Context, src *source.Source, destGrip driver.Grip, xfile *excelize.File) error {
	log := lg.FromContext(ctx)
	start := time.Now()
	log.Debug("Beginning import from XLSX",
		lga.Src, src,
		lga.Target, destGrip.Source())

	var sheets []*xSheet

	sheetNames := xfile.GetSheetList()
	sheets = make([]*xSheet, len(sheetNames))
	for i := range sheetNames {
		sheets[i] = &xSheet{file: xfile, name: sheetNames[i]}
	}

	srcIngestHeader := getSrcIngestHeader(src.Options)
	sheetTbls, err := buildSheetTables(ctx, srcIngestHeader, sheets)
	if err != nil {
		return err
	}

	lg.FromContext(ctx).Error("count is woah", lga.Count, len(sheetTbls))
	bar := progress.FromContext(ctx).NewUnitTotalCounter(
		"Ingesting sheets",
		"",
		int64(len(sheetTbls)),
	)
	defer bar.Stop()

	for _, sheetTbl := range sheetTbls {
		if sheetTbl == nil {
			// tblDef can be nil if its sheet is empty (has no data).
			continue
		}

		var db *sql.DB
		if db, err = destGrip.DB(ctx); err != nil {
			return err
		}

		if err = destGrip.SQLDriver().CreateTable(ctx, db, sheetTbl.def); err != nil {
			return err
		}
	}

	log.Debug("Tables created (but not yet populated)",
		lga.Count, len(sheetTbls),
		lga.Target, destGrip.Source(),
		lga.Elapsed, time.Since(start))

	var ingestCount, skipped int
	for i := range sheetTbls {
		time.Sleep(progress.DebugDelay)
		if sheetTbls[i] == nil {
			// tblDef can be nil if its sheet is empty (has no data).
			skipped++
			bar.IncrBy(1)
			continue
		}

		if err = ingestSheetToTable(ctx, destGrip, sheetTbls[i]); err != nil {
			return err
		}
		ingestCount++
		bar.IncrBy(1)
	}

	log.Debug("Sheets ingested",
		lga.Count, ingestCount,
		"skipped", skipped,
		lga.From, src,
		lga.To, destGrip.Source(),
		lga.Elapsed, time.Since(start),
	)

	return nil
}

// ingestSheetToTable imports the sheet data into the appropriate table
// in destGrip. The scratch table must already exist.
func ingestSheetToTable(ctx context.Context, destGrip driver.Grip, sheetTbl *sheetTable) error {
	var (
		log          = lg.FromContext(ctx)
		startTime    = time.Now()
		sheet        = sheetTbl.sheet
		hasHeader    = sheetTbl.hasHeaderRow
		tblDef       = sheetTbl.def
		destColKinds = tblDef.ColKinds()
	)

	db, err := destGrip.DB(ctx)
	if err != nil {
		return err
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDB, conn)

	drvr := destGrip.SQLDriver()

	batchSize := driver.MaxBatchRows(drvr, len(destColKinds))
	bi, err := driver.NewBatchInsert(
		ctx,
		"Ingest "+sheet.name,
		drvr,
		conn,
		tblDef.Name,
		tblDef.ColNames(),
		batchSize,
	)
	if err != nil {
		return err
	}

	iter, err := sheetTbl.sheet.file.Rows(sheetTbl.sheet.name)
	if err != nil {
		return errw(err)
	}

	defer lg.WarnIfCloseError(log, msgCloseRowIter, iter)

	var cells []string

	i := -1
	for iter.Next() {
		i++
		if hasHeader && i == 0 {
			continue
		}

		if cells, err = iter.Columns(); err != nil {
			close(bi.RecordCh)
			return err
		}

		if loz.IsSliceZeroed(cells) {
			// Skip empty row
			continue
		}

		rec := rowToRecord(ctx, destColKinds, sheetTbl.colIngestMungeFns, sheet.name, i, cells)
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

	err = <-bi.ErrCh // Stop for bi to complete
	if err != nil {
		return err
	}

	if err = iter.Error(); err != nil {
		return errz.Wrap(err, "excel: sheet iterator")
	}

	log.Debug("Inserted rows from sheet into table",
		lga.Count, bi.Written(),
		laSheet, sheet.name,
		lga.Target, source.Target(destGrip.Source(), tblDef.Name),
		lga.Elapsed, time.Since(startTime))

	return nil
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
				if errz.Has[*driver.EmptyDataError](err) {
					//if errz.IsErrNoData(err) { // FIXME: remove after testing
					// If the sheet has no data, we log it and skip it.
					lg.FromContext(ctx).Warn("Excel sheet has no data",
						laSheet, sheets[i].name,
						lga.Err, err)
					return nil
				}
				return err
			}
			sheetTbls[i] = sheetTbl
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Remove any nil sheets (which can happen if the sheet is empty).
	sheetTbls = lo.Compact(sheetTbls)

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
// If the sheet has no data, errz.EmptyDataError is returned.
func buildSheetTable(ctx context.Context, srcIngestHeader *bool, sheet *xSheet) (*sheetTable, error) {
	log := lg.FromContext(ctx)

	sampleSize := driver.OptIngestSampleSize.Get(options.FromContext(ctx))
	if err := sheet.loadSampleRows(ctx, sampleSize); err != nil {
		return nil, err
	}

	if len(sheet.sampleRows) == 0 {
		return nil, driver.NewEmptyDataError("excel: sheet {%s} has no row data", sheet.name)
	}

	if sheet.sampleRowsMaxWidth == 0 {
		return nil, driver.NewEmptyDataError("excel: sheet {%s} has no column data", sheet.name)
	}

	var hasHeader bool
	if srcIngestHeader != nil {
		hasHeader = *srcIngestHeader
	} else {
		var err error
		if hasHeader, err = detectHeaderRow(ctx, sheet); err != nil {
			return nil, err
		}

		log.Debug("Detect header row for sheet", laSheet, sheet.name, lga.Val, hasHeader)
	}

	maxCols := sheet.sampleRowsMaxWidth
	if maxCols == 0 {
		log.Warn("sheet is empty: skipping", laSheet, sheet.name)
		return nil, nil //nolint:nilnil
	}

	colNames := make([]string, maxCols)
	colKinds := make([]kind.Kind, maxCols)
	colIngestMungeFns := make([]kind.MungeFunc, maxCols)

	firstDataRow := 0

	// sheet is non-empty

	// Set up the column names
	if hasHeader {
		firstDataRow = 1
		copy(colNames, sheet.sampleRows[0])
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
		colKinds, colIngestMungeFns, err = detectSheetColumnKinds(sheet, firstDataRow)
		if err != nil {
			return nil, err
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
		sheet:             sheet,
		def:               tblDef,
		hasHeaderRow:      hasHeader,
		colIngestMungeFns: colIngestMungeFns,
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

// rowToRecord accepts a row (in arg cells), and converts it into an appropriate
// format for insertion to the DB.
func rowToRecord(ctx context.Context, destColKinds []kind.Kind, ingestMungeFns []kind.MungeFunc,
	sheetName string, rowi int, cells []string,
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

		if str == "" {
			vals[coli] = nil
			continue
		}

		if fn := ingestMungeFns[coli]; fn != nil {
			v, err := fn(str)
			if err != nil {
				// This shouldn't happen, but if it does, fall back
				// to the string value.
				vals[coli] = str
				log.Warn("Cell munge func failed",
					laSheet, sheetName,
					"cell", fmt.Sprintf("%d:%d", rowi, coli),
					lga.Val, vals[coli],
				)
			} else {
				vals[coli] = v
			}
			continue
		}

		// No munge func, just set the string.
		vals[coli] = str
	}
	return vals
}

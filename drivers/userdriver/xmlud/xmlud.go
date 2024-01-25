// Package xmlud provides user driver XML import functionality.
// Note that this implementation is experimental, not well-tested,
// inefficient, possibly incomprehensible, and subject to change.
//
// Also, it's really old, and just generally embarrassing. Don't look.
package xmlud

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/drivers/userdriver"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// Genre is the user driver genre that this package supports.
const Genre = "xml"

// Ingest implements userdriver.IngestFunc.
func Ingest(ctx context.Context, def *userdriver.DriverDef, data io.Reader, destGrip driver.Grip) error {
	if def.Genre != Genre {
		return errz.Errorf("xmlud.Ingest does not support genre {%s}", def.Genre)
	}

	log := lg.FromContext(ctx)
	db, err := destGrip.DB(ctx)
	if err != nil {
		return err
	}

	ing := &ingester{
		log:           log,
		destGrip:      destGrip,
		destDB:        db,
		data:          data,
		def:           def,
		selStack:      newSelStack(),
		rowStack:      newRowStack(),
		tblDefs:       map[string]*schema.Table{},
		tblSequence:   map[string]int64{},
		execInsertFns: map[string]func(ctx context.Context, insertVals []any) error{},
		execUpdateFns: map[string]func(ctx context.Context, updateVals, whereArgs []any) error{},
		clnup:         cleanup.New(),
		msgOnce:       map[string]struct{}{},
	}

	if err = ing.execIngest(ctx); err != nil {
		lg.WarnIfFuncError(log, "xml ingest: cleanup", ing.clnup.Run)
		return errz.Wrap(err, "xml ingest")
	}

	return errz.Wrap(ing.clnup.Run(), "xml ingest: cleanup")
}

// ingester does the work of importing data from XML.
type ingester struct {
	log      *slog.Logger
	def      *userdriver.DriverDef
	data     io.Reader
	destGrip driver.Grip
	destDB   sqlz.DB
	selStack *selStack
	rowStack *rowStack
	tblDefs  map[string]*schema.Table

	// tblSequence is a map of table name to the last
	// insert ID value for that table. See dbInsert for more.
	tblSequence map[string]int64

	// execInsertFns is a map of a table+cols key to a func for inserting
	// vals. Effectively it can be considered a cache of prepared insert
	// statements. See the dbInsert function.
	execInsertFns map[string]func(ctx context.Context, vals []any) error

	// execUpdateFns is similar to execInsertFns, but for UPDATE instead
	// of INSERT. The whereArgs param is the arguments for the
	// update's WHERE clause.
	execUpdateFns map[string]func(ctx context.Context, updateVals, whereArgs []any) error

	// clnup holds cleanup funcs that should be run when the ingester
	// finishes.
	clnup *cleanup.Cleanup

	// msgOnce is used by method msgOncef.
	msgOnce map[string]struct{}
}

func (in *ingester) execIngest(ctx context.Context) error { //nolint:gocognit
	err := in.createTables(ctx)
	if err != nil {
		return err
	}

	decoder := xml.NewDecoder(in.data)
	for {
		t, err := decoder.Token()
		if t == nil {
			break
		}
		if err != nil {
			return errz.Err(err)
		}

		switch elem := t.(type) {
		case xml.StartElement:
			in.selStack.push(elem.Name.Local)
			if in.isRootSelector() {
				continue
			}

			if in.isRowSelector() {
				// We found a new row...
				prevRow := in.rowStack.peek()
				if prevRow != nil {
					// Because the new row might require the primary key of the prev row,
					// we need to save the previous row, to ensure its primary key is
					// generated.
					err = in.saveRow(ctx, prevRow)
					if err != nil {
						return err
					}
				}

				var curRow *rowState
				curRow, err = in.buildRow()
				if err != nil {
					return err
				}

				in.rowStack.push(curRow)

				err = in.handleElemAttrs(elem, curRow)
				if err != nil {
					return err
				}

				continue
			}

			// It's not a row element, it's a col element
			curRow := in.rowStack.peek()
			if curRow == nil {
				return errz.Errorf("unable to parse XML: no current row on stack for elem {%s}", elem.Name.Local)
			}

			col := curRow.tbl.ColBySelector(in.selStack.selector())
			if col == nil {
				if msg, ok := in.msgOncef("Skip: element {%s} is not a column of table {%s}", elem.Name.Local,
					curRow.tbl.Name); ok {
					in.log.Debug(msg)
				}
				continue
			}

			curRow.curCol = col

			err = in.handleElemAttrs(elem, curRow)
			if err != nil {
				return err
			}

		case xml.EndElement:
			if in.isRowSelector() {
				row := in.rowStack.peek()
				if row.dirty() {
					err = in.saveRow(ctx, row)
					if err != nil {
						return err
					}
				}
				in.rowStack.pop()
			}
			in.selStack.pop()

		case xml.CharData:
			data := string(elem)
			curRow := in.rowStack.peek()

			if curRow == nil {
				continue
			}

			if curRow.curCol == nil {
				continue
			}

			val, err := in.convertVal(curRow.tbl.Name, curRow.curCol, data)
			if err != nil {
				return err
			}

			curRow.dirtyColVals[curRow.curCol.Name] = val
			curRow.curCol = nil
		}
	}

	return nil
}

func (in *ingester) convertVal(tbl string, col *userdriver.ColMapping, data any) (any, error) {
	const errTpl = `conversion error: %s.%s: expected "%s" but got %T(%v)`
	const errTplMsg = `conversion error: %s.%s: expected "%s" but got %T(%v): %v`

	switch col.Kind { //nolint:exhaustive
	default:
		return nil, errz.Errorf("unknown data kind {%s} for col %s", col.Kind, col.Name)
	case kind.Text, kind.Time:
		return data, nil
	case kind.Int:
		switch data := data.(type) {
		case int, int32, int64:
			return data, nil
		case string:
			val, err := strconv.ParseInt(data, 0, 64)
			if err != nil {
				return nil, errz.Errorf(errTplMsg, tbl, col.Name, col.Kind, data, data, err)
			}
			return val, nil
		default:
			return nil, errz.Errorf(errTpl, tbl, col.Name, col.Kind, data, data)
		}
	case kind.Float:
		switch data := data.(type) {
		case float32, float64:
			return data, nil
		case string:
			val, err := strconv.ParseFloat(data, 64)
			if err != nil {
				return nil, errz.Errorf(errTplMsg, tbl, col.Name, col.Kind, data, data, err)
			}
			return val, nil
		default:
			return nil, errz.Errorf(errTpl, tbl, col.Name, col.Kind, data, data)
		}
	case kind.Decimal:
		return data, nil
	case kind.Bool:
		switch data := data.(type) {
		case bool:
			return data, nil
		case int, int32, int64:
			if data == 0 {
				return false, nil
			}
			return true, nil
		case string:
			val, err := strconv.ParseBool(data)
			if err != nil {
				return nil, errz.Errorf(errTplMsg, tbl, col.Name, col.Kind, data, data, err)
			}
			return val, nil
		default:
			return nil, errz.Errorf(errTpl, tbl, col.Name, col.Kind, data, data)
		}
	case kind.Datetime, kind.Date:
		return data, nil
	case kind.Bytes:
		return data, nil
	case kind.Null:
		return data, nil
	}
}

func (in *ingester) handleElemAttrs(elem xml.StartElement, curRow *rowState) error {
	if len(elem.Attr) > 0 {
		baseSel := in.selStack.selector()

		for _, attr := range elem.Attr {
			attrSel := baseSel + "/@" + attr.Name.Local
			attrCol := curRow.tbl.ColBySelector(attrSel)
			if attrCol == nil {
				if msg, ok := in.msgOncef("Skip: attr {%s} is not a column of table {%s}", attrSel, curRow.tbl.Name); ok {
					in.log.Debug(msg)
				}

				continue
			}
			// We have found the col matching the attribute
			val, err := in.convertVal(curRow.tbl.Name, attrCol, attr.Value)
			if err != nil {
				return err
			}

			curRow.dirtyColVals[attrCol.Name] = val
		}
	}

	return nil
}

// setForeignColsVals sets the values of any column that needs to be
// populated from a foreign key.
func (in *ingester) setForeignColsVals(row *rowState) error {
	// check if we need to populate any of the row's values with
	// foreign key data (e.g. from parent table).
	for _, col := range row.tbl.Cols {
		if col.Foreign == "" {
			continue
		}
		// yep, we need to add a foreign key
		parts := strings.Split(col.Foreign, "/")
		// parts will look like [ "..", "channel_id" ]
		if len(parts) != 2 || parts[0] != ".." {
			return errz.Errorf(`%s.%s: "foreign" field should be of form "../col_name" but was {%s}`, row.tbl.Name,
				col.Name, col.Foreign)
		}

		fkName := parts[1]

		parentRow := in.rowStack.peekN(1)
		if parentRow == nil {
			return errz.Errorf("unable to find parent() table for foreign key for %s.%s", row.tbl.Name, col.Name)
		}

		fkVal, ok := parentRow.savedColVals[fkName]
		if !ok {
			return errz.Errorf(`%s.%s: unable to find foreign key value in parent table {%s}`, row.tbl.Name, col.Name,
				parentRow.tbl.Name)
		}

		row.dirtyColVals[col.Name] = fkVal
	}
	return nil
}

func (in *ingester) setSequenceColsVals(row *rowState, nextSeqVal int64) {
	seqColNames := userdriver.NamesFromCols(row.tbl.SequenceCols())
	for _, seqColName := range seqColNames {
		if _, ok := row.savedColVals[seqColName]; ok {
			// This seq col has already been saved
			continue
		}

		if _, ok := row.dirtyColVals[seqColName]; ok {
			// Hmmmn... seqColName is already present. This shouldn't happen,
			// as the point of a sequence col is to auto-generate the col
			// value. The input data is inconsistent, or at least, it
			// clashes with the user driver def.
			//
			// We could override the value, or trust the input.
			//
			// But given that the seqCol is typically the primary key,
			// trusting the input could cause a constraint violation
			// if a subsequent row doesn't have a value for the seqCol.
			//
			// Probably safer to override the value.
			row.dirtyColVals[seqColName] = nextSeqVal

			in.log.Warn("%s.%s is a auto-generated sequence() column: ignoring the value found in input",
				row.tbl.Name, seqColName)
			continue
		}

		// Else, this seq col has not yet been saved
		row.dirtyColVals[seqColName] = nextSeqVal
	}
}

func (in *ingester) saveRow(ctx context.Context, row *rowState) error {
	if !row.dirty() {
		return nil
	}

	tblDef, ok := in.tblDefs[row.tbl.Name]
	if !ok {
		return errz.Errorf("unable to find definition for table {%s}", row.tbl.Name)
	}

	if row.created() {
		// Row already exists in the db
		err := in.dbUpdate(ctx, row)
		if err != nil {
			return errz.Wrapf(err, "failed to update table {%s}", tblDef.Name)
		}

		row.markDirtyAsSaved()
		return nil
	}

	// We're going to INSERT the row.

	// Maintain the table's sequence. Note that we always increment the
	// seq val even if there are no sequence cols for this table.
	prevSeqVal := in.tblSequence[tblDef.Name]
	nextSeqVal := prevSeqVal + 1
	in.tblSequence[tblDef.Name] = nextSeqVal

	in.setSequenceColsVals(row, nextSeqVal)

	// Set any foreign cols
	err := in.setForeignColsVals(row)
	if err != nil {
		return err
	}

	// Verify that all required cols are present
	for _, requiredCol := range row.tbl.RequiredCols() {
		if _, ok = row.dirtyColVals[requiredCol.Name]; !ok {
			return errz.Errorf("no value for required column %s.%s", row.tbl.Name, requiredCol.Name)
		}
	}

	err = in.dbInsert(ctx, row)
	if err != nil {
		return errz.Wrapf(err, "failed to insert to table {%s}", tblDef.Name)
	}

	row.markDirtyAsSaved()
	return nil
}

// dbInsert inserts row's dirty col values to row's table.
func (in *ingester) dbInsert(ctx context.Context, row *rowState) error {
	tblName := row.tbl.Name
	colNames := make([]string, len(row.dirtyColVals))
	vals := make([]any, len(row.dirtyColVals))

	i := 0
	for k, v := range row.dirtyColVals {
		colNames[i], vals[i] = k, v
		i++
	}

	// We cache the prepared insert statements.
	cacheKey := "##insert_func__" + tblName + "__" + strings.Join(colNames, ",")

	execInsertFn, ok := in.execInsertFns[cacheKey]
	if !ok {
		// Nothing cached, prepare the insert statement and insert munge func
		stmtExecer, err := in.destGrip.SQLDriver().PrepareInsertStmt(ctx, in.destDB, tblName, colNames, 1)
		if err != nil {
			return err
		}

		// Make sure we close stmt eventually.
		in.clnup.AddC(stmtExecer)

		execInsertFn = func(ctx context.Context, vals []any) error {
			// Munge vals so that they're as the target DB expects
			err = stmtExecer.Munge(vals)
			if err != nil {
				return err
			}

			_, err = stmtExecer.Exec(ctx, vals...)
			return errz.Err(err)
		}

		// Cache the execInsertFn.
		in.execInsertFns[cacheKey] = execInsertFn
	}

	err := execInsertFn(ctx, vals)
	if err != nil {
		return err
	}

	return nil
}

// dbUpdate updates row's table with row's dirty values, using row's
// primary key cols as the args to the WHERE clause.
func (in *ingester) dbUpdate(ctx context.Context, row *rowState) error {
	drvr := in.destGrip.SQLDriver()
	tblName := row.tbl.Name
	pkColNames := row.tbl.PrimaryKey

	var whereBuilder strings.Builder
	var pkVals []any
	for i, pkColName := range pkColNames {
		if pkVal, ok := row.savedColVals[pkColName]; ok {
			pkVals = append(pkVals, pkVal)

			if i > 0 {
				whereBuilder.WriteString(" AND ")
			}
			whereBuilder.WriteString(drvr.Dialect().Enquote(pkColName))
			whereBuilder.WriteString(" = ?")

			continue
		}

		// Else, we're missing a pk val
		return errz.Errorf("failed to update table {%s}: primary key value {%s} not present", tblName, pkColName)
	}

	whereClause := whereBuilder.String()
	colNames := make([]string, len(row.dirtyColVals))
	dirtyVals := make([]any, len(row.dirtyColVals))

	i := 0
	for k, v := range row.dirtyColVals {
		colNames[i], dirtyVals[i] = k, v
		i++
	}

	// We cache the prepared statement.
	cacheKey := "##update_func__" + tblName + "__" + strings.Join(colNames, ",") + whereClause
	execUpdateFn, ok := in.execUpdateFns[cacheKey]
	if !ok {
		// Nothing cached, prepare the update statement and munge func
		stmtExecer, err := drvr.PrepareUpdateStmt(ctx, in.destDB, tblName, colNames, whereClause)
		if err != nil {
			return err
		}

		// Make sure we close stmt eventually.
		in.clnup.AddC(stmtExecer)

		execUpdateFn = func(ctx context.Context, updateVals, whereArgs []any) error {
			// Munge vals so that they're as the target DB expects
			err := stmtExecer.Munge(updateVals)
			if err != nil {
				return err
			}

			// Append the WHERE clause args
			updateVals = append(updateVals, whereArgs...)
			_, err = stmtExecer.Exec(ctx, updateVals...)
			return errz.Err(err)
		}

		// Cache the execInsertFn.
		in.execUpdateFns[cacheKey] = execUpdateFn
	}

	err := execUpdateFn(ctx, dirtyVals, pkVals)
	if err != nil {
		return err
	}

	return nil
}

func (in *ingester) buildRow() (*rowState, error) {
	tbl := in.def.TableBySelector(in.selStack.selector())
	if tbl == nil {
		return nil, errz.Errorf("no tbl matching current selector: %s", in.selStack.selector())
	}

	r := &rowState{tbl: tbl}
	r.dirtyColVals = make(map[string]any)
	r.savedColVals = make(map[string]any)

	for i := range r.tbl.Cols {
		// If the table has a column that has a "text()" selector, then we need to capture the
		// next CharData token, so we mark that col as the current col.
		if strings.HasSuffix(r.tbl.Cols[i].Selector, "text()") {
			r.curCol = r.tbl.Cols[i]
			break
		}
	}

	return r, nil
}

func (in *ingester) createTables(ctx context.Context) error {
	for i := range in.def.Tables {
		tblDef, err := userdriver.ToTableDef(in.def.Tables[i])
		if err != nil {
			return err
		}

		in.tblDefs[tblDef.Name] = tblDef

		err = in.destGrip.SQLDriver().CreateTable(ctx, in.destDB, tblDef)
		if err != nil {
			return err
		}
		in.log.Debug("Created table", lga.Target, source.Target(in.destGrip.Source(), tblDef.Name))
	}

	return nil
}

// isRootSelector returns true if the current selector matches the root selector.
func (in *ingester) isRootSelector() bool {
	return in.selStack.selector() == in.def.Selector
}

// isRowSelector returns true if entity referred to by the current selector
// maps to a table row (as opposed to a column).
func (in *ingester) isRowSelector() bool {
	return in.def.TableBySelector(in.selStack.selector()) != nil
}

// msgOncef is used to prevent repeated logging of a message. The
// method returns ok=true and the formatted string if the formatted
// string has not been previous seen by msgOncef.
func (in *ingester) msgOncef(format string, a ...any) (msg string, ok bool) {
	msg = fmt.Sprintf(format, a...)

	if _, exists := in.msgOnce[msg]; exists {
		// msg already seen, return ok=false.
		return "", false
	}

	in.msgOnce[msg] = struct{}{}
	return msg, true
}

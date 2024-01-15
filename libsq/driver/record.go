package driver

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"math"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/atomic"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/loz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// NewRecordFunc is invoked on a query result row (scanRow) to
// normalize and standardize the data, returning a new record.
// The provided scanRow arg is available for reuse after this
// func returns.
//
// Ultimately rec should only contain:
//
//	nil, *int64, *bool, *float64, *string, *[]byte, *time.Time
//
// Thus a func instance might unbox sql.NullString et al, or deal
// with any driver specific quirks.
type NewRecordFunc func(scanRow []any) (rec record.Record, err error)

// InsertMungeFunc is invoked on vals before insertion (or
// update, despite the name). Note that InsertMungeFunc operates
// on the vals slice, while NewRecordFunc returns a new slice.
type InsertMungeFunc func(vals record.Record) error

// StmtExecFunc is provided by driver implementations to wrap
// execution of a prepared statement. Typically the func will
// perform some driver-specific action, such as managing
// retryable errors.
type StmtExecFunc func(ctx context.Context, args ...any) (affected int64, err error)

// NewStmtExecer returns a new StmtExecer instance. The caller is responsible
// for invoking Close on the returned StmtExecer.
func NewStmtExecer(stmt *sql.Stmt, mungeFn InsertMungeFunc, execFn StmtExecFunc, destMeta record.Meta,
) *StmtExecer {
	return &StmtExecer{
		stmt:     stmt,
		mungeFn:  mungeFn,
		execFn:   execFn,
		destMeta: destMeta,
	}
}

// StmtExecer encapsulates the elements required to execute
// a SQL statement. Typically the statement is an INSERT.
// The Munge method should be applied to each
// row of values prior to invoking Exec. The caller
// is responsible for invoking Close.
type StmtExecer struct {
	stmt     *sql.Stmt
	mungeFn  InsertMungeFunc
	execFn   StmtExecFunc
	destMeta record.Meta
}

// DestMeta returns the record.Meta for the destination table columns.
func (x *StmtExecer) DestMeta() record.Meta {
	return x.destMeta
}

// Munge should be applied to each row of values prior
// to inserting invoking Exec.
func (x *StmtExecer) Munge(rec []any) error {
	if x.mungeFn == nil {
		return nil
	}

	err := x.mungeFn(rec)
	if err != nil {
		return err
	}
	return nil
}

// Exec executes the statement. The caller should invoke Munge on
// each row of values prior to passing those values to Exec.
func (x *StmtExecer) Exec(ctx context.Context, args ...any) (affected int64, err error) {
	return x.execFn(ctx, args...)
}

// Close closes x's statement.
func (x *StmtExecer) Close() error {
	return errz.Err(x.stmt.Close())
}

// NewRecordFromScanRow iterates over the elements of the row slice
// from rows.Scan, and returns a new (record) slice, replacing any
// wrapper types such as sql.NullString with the unboxed value,
// and other similar sanitization. For example, it will
// make a copy of any sql.RawBytes. The row slice
// can be reused by rows.Scan after this function returns.
//
// Any row elements specified in skip will not be processed; the
// value will be copied directly from row[i] into rec[i]. If any
// element of row otherwise cannot be processed, its value is
// copied directly into rec, and its index is returned in skipped.
// The caller must take appropriate action to deal with all
// elements of rec listed in skipped.
//
// REVISIT: Do we need the skip mechanism at all?
// REVISIT: This function implementation is an unholy mess. Much of it
// dates from sq's earliest days, and it's been hacked on ever since.
// Several of the code paths can probably never be reached.
// It should be completely rewritten.
//
//nolint:funlen,gocognit,gocyclo,cyclop
func NewRecordFromScanRow(meta record.Meta, row []any, skip []int) (rec record.Record, skipped []int) {
	rec = make([]any, len(row))

	// For convenience, make a map of the skip row indices.
	mSkip := map[int]struct{}{}
	for _, i := range skip {
		mSkip[i] = struct{}{}
	}

	for i := 0; i < len(row); i++ {
		// we're skipping this column, but still need to copy the value.
		if _, ok := mSkip[i]; ok {
			rec[i] = row[i]
			skipped = append(skipped, i)
			continue
		}

		if row[i] == nil {
			rec[i] = nil
			continue
		}

		// Dereference *any before the switch
		col := row[i]
		if a, ok := col.(*any); ok {
			col = *a
		}

		switch col := col.(type) {
		default:
			rec[i] = col
			skipped = append(skipped, i)
			continue
		case nil:
			rec[i] = nil
		case *int64:
			rec[i] = *col
		case int64:
			rec[i] = col
		case *float64:
			rec[i] = *col
		case float64:
			rec[i] = col
		case *bool:
			rec[i] = *col
		case bool:
			rec[i] = col
		case *string:
			rec[i] = *col
		case string:
			rec[i] = col
		case *[]byte:
			if col == nil || *col == nil {
				rec[i] = nil
				continue
			}

			if meta[i].Kind() != kind.Bytes {
				// We only want to use []byte for KindByte. Otherwise
				// switch to a string.
				s := string(*col)
				rec[i] = s
				continue
			}

			if len(*col) == 0 {
				rec[i] = []byte{}
			} else {
				dest := make([]byte, len(*col))
				copy(dest, *col)
				rec[i] = dest
			}

		case *sql.NullInt64:
			if col.Valid {
				rec[i] = col.Int64
			} else {
				rec[i] = nil
			}

		case *sql.NullString:
			if col.Valid {
				rec[i] = col.String
			} else {
				rec[i] = nil
			}

		case *sql.RawBytes:
			if col == nil || *col == nil {
				// Explicitly set rec[i] so that its type becomes nil
				rec[i] = nil
				continue
			}

			knd := meta[i].Kind()

			// If RawBytes is of length zero, there's no
			// need to copy.
			if len(*col) == 0 {
				if knd == kind.Bytes {
					rec[i] = []byte{}
				} else {
					// Else treat it as an empty string
					rec[i] = ""
				}

				continue
			}

			dest := make([]byte, len(*col))
			copy(dest, *col)

			if knd == kind.Bytes {
				rec[i] = dest
			} else {
				rec[i] = string(dest)
			}

		case *sql.NullFloat64:
			if col.Valid {
				rec[i] = col.Float64
			} else {
				rec[i] = nil
			}

		case *sql.NullBool:
			if col.Valid {
				rec[i] = col.Bool
			} else {
				rec[i] = nil
			}

		case *sqlz.NullBool:
			// This custom NullBool type is only used by sqlserver at this time.
			// Possibly this code should skip this item, and allow
			// the sqlserver munge func handle the conversion?
			if col.Valid {
				rec[i] = col.Bool
			} else {
				rec[i] = nil
			}

		case *sql.NullTime:
			if col.Valid {
				rec[i] = col.Time
			} else {
				rec[i] = nil
			}

		case *decimal.Decimal:
			rec[i] = *col

		case *decimal.NullDecimal:
			if col.Valid {
				rec[i] = col.Decimal
			} else {
				rec[i] = nil
			}
		case *time.Time:
			rec[i] = *col

		case *int:
			rec[i] = int64(*col)
		case *int8:
			rec[i] = int64(*col)
		case *int16:
			rec[i] = int64(*col)
		case *int32:
			rec[i] = int64(*col)
		case *uint:
			rec[i] = int64(*col)
		case *uint8:
			rec[i] = int64(*col)
		case *uint16:
			rec[i] = int64(*col)
		case *uint32:
			rec[i] = int64(*col)
		case *uint64:
			rec[i] = int64(*col)
		case *float32:
			rec[i] = float64(*col)
		}
	}

	return rec, skipped
}

// Comma is the comma string to use in SQL queries.
const Comma = ", "

// PrepareInsertStmt prepares an insert statement using
// driver-specific syntax from drvr. numRows specifies
// how many rows of values are inserted by each execution of
// the insert statement (1 row being the prototypical usage).
func PrepareInsertStmt(ctx context.Context, drvr SQLDriver, db sqlz.Preparer, destTbl string, destCols []string,
	numRows int,
) (stmt *sql.Stmt, err error) {
	const stmtTpl = `INSERT INTO %s (%s) VALUES %s`

	if numRows <= 0 {
		return nil, errz.Errorf("numRows must be a positive integer but got %d", numRows)
	}

	dialect := drvr.Dialect()
	tblNameQuoted := dialect.Enquote(destTbl)
	colNamesQuoted := loz.Apply(destCols, dialect.Enquote)
	colsJoined := strings.Join(colNamesQuoted, Comma)
	placeholders := dialect.Placeholders(len(colNamesQuoted), numRows)

	query := fmt.Sprintf(stmtTpl, tblNameQuoted, colsJoined, placeholders)
	stmt, err = db.PrepareContext(ctx, query)
	return stmt, errz.Err(err)
}

// BatchInsert encapsulates inserting records to a db. The caller sends
// (munged) records on recCh; the record values should be munged via
// the Munge method prior to sending. Records are written to db in
// batches of batchSize as passed to NewBatchInsert (the final batch may
// be less than batchSize). The caller must close recCh to indicate that
// all records have been sent, or cancel the ctx passed to
// NewBatchInsert to stop the insertion goroutine. Any error is returned
// on errCh. Processing is complete when errCh is closed: the caller
// must select on errCh.
type BatchInsert struct {
	// RecordCh is the channel that the caller sends records on. The
	// caller must close RecordCh when done.
	RecordCh chan<- []any

	// ErrCh returns any errors that occur during insert. ErrCh is
	// closed by BatchInsert when processing is complete.
	ErrCh <-chan error

	written *atomic.Int64

	mungeFn InsertMungeFunc
}

// Written returns the number of records inserted (at the time of
// invocation). For the final value, Written should be invoked after
// ErrCh is closed.
func (bi *BatchInsert) Written() int64 {
	return bi.written.Load()
}

// Munge should be invoked on every record before sending
// on RecordCh.
func (bi *BatchInsert) Munge(rec []any) error {
	return bi.mungeFn(rec)
}

// NewBatchInsert returns a new BatchInsert instance. The internal
// goroutine is started.
//
// Note that the db arg must guarantee a single connection: that is,
// it must be a sql.Conn or sql.Tx.
//
//nolint:gocognit
func NewBatchInsert(ctx context.Context, msg string, drvr SQLDriver, db sqlz.DB,
	destTbl string, destColNames []string, batchSize int,
) (*BatchInsert, error) {
	log := lg.FromContext(ctx)

	if err := sqlz.RequireSingleConn(db); err != nil {
		return nil, err
	}

	pbar := progress.FromContext(ctx).NewUnitCounter(msg, "rec")

	recCh := make(chan []any, batchSize*8)
	errCh := make(chan error, 1)
	rowLen := len(destColNames)

	inserter, err := drvr.PrepareInsertStmt(ctx, db, destTbl, destColNames, batchSize)
	if err != nil {
		return nil, err
	}

	bi := &BatchInsert{RecordCh: recCh, ErrCh: errCh, written: atomic.NewInt64(0), mungeFn: inserter.mungeFn}

	go func() {
		// vals holds rows of values as a single slice. That is, vals is
		// a bunch of record fields appended to one big slice to pass
		// as args to the INSERT statement
		vals := make([]any, 0, rowLen*batchSize)

		var rec []any
		var affected int64

		defer func() {
			pbar.Stop()

			if inserter != nil {
				if err == nil {
					// If no pre-existing error, any inserter.Close error
					// becomes the error.
					err = errz.Err(inserter.Close())
				} else {
					// If there's already an error, we just log any
					// error from inserter.Close: the pre-existing error
					// is the primary concern.
					lg.WarnIfError(log, lgm.CloseDBStmt, errz.Err(inserter.Close()))
				}
			}

			if err != nil {
				errCh <- err
			}

			close(errCh)
		}()

		for {
			rec = nil

			select {
			case <-ctx.Done():
				err = ctx.Err()
				return
			case rec = <-recCh:
			}

			if rec != nil {
				if len(rec) != rowLen {
					err = errz.Errorf("batch insert: record should have %d values but found %d", rowLen, len(rec))
					return
				}

				vals = append(vals, rec...)
			}

			if len(vals) == 0 {
				// Nothing to do here, we're done
				return
			}

			if len(vals)/rowLen == batchSize { // We've got a full batch to send
				affected, err = inserter.Exec(ctx, vals...)
				if err != nil {
					return
				}

				bi.written.Add(affected)
				pbar.Incr(int(affected))
				progress.DebugSleep(ctx)

				if rec == nil {
					// recCh is closed (coincidentally exactly on the
					// batch size), so we're successfully done.
					return
				}

				// reset vals for the next batch
				vals = vals[0:0]
				continue
			}

			if rec != nil {
				// recCh is not closed, so we loop to accumulate more records
				continue
			}

			// If we get this far, it means that rec is nil (indicating
			// no more records), but the number of remaining records
			// to write is less than batchSize. So, we'll need a new
			// inserter to write the remaining records.

			// First, close the existing full-batch-size inserter
			if inserter != nil {
				err = errz.Err(inserter.Close())
				inserter = nil
				if err != nil {
					return
				}
			}

			inserter, err = drvr.PrepareInsertStmt(ctx, db, destTbl, destColNames, len(vals)/rowLen)
			if err != nil {
				return
			}

			affected, err = inserter.Exec(ctx, vals...)
			if err != nil {
				return
			}

			bi.written.Add(affected)
			pbar.Incr(int(affected))
			progress.DebugSleep(ctx)

			// We're done
			return
		}
	}()

	return bi, nil
}

// MaxBatchRows returns the maximum number of rows allowed for a
// batch insert for drvr. Note that the returned value may differ
// for each database driver.
func MaxBatchRows(drvr SQLDriver, numCols int) int {
	return int(math.Ceil(float64(drvr.Dialect().MaxBatchValues) / float64(numCols)))
}

// DefaultInsertMungeFunc returns an InsertMungeFunc
// that checks the values of rec against destMeta and
// performs necessary munging. For example, if any element
// is a ptr to an empty string and the dest type
// is not of kind Text, the empty string was probably
// intended to mean nil. This happens when the original
// source doesn't handle nil, e.g. with CSV, where nil is
// effectively represented by "".
//
// The returned InsertMungeFunc accounts for common cases, but it's
// possible that certain databases will require a custom
// InsertMungeFunc.
func DefaultInsertMungeFunc(destTbl string, destMeta record.Meta) InsertMungeFunc {
	return func(rec record.Record) error {
		if len(rec) != len(destMeta) {
			return errz.Errorf("insert record has %d vals but dest table %s has %d cols (%s)",
				len(rec), destTbl, len(destMeta), strings.Join(destMeta.Names(), Comma))
		}

		for i := range rec {
			nullable, _ := destMeta[i].Nullable()
			if rec[i] == nil && !nullable {
				mungeSetZeroValue(i, rec, destMeta)
				continue
			}

			if destMeta[i].Kind() == kind.Text {
				// text doesn't need our help
				continue
			}

			// The dest col kind is something other than text, let's inspect
			// the actual value and check its type.
			switch val := rec[i].(type) {
			default:
				continue
			case string:
				if val == "" {
					if nullable {
						rec[i] = nil
					} else {
						mungeSetZeroValue(i, rec, destMeta)
					}
				}
				// else we let the DB figure it out

			case *string:
				if *val == "" {
					if nullable {
						rec[i] = nil
					} else {
						mungeSetZeroValue(i, rec, destMeta)
					}
				}
				// else we let the DB figure it out
			}
		}
		return nil
	}
}

// mungeSetZeroValue is invoked when rec[i] is nil, but
// destMeta[i] is not nullable.
func mungeSetZeroValue(i int, rec []any, destMeta record.Meta) {
	// REVISIT: do we need to do special handling for kind.Datetime
	//  and kind.Time (e.g. "00:00" for time)?
	z := reflect.Zero(destMeta[i].ScanType()).Interface()
	rec[i] = z
}

// OptResultColRename transforms a column name returned from the DB.
var OptResultColRename = options.NewString(
	"result.column.rename",
	"",
	0,
	"{{.Name}}{{with .Recurrence}}_{{.}}{{end}}",
	func(s string) error {
		return stringz.ValidTemplate("result.column.rename", s)
	},
	"Template to rename result columns",
	`This Go text template is executed on the column names returned
from the DB. Its primary purpose is to rename duplicate column names. For
example, given a query that results in this SQL:

  SELECT * FROM actor JOIN film_actor ON actor.actor_id = film_actor.actor_id

The returned result set will have these column names:

  actor_id, first_name, last_name, last_update, actor_id, film_id, last_update
  |-              from "actor"               -| |-    from "film_actor"     -|

Note the duplicate "actor_id" and "last_update" column names. When output in a
format (such as JSON) that doesn't permit duplicate keys, only one of each
duplicate column could appear.

The fields available in the template are:

  .Name         column name
  .Index        zero-based index of the column in the result set
  .Alpha        alphabetical index of the column, i.e. e.g. [A, B ... Z, AA, AB]
  .Recurrence   nth recurrence of the colum name in the result set

For a unique column name, e.g. "first_name" above, ".Recurrence" will be 0.
For duplicate column names, ".Recurrence" will be 0 for the first instance,
then 1 for the next instance, and so on.

The default template renames the columns to:

  actor_id, first_name, last_name, last_update, actor_id_1, film_id, last_update_1`,
	options.TagOutput,
)

// MungeResultColNames transforms column names, per the template defined
// in the option driver.OptResultColRename found on the context.
// This mechanism is used to deduplicate column names, as can happen in
// in "SELECT * FROM ... JOIN" situations. For example, if the result set
// has columns [actor_id, first_name, actor_id], the columns might be
// transformed to [actor_id, first_name, actor_id_1].
//
// driver.MungeResultColNames should be invoked by each impl
// of SQLDriver.RecordMeta before returning the record.Meta.
//
// See also: MungeIngestColNames.
func MungeResultColNames(ctx context.Context, ogColNames []string) (colNames []string, err error) {
	if len(ogColNames) == 0 {
		return ogColNames, nil
	}

	o := options.FromContext(ctx)
	tplText := OptResultColRename.Get(o)
	if tplText == "" {
		return ogColNames, nil
	}

	tpl, err := stringz.NewTemplate(OptResultColRename.Key(), tplText)
	if err != nil {
		return nil, errz.Wrap(err, "config: ")
	}

	return doMungeColNames(tpl, ogColNames)
}

func doMungeColNames(tpl *template.Template, ogColNames []string) (colNames []string, err error) {
	cols := make([]columnRenameTemplateData, len(ogColNames))

	for i := range ogColNames {
		data := columnRenameTemplateData{
			Name:  ogColNames[i],
			Index: i,
			Alpha: stringz.GenerateAlphaColName(i, false),
		}

		for j := 0; j < i; j++ {
			if ogColNames[j] == data.Name {
				data.Recurrence++
			}
		}

		cols[i] = data
	}

	colNames = make([]string, len(cols))
	buf := &bytes.Buffer{}
	for i := range cols {
		if err = tpl.Execute(buf, cols[i]); err != nil {
			return nil, err
		}

		colNames[i] = buf.String()
		buf.Reset()
	}

	return colNames, nil
}

// columnRenameTemplateData is the struct passed to the template from OptResultColRename
// and OptIngestColRename. It is used in MungeResultColNames.
type columnRenameTemplateData struct {
	// Name is the original column name.
	Name string

	// Index is the column index.
	Index int

	// Alpha is the Excel-style alphabetical index, i.e. A, B, ..., Z, AA, AB.
	Alpha string

	// Recurrence is the count of times this column name has already
	// appeared in the list of column names. If the column name is unique,
	// this value is zero.
	Recurrence int
}

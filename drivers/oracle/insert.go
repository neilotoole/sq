package oracle

import (
	"context"
	"database/sql"
	"reflect"
	"slices"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// Oracle before 23ai doesn't accept a multi-row VALUES list, so the driver
// inserts a batch of rows with array binding (Oracle "array DML") instead:
// a single-row INSERT executed once, with each bind parameter given a slice
// holding that column's value for every row. go-ora sends the whole batch in
// one round trip. Unlike a multi-row VALUES list, array binding doesn't need
// 23ai. CI tests this path on 21c and 26ai.
//
// Two go-ora behaviors mean a column's values must be of a consistent type:
//
//  1. go-ora encodes each slice element as its own Oracle type, but declares
//     the bind parameter with the type of the last non-nil element, and the
//     server decodes every element as that type.
//  2. When a prepared statement is executed again, go-ora resends the
//     parameter definitions only if a value's length grew or a string
//     switched between its short and long forms. A change of type alone,
//     such as NUMBER to VARCHAR2, is not resent, and the server decodes the
//     new values as the old type.
//
// Either way, values are mis-sent, and the insert doesn't always fail. An
// int64 followed by a string in a VARCHAR2 column can insert without error,
// storing the int64's NUMBER bytes as text. A string followed by an int64 in
// a NUMBER column can insert a value that fails with ORA-03090 when read.
//
// So arrayBinder reuses its prepared statement only for batches whose
// columns have the same value types as the statement's first execution. Any
// other batch runs as a one-off statement, which go-ora prepares afresh with
// new parameter definitions. A batch with a column of mixed types runs as a
// one-off statement per row.
//
// Atomicity is left to the caller's transaction. DBWriter, which inserts
// query results, wraps the whole insert in one. Without a transaction, as
// when ingest writes to an Oracle scratch source on a bare connection, a batch
// that fails partway can leave its earlier rows committed, on either path:
// Oracle array DML processes rows up to the failing one, and go-ora
// autocommits. Earlier batches are committed in that case anyway.
//
// A long []byte value also needs care. go-ora binds one longer than the
// server's RAW limit as LONG RAW, so a column of long and short values mixes
// RAW and LONG RAW, and fails with ORA-01461 whenever a long value isn't the
// last, or the statement last declared the column as RAW (go-ora's resend
// check misses RAW to LONG RAW). So a batch with a []byte value longer than
// maxRawBind also runs as a one-off statement per row, where each value is
// bound as its own type. Long strings don't need this: a column mixing LONG
// and VARCHAR2 values binds correctly, in any order and across executions.

// maxRawBind is the longest []byte that go-ora always binds as RAW. A longer
// one is bound as LONG RAW if it exceeds the server's RAW bind limit, which
// go-ora sets to 4000 bytes, or to 32767 when the server supports 32k binds.
const maxRawBind = 4000

// arrayBinder executes a single-row INSERT for batches of rows using array
// binding. Its exec method is a driver.StmtExecFunc.
type arrayBinder struct {
	// db executes one-off statements. It is the sqlz.DB the statement was
	// prepared on.
	db sqlz.Execer

	// stmt is the prepared single-row INSERT.
	stmt *sql.Stmt

	// query is stmt's SQL text, for one-off statements.
	query string

	// stmtTypes holds each column's value type from stmt's first execution,
	// as returned by columnTypes. It is nil until stmt is first executed.
	stmtTypes []reflect.Type

	numCols int
}

// exec executes the INSERT for a batch of rows. As with a multi-row INSERT,
// args holds the rows' values back to back, so its length must be a
// positive multiple of b.numCols.
func (b *arrayBinder) exec(ctx context.Context, args ...any) (int64, error) {
	cols, err := columnsFromRows(args, b.numCols)
	if err != nil {
		return 0, err
	}

	types, ok := columnTypes(cols)
	if !ok || hasLongBytes(cols) {
		// A column mixes value types or holds a long []byte: insert each
		// row as its own one-off statement.
		var total int64
		for row := range len(args) / b.numCols {
			affected, err := execResult(b.db.ExecContext(ctx, b.query,
				args[row*b.numCols:(row+1)*b.numCols]...))
			if err != nil {
				return total, err
			}
			total += affected
		}
		return total, nil
	}

	bindArgs := make([]any, len(cols))
	for i := range cols {
		bindArgs[i] = cols[i]
	}

	if b.stmtTypes != nil && !slices.Equal(types, b.stmtTypes) {
		// stmt's parameter definitions don't fit this batch.
		return execResult(b.db.ExecContext(ctx, b.query, bindArgs...))
	}

	if b.stmtTypes == nil {
		b.stmtTypes = types
	}
	return execResult(b.stmt.ExecContext(ctx, bindArgs...))
}

// hasLongBytes reports whether any value in cols is a []byte longer than
// maxRawBind, which go-ora might bind as LONG RAW.
func hasLongBytes(cols [][]any) bool {
	for _, col := range cols {
		for _, v := range col {
			if b, ok := v.([]byte); ok && len(b) > maxRawBind {
				return true
			}
		}
	}
	return false
}

// execResult returns the rows affected by an exec that returned res and err.
func execResult(res sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, errw(err)
	}

	affected, err := res.RowsAffected()
	return affected, errw(err)
}

// columnsFromRows regroups args, which holds rows of numCols values back to
// back, into one slice per column.
func columnsFromRows(args []any, numCols int) ([][]any, error) {
	if numCols <= 0 {
		return nil, errz.Errorf("oracle: array bind: numCols must be positive but got %d", numCols)
	}
	if len(args) == 0 || len(args)%numCols != 0 {
		return nil, errz.Errorf("oracle: array bind: got %d args, want a positive multiple of %d",
			len(args), numCols)
	}

	numRows := len(args) / numCols
	cols := make([][]any, numCols)
	for col := range numCols {
		vals := make([]any, numRows)
		for row := range numRows {
			vals[row] = args[row*numCols+col]
		}
		cols[col] = vals
	}
	return cols, nil
}

// columnTypes returns the Go type of each column's non-nil values, with nil
// for a column whose values are all nil. It returns false if any column's
// non-nil values are not all of the same type, in which case go-ora can't
// bind that column as a single array parameter.
func columnTypes(cols [][]any) ([]reflect.Type, bool) {
	types := make([]reflect.Type, len(cols))
	for i, col := range cols {
		for _, v := range col {
			if v == nil {
				continue
			}

			t := reflect.TypeOf(v)
			if types[i] == nil {
				types[i] = t
				continue
			}
			if t != types[i] {
				return nil, false
			}
		}
	}
	return types, true
}

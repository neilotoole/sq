package oracle

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
)

// Oracle before 23ai doesn't accept a multi-row VALUES list, so the driver
// inserts a batch of rows with array binding (Oracle "array DML") instead:
// a single-row INSERT executed once, with each bind parameter given a slice
// holding that column's value for every row. go-ora sends the whole batch in
// one round trip. This works on every Oracle version.
//
// go-ora encodes each slice element as its own Oracle type, but declares the
// bind parameter with the type of the last non-nil element. A column whose
// values would encode as different Oracle types would then send some values
// under the wrong type. arrayBindable detects that case, and the batch is
// executed one row at a time instead.

const (
	// maxArrayBindString is the length in bytes beyond which go-ora may bind
	// a string as LONG instead of VARCHAR2. It is the VARCHAR2 bind limit
	// when MAX_STRING_SIZE is STANDARD, the smallest the limit can be.
	maxArrayBindString = 4000

	// maxArrayBindBytes is the length beyond which go-ora may bind a []byte
	// as LONG RAW instead of RAW. It is the RAW bind limit when
	// MAX_STRING_SIZE is STANDARD, the smallest the limit can be.
	maxArrayBindBytes = 2000
)

// newArrayBindExecFunc returns a driver.StmtExecFunc that executes stmt, a
// prepared single-row INSERT of numCols columns, for a batch of rows. As
// with a multi-row INSERT, args holds the rows' values back to back, so its
// length must be a positive multiple of numCols.
func newArrayBindExecFunc(stmt *sql.Stmt, numCols int) driver.StmtExecFunc {
	return func(ctx context.Context, args ...any) (int64, error) {
		cols, err := columnsFromRows(args, numCols)
		if err != nil {
			return 0, err
		}

		if arrayBindable(cols) {
			return execArrayBind(ctx, stmt, cols)
		}

		// Execute one row at a time, still passing each value as a
		// one-element slice: go-ora stops array binding on a statement
		// for good once it is executed with a scalar arg.
		var total int64
		for row := range len(args) / numCols {
			rowCols := make([][]any, numCols)
			for col := range numCols {
				rowCols[col] = []any{args[row*numCols+col]}
			}

			affected, err := execArrayBind(ctx, stmt, rowCols)
			if err != nil {
				return total, err
			}
			total += affected
		}
		return total, nil
	}
}

// execArrayBind executes stmt once, binding each element of cols (one
// column's values) to the corresponding parameter.
func execArrayBind(ctx context.Context, stmt *sql.Stmt, cols [][]any) (int64, error) {
	args := make([]any, len(cols))
	for i := range cols {
		args[i] = cols[i]
	}

	res, err := stmt.ExecContext(ctx, args...)
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

// arrayBindable reports whether go-ora can bind every column of cols as a
// single array parameter. That requires all non-nil values in a column to
// be of the same Go type, and no string or []byte value to be long enough
// for go-ora to switch it to the LONG or LONG RAW type.
func arrayBindable(cols [][]any) bool {
	for _, col := range cols {
		var typ reflect.Type
		for _, v := range col {
			switch v := v.(type) {
			case nil:
				continue
			case string:
				if len(v) > maxArrayBindString {
					return false
				}
			case []byte:
				if len(v) > maxArrayBindBytes {
					return false
				}
			}

			t := reflect.TypeOf(v)
			if typ == nil {
				typ = t
				continue
			}
			if t != typ {
				return false
			}
		}
	}
	return true
}

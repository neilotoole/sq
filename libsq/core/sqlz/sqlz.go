// Package sqlz contains core types such as Kind and Record.
package sqlz

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
)

// Execer abstracts the ExecContext method
// from sql.DB and friends.
type Execer interface {
	// ExecContext is documented by sql.DB.ExecContext.
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Queryer abstracts the QueryContext and QueryRowContext
// methods from sql.DB and friends.
type Queryer interface {
	// QueryContext is documented by sql.DB.QueryContext.
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)

	// QueryRowContext is documented by sql.DB.QueryRowContext.
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Preparer abstracts the PrepareContext method from sql.DB and
// friends.
type Preparer interface {
	// PrepareContext is documented by sql.DB.PrepareContext.
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

// DB is a union of Execer, Queryer and Preparer. DB's method
// set is implemented by sql.DB, sql.Conn and sql.Tx.
// This can probably be better named, as it conflicts
// with sql.DB.
type DB interface {
	Execer
	Queryer
	Preparer
}

// ExecAffected invokes db.ExecContext, returning the count of rows
// affected and any error.
func ExecAffected(ctx context.Context, db Execer, query string, args ...any) (affected int64, err error) {
	var res sql.Result
	res, err = db.ExecContext(ctx, query, args...)
	if err != nil {
		return affected, errz.Err(err)
	}

	affected, err = res.RowsAffected()
	if err != nil {
		return affected, errz.Err(err)
	}

	return affected, nil
}

// Canonical driver-independent names for table types.
const (
	TableTypeTable   = "table"
	TableTypeView    = "view"
	TableTypeVirtual = "virtual"
)

// RequireSingleConn returns nil if db is a type that guarantees a
// single database connection. That is, RequireSingleConn returns an
// error if db does not have type *sql.Conn or *sql.Tx.
func RequireSingleConn(db DB) error {
	switch db.(type) {
	case *sql.Conn, *sql.Tx:
	default:
		return errz.Errorf("sql.Conn or sql.Tx required, but was %T", db)
	}

	return nil
}

// RowsScanColumn scans a single-column [*sql.Rows] into a slice of T. If the
// returned value could be null, use a nullable type, e.g. [sql.NullString].
// Arg rows is always closed. On any error, the returned slice is nil.
func RowsScanColumn[T any](ctx context.Context, rows *sql.Rows) (vals []T, err error) {
	defer func() {
		if rows != nil {
			CloseRows(lg.FromContext(ctx), rows)
		}
	}()

	// We want to return an empty slice rather than nil.
	vals = make([]T, 0)
	for rows.Next() {
		var val T
		if err = rows.Scan(&val); err != nil {
			return nil, errz.Err(err)
		}
		vals = append(vals, val)
	}

	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	err = rows.Close()
	rows = nil
	if err != nil {
		return nil, errz.Err(err)
	}

	return vals, nil
}

// CloseRows closes the supplied *sql.Rows, logging a warning on close errors
// other than context errors. If log is nil, no logging is performed. If rows
// is nil, CloseRows is no-op.
func CloseRows(log *slog.Logger, rows *sql.Rows) {
	if rows == nil {
		return
	}

	err := rows.Close()
	if log == nil || err == nil {
		return
	}

	if errz.IsErrContext(err) {
		// We don't want to log context errors.
		return
	}

	log.Warn(lgm.CloseDBRows, lga.Err, err)
}

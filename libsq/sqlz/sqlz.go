// Package sqlz contains core types such as Kind and Record.
package sqlz

import (
	"context"
	"database/sql"

	"github.com/neilotoole/sq/libsq/errz"
)

// Execer abstracts the ExecContext method
// from sql.DB and friends.
type Execer interface {
	// ExecContext is documented by sql.DB.ExecContext.
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// Queryer abstracts the QueryContext and QueryRowContext
// methods from sql.DB and friends.
type Queryer interface {
	// QueryContext is documented by sql.DB.QueryContext.
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)

	// QueryRowContext is documented by sql.DB.QueryRowContext.
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
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

// ExecResult invokes db.ExecContext, returning the count of rows
// affected and any error.
func ExecResult(ctx context.Context, db Execer, query string, args ...interface{}) (affected int64, err error) {
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

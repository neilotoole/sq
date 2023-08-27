// Package sqlz contains core types such as Kind and Record.
package sqlz

import (
	"context"
	"database/sql"
	"strings"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/core/errz"
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

// TableFQ is a fully-qualified table name of the form CATALOG.SCHEMA.NAME.
type TableFQ struct {
	Catalog string
	Schema  string
	Table   string
}

// String returns a representation of t using double-quoted escaped
// values for the components. Note that a DB may use a different
// escaping mechanism, so do not use this representation for rendering
// SQL.
func (t TableFQ) String() string {
	sb := strings.Builder{}
	if t.Catalog != "" {
		sb.WriteString(stringz.DoubleQuote(t.Catalog))
		sb.WriteRune('.')
	}
	if t.Schema != "" {
		sb.WriteString(stringz.DoubleQuote(t.Schema))
		sb.WriteRune('.')
	}
	sb.WriteString(stringz.DoubleQuote(t.Table))
	return sb.String()
}

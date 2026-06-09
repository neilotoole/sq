package rqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"github.com/rqlite/gorqlite"
	"github.com/rqlite/gorqlite/stdlib"
)

// sqDBDrvrName is the registration name of sq's wrapper around
// gorqlite's database/sql driver. The wrapper exists to expose
// per-column type names through *sql.ColumnType.DatabaseTypeName: the
// stock gorqlite/stdlib driver does not implement
// driver.RowsColumnTypeDatabaseTypeName, so DatabaseTypeName always
// returns the empty string, which downstream demotes every column
// kind to kind.Unknown.
//
// The wrapped driver consults the gorqlite QueryResult's Types
// (populated from the JSON "types" array on every response, including
// empty result sets) and returns those values from
// ColumnTypeDatabaseTypeName. That single change is enough for the
// rest of the rqlite driver to resolve column kinds, scan-types, and
// the float64-from-JSON to int64 coercion path through *sql.NullInt64.
const sqDBDrvrName = "rqlite-sq"

func init() { //nolint:gochecknoinits
	sql.Register(sqDBDrvrName, &sqDriver{inner: &stdlib.Driver{}})
}

// sqDriver wraps gorqlite/stdlib's driver to add the optional
// RowsColumnTypeDatabaseTypeName interface on the returned Rows.
type sqDriver struct {
	inner driver.Driver
}

// Open implements driver.Driver.
func (d *sqDriver) Open(name string) (driver.Conn, error) {
	c, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	sc, ok := c.(*stdlib.Conn)
	if !ok {
		// Defensive: gorqlite/stdlib only returns *stdlib.Conn. If
		// that ever changes, fall back to the unwrapped conn rather
		// than failing the open: callers lose the DBTypeName
		// enrichment but everything else still works.
		return c, nil
	}
	return &sqConn{Conn: sc}, nil
}

// sqConn wraps stdlib.Conn so its Prepare returns sqStmt. The embedded
// *stdlib.Conn keeps gorqlite.Connection's methods (notably
// WriteParameterizedContext) reachable via conn.Raw, which the
// writeAtomic helper depends on.
type sqConn struct {
	*stdlib.Conn
}

// Prepare implements driver.Conn.
func (c *sqConn) Prepare(query string) (driver.Stmt, error) {
	s, err := c.Conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	ss, ok := s.(*stdlib.Stmt)
	if !ok {
		return s, nil
	}
	return &sqStmt{Stmt: ss}, nil
}

// sqStmt wraps stdlib.Stmt so its Query / QueryContext return sqRows.
// Exec / ExecContext are inherited from the embedded *stdlib.Stmt.
type sqStmt struct {
	*stdlib.Stmt
}

// Query implements driver.Stmt.
func (s *sqStmt) Query(args []driver.Value) (driver.Rows, error) {
	r, err := s.Stmt.Query(args)
	return wrapRows(r), err
}

// QueryContext implements driver.StmtQueryContext.
func (s *sqStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	r, err := s.Stmt.QueryContext(ctx, args)
	return wrapRows(r), err
}

// wrapRows wraps a *stdlib.Rows in sqRows. nil and non-stdlib Rows
// pass through unchanged so transient error paths don't blow up.
func wrapRows(r driver.Rows) driver.Rows {
	if r == nil {
		return nil
	}
	sr, ok := r.(*stdlib.Rows)
	if !ok {
		return r
	}
	return &sqRows{Rows: sr}
}

// sqRows wraps stdlib.Rows to implement
// driver.RowsColumnTypeDatabaseTypeName, sourcing per-column types
// from the gorqlite QueryResult.
type sqRows struct {
	*stdlib.Rows
}

// Compile-time assertion that sqRows satisfies the optional
// interface; if the upstream Rows shape changes such that the
// embedded QueryResult Types method goes away, this becomes a
// build error.
var _ driver.RowsColumnTypeDatabaseTypeName = (*sqRows)(nil)

// ColumnTypeDatabaseTypeName implements
// driver.RowsColumnTypeDatabaseTypeName. The strings come from the
// SELECT's "types" array, which rqlite populates from SQLite's
// pragma-style column metadata. For empty result sets the array is
// still populated, so this works equally well for empty tables.
//
// Out-of-range indexes return the empty string, mirroring the
// database/sql default and matching what callers in
// kindFromDBTypeName already handle.
func (r *sqRows) ColumnTypeDatabaseTypeName(index int) string {
	types := typesOf(r.Rows)
	if index < 0 || index >= len(types) {
		return ""
	}
	return types[index]
}

// typesOf returns the per-column type strings from the gorqlite
// QueryResult embedded inside a stdlib.Rows. The result may be nil
// if the underlying response carried no types array.
func typesOf(r *stdlib.Rows) []string {
	if r == nil {
		return nil
	}
	// stdlib.Rows embeds gorqlite.QueryResult by value, and
	// QueryResult exposes Types(). The interface match shields
	// against future internal renames.
	qr, ok := any(&r.QueryResult).(interface{ Types() []string })
	if !ok {
		return nil
	}
	return qr.Types()
}

// Compile-time check that gorqlite.QueryResult still exposes Types.
// If gorqlite ever renames or removes the method, this fails the
// build rather than silently leaving every column kind as Unknown.
var _ interface{ Types() []string } = (*gorqlite.QueryResult)(nil)

package rqlite

import (
	"context"
	"database/sql"

	"github.com/rqlite/gorqlite"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// gorqliteConn is the structural interface satisfied by gorqlite/stdlib's
// *Conn (whose embedded *gorqlite.Connection provides the method).
// Type-asserting against this interface lets us avoid importing
// gorqlite/stdlib directly.
type gorqliteConn interface {
	WriteParameterizedContext(context.Context,
		[]gorqlite.ParameterizedStatement) ([]gorqlite.WriteResult, error)
}

// writeAtomic executes stmts as a single atomic batch through gorqlite's
// native WriteParameterizedContext. The whole batch is rolled back
// server-side on the first per-statement error.
//
// db must be *sql.DB or *sql.Conn. Passing *sql.Tx returns an error:
// gorqlite's stdlib Tx is a no-op, so wrapping atomic writes inside an
// outer Tx would silently lose atomicity.
//
// Returns the raw []WriteResult so callers can extract the
// per-statement RowsAffected / LastInsertID they actually trust.
// Be aware that rqlite reports rows_affected as the underlying SQLite
// changes() value, which is the most recent DML row count and is
// therefore stale for DDL statements in a batch. Pick the index of
// the statement whose count is meaningful (typically the last DML).
func writeAtomic(ctx context.Context, db sqlz.DB,
	stmts ...gorqlite.ParameterizedStatement,
) (results []gorqlite.WriteResult, err error) {
	log := lg.FromContext(ctx)

	var conn *sql.Conn
	switch v := db.(type) {
	case *sql.DB:
		conn, err = v.Conn(ctx)
		if err != nil {
			return nil, errw(err)
		}
		defer lg.WarnIfFuncError(log, lgm.CloseConn, conn.Close)
	case *sql.Conn:
		conn = v
	case *sql.Tx:
		return nil, errz.New("rqlite: writeAtomic cannot run inside *sql.Tx; " +
			"gorqlite's tx is a no-op so atomicity would be silently lost")
	default:
		return nil, errz.Errorf("rqlite: writeAtomic: unsupported db type %T", v)
	}

	rawErr := conn.Raw(func(raw any) error {
		gc, ok := raw.(gorqliteConn)
		if !ok {
			return errz.Errorf("rqlite: driver conn is %T, "+
				"expected gorqlite-backed", raw)
		}
		var wErr error
		results, wErr = gc.WriteParameterizedContext(ctx, stmts)
		return wErr
	})
	if rawErr != nil {
		log.Debug("rqlite: writeAtomic raw error", lga.Err, rawErr)
		return results, errw(rawErr)
	}

	for i, wr := range results {
		if wr.Err != nil {
			return results, errz.Wrapf(errw(wr.Err),
				"rqlite: statement %d/%d failed", i+1, len(stmts))
		}
	}
	return results, nil
}

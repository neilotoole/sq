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
// Returns the sum of RowsAffected across stmts (informational when an
// error is returned; rqlite rolled back the whole batch).
func writeAtomic(ctx context.Context, db sqlz.DB,
	stmts ...gorqlite.ParameterizedStatement,
) (totalAffected int64, err error) {
	log := lg.FromContext(ctx)

	var conn *sql.Conn
	switch v := db.(type) {
	case *sql.DB:
		conn, err = v.Conn(ctx)
		if err != nil {
			return 0, errw(err)
		}
		defer lg.WarnIfFuncError(log, lgm.CloseConn, conn.Close)
	case *sql.Conn:
		conn = v
	case *sql.Tx:
		return 0, errz.New("rqlite: writeAtomic cannot run inside *sql.Tx; " +
			"gorqlite's tx is a no-op so atomicity would be silently lost")
	default:
		return 0, errz.Errorf("rqlite: writeAtomic: unsupported db type %T", v)
	}

	var results []gorqlite.WriteResult
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
		return 0, errw(rawErr)
	}

	for i, wr := range results {
		if wr.Err != nil {
			return totalAffected, errz.Wrapf(errw(wr.Err),
				"rqlite: statement %d/%d failed", i+1, len(stmts))
		}
		totalAffected += wr.RowsAffected
	}
	return totalAffected, nil
}

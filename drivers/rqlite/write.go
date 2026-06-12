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

// gorqliteWriter is the structural interface satisfied by gorqlite/stdlib's
// *Conn (whose embedded *gorqlite.Connection provides the methods).
// Asserting against this method-shape interface lets us reach the
// atomic-batch primitive without importing gorqlite/stdlib directly;
// the coupling shifts from a package import to a method-shape match
// on *gorqlite.Connection. An upstream signature change becomes a
// runtime type-assert failure rather than a compile error. The
// `var _ gorqliteWriter = (*gorqlite.Connection)(nil)` line below
// keeps that case caught at build time.
type gorqliteWriter interface {
	WriteParameterizedContext(context.Context,
		[]gorqlite.ParameterizedStatement) ([]gorqlite.WriteResult, error)
	SetExecutionWithTransaction(bool) error
}

// Compile-time check that *gorqlite.Connection satisfies gorqliteWriter.
// If gorqlite ever renames or changes the WriteParameterizedContext
// signature, this becomes a build error rather than a runtime assert
// failure.
var _ gorqliteWriter = (*gorqlite.Connection)(nil)

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
//
// On success, results has exactly len(stmts) entries. On error, results
// may be partial or empty and should not be inspected for counts.
func writeAtomic(ctx context.Context, db sqlz.DB,
	stmts ...gorqlite.ParameterizedStatement,
) ([]gorqlite.WriteResult, error) {
	return write(ctx, db, true, stmts)
}

// writeNonTx executes stmts as a single /db/execute request WITHOUT
// rqlite's transaction wrapper, by clearing gorqlite's per-connection
// transaction flag for the duration of the request (gorqlite otherwise
// unconditionally appends &transaction, and rqlite then wraps the
// request in BEGIN/COMMIT). The flag is restored before the connection
// returns to the pool.
//
// This exists for statements that are no-ops inside a transaction:
// SQLite specifies that PRAGMA foreign_keys does nothing while a
// transaction is pending, so toggling it requires a non-transactional
// request (gh776). It takes exactly one statement: a non-transactional
// multi-statement batch would not be atomic, so anything that must
// roll back together belongs in writeAtomic.
func writeNonTx(ctx context.Context, db sqlz.DB,
	stmt gorqlite.ParameterizedStatement,
) ([]gorqlite.WriteResult, error) {
	return write(ctx, db, false, []gorqlite.ParameterizedStatement{stmt})
}

// write implements writeAtomic and writeNonTx. If withTx is true, the
// request rides rqlite's transaction wrapper (gorqlite's default);
// otherwise the connection's transaction flag is cleared around the
// request and restored afterward.
func write(ctx context.Context, db sqlz.DB, withTx bool,
	stmts []gorqlite.ParameterizedStatement,
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
		return nil, errz.New("rqlite: write cannot run inside *sql.Tx: " +
			"gorqlite's transaction support is a no-op")
	default:
		return nil, errz.Errorf("rqlite: write batch: unsupported db type %T", v)
	}

	rawErr := conn.Raw(func(raw any) error {
		gc, ok := raw.(gorqliteWriter)
		if !ok {
			return errz.Errorf("rqlite: driver conn is %T, "+
				"expected gorqlite-backed", raw)
		}
		if !withTx {
			if txErr := gc.SetExecutionWithTransaction(false); txErr != nil {
				return errz.Wrap(txErr,
					"rqlite: failed to clear connection transaction flag")
			}
			// Restore gorqlite's default (transactions on) before the
			// connection goes back to the pool, whether or not the
			// write succeeded.
			defer func() {
				if txErr := gc.SetExecutionWithTransaction(true); txErr != nil {
					log.Error("rqlite: failed to restore connection transaction flag",
						lga.Err, txErr)
				}
			}()
		}
		var wErr error
		results, wErr = gc.WriteParameterizedContext(ctx, stmts)
		return wErr
	})

	// gorqlite surfaces per-statement failures BOTH as a top-level
	// aggregate error AND in results[i].Err. Prefer the per-statement
	// wrap when available: it tells the caller which statement in the
	// batch failed, which gorqlite's "there were N statement errors"
	// aggregate does not. Fall through to the raw error only if no
	// per-statement error is present (e.g. transport, auth, parse).
	for i, wr := range results {
		if wr.Err != nil {
			return results, errz.Wrapf(errw(wr.Err),
				"rqlite: statement %d/%d failed", i+1, len(stmts))
		}
	}
	if rawErr != nil {
		log.Debug("rqlite: write batch raw error", lga.Err, rawErr)
		return results, errw(rawErr)
	}
	if len(results) != len(stmts) {
		return results, errz.Errorf(
			"rqlite: write batch: expected %d results but got %d",
			len(stmts), len(results))
	}
	return results, nil
}

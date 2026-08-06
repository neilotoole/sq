package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
)

// errw wraps any error from the db. It should be called at
// every interaction with the db. If err is nil, nil is returned.
// Certain errors will be wrapped in specific error types,
// e.g, errz.NotExistError.
func errw(err error) error {
	switch {
	case err == nil:
		return nil
	case hasErrCode(err, errCodeRelationNotExist):
		return driver.NewNotExistError(err)
	default:
		return errz.Err(err)
	}
}

const (
	errCodeRelationNotExist   = "42P01"
	errCodeTooManyConnections = "53300"
	// errCodeInternalError (XX000) is raised, among other cases, as "could not
	// open relation with OID ..." when a relation is dropped between OID
	// resolution and access, i.e. by concurrent DDL.
	errCodeInternalError = "XX000"
)

// isErrTooManyConnections returns true if err is a postgres error
// with code 53300 (too_many_connections).
//
// See: https://www.postgresql.org/docs/14/errcodes-appendix.html
func isErrTooManyConnections(err error) bool {
	return hasErrCode(err, errCodeTooManyConnections)
}

// isErrRelationNotExist  returns true if err is a postgres error
// with code 42P01 (undefined_table).
//
// See: https://www.postgresql.org/docs/14/errcodes-appendix.html
func isErrRelationNotExist(err error) bool {
	return hasErrCode(err, errCodeRelationNotExist)
}

// isErrTableNotFound reports whether err indicates a relation that does not
// exist, however it surfaced: a 42P01 (which errw wraps in
// driver.NotExistError), or the typed not-found error returned by the
// getTableMetadata / getMatviewMetadata existence gates (#945). The
// source-wide scan in getSourceMetadata uses this to tolerate a table dropped
// mid-scan by concurrent DDL.
func isErrTableNotFound(err error) bool {
	return errz.Has[*driver.NotExistError](err)
}

// isErrScanRetryable reports whether a bulk metadata-loader error is worth
// retrying during a source-wide scan of a live database: too-many-connections,
// or a relation that vanished mid-query. The latter surfaces either as 42P01,
// or as an XX000 internal error (e.g. "could not open relation with OID") when a
// relation is dropped between resolution and access. Matching XX000 by code
// (not message text) keeps this locale-independent, and retrying is safe: a
// persistent error still surfaces once the retry budget is exhausted.
func isErrScanRetryable(err error) bool {
	return isErrTooManyConnections(err) ||
		isErrRelationNotExist(err) ||
		hasErrCode(err, errCodeInternalError)
}

// hasErrCode returns true if err (or its cause error)
// is of type *pgconn.PgError and err.Number equals code.
// See: isErrTooManyConnections.
func hasErrCode(err error, code string) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		return pgErr.Code == code
	}

	return false
}

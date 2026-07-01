package postgres

import (
	"errors"
	"strings"

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

// isErrRelationDroppedMidScan reports whether err indicates a relation
// disappeared while a source-wide metadata scan was reading it: the canonical
// "relation does not exist" (42P01), or the lower-level "could not open
// relation with OID ..." (XX000) that pg_total_relation_size / regclass raise
// when a relation is dropped between OID resolution and access. A scan reads a
// live database, so it tolerates such a drop rather than failing.
func isErrRelationDroppedMidScan(err error) bool {
	if isErrRelationNotExist(err) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == errCodeInternalError &&
			strings.Contains(pgErr.Message, "could not open relation")
	}
	return false
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

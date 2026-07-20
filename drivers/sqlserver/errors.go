package sqlserver

import (
	"errors"

	mssql "github.com/microsoft/go-mssqldb"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
)

// mssql error codes
//
//nolint:lll
const (
	// See: https://docs.microsoft.com/en-us/sql/relational-databases/errors-events/database-engine-events-and-errors?view=sql-server-ver15
	errCodeIdentityInsert int32 = 544
	errCodeObjectNotExist int32 = 15009
	errCodeBadObject      int32 = 208
	errCodeViewBindingErr int32 = 4413
	errCodeDeadlockVictim int32 = 1205
	errNoIdentityColumn   int32 = 7997
)

// hasErrCode returns true if err (or its cause err) is
// of type mssql.Error and err.Number equals code.
func hasErrCode(err error, code int32) bool {
	if err == nil {
		return false
	}

	var msErr mssql.Error
	if errors.As(err, &msErr) {
		return msErr.Number == code
	}

	return false
}

// isObjectVanishedErr reports whether err indicates that an object vanished
// mid-scan because of concurrent DDL. This surfaces as error 15009 ("object
// does not exist", from sp_spaceused) when the object itself is gone, as
// error 208 ("invalid object name", e.g. from the view row-count fallback's
// SELECT COUNT(*)) when it vanishes after resolution, or as error 4413
// ("could not use view or function because of binding errors") when a view's
// underlying object is what vanished. Code matching holds whether or not err
// has been wrapped (errw maps 208 to driver.NotExistError, which unwraps to
// the mssql.Error). A bare driver.NotExistError also matches: getTableMetadata
// returns one when sp_spaceused reports NULL fields for an object dropped
// mid-call. A source-wide metadata scan of a live database tolerates all of
// these rather than failing the whole scan over one dropped object.
func isObjectVanishedErr(err error) bool {
	return hasErrCode(err, errCodeObjectNotExist) ||
		hasErrCode(err, errCodeBadObject) ||
		hasErrCode(err, errCodeViewBindingErr) ||
		errz.Has[*driver.NotExistError](err)
}

// isErrDeadlockVictim reports whether err is mssql error 1205: the query was
// chosen as a deadlock victim. Catalog metadata queries can lose a lock race
// against concurrent DDL; error 1205 is retryable by definition ("Rerun the
// transaction").
func isErrDeadlockVictim(err error) bool {
	return hasErrCode(err, errCodeDeadlockVictim)
}

// errw wraps any error from the db. It should be called at
// every interaction with the db. If err is nil, nil is returned.
// Certain errors will be wrapped in specific error types,
// e.g, errz.NotExistError.
func errw(err error) error {
	switch {
	case err == nil:
		return nil
	case hasErrCode(err, errCodeBadObject):
		return driver.NewNotExistError(err)
	default:
		var mssqlErr mssql.Error
		if errors.As(err, &mssqlErr) {
			return errz.Wrapf(err, "ERROR %d", mssqlErr.Number)
		}

		return errz.Err(err)
	}
}

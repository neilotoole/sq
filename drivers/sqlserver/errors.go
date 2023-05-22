package sqlserver

import (
	"errors"

	"github.com/neilotoole/sq/libsq/core/errz"

	mssql "github.com/microsoft/go-mssqldb"
)

// mssql error codes
//
//nolint:lll
const (
	// See: https://docs.microsoft.com/en-us/sql/relational-databases/errors-events/database-engine-events-and-errors?view=sql-server-ver15
	errCodeIdentityInsert int32 = 544
	errCodeObjectNotExist int32 = 15009
	errCodeBadObject      int32 = 208
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

// errw wraps any error from the db. It should be called at
// every interaction with the db. If err is nil, nil is returned.
// Certain errors will be wrapped in specific error types,
// e.g, errz.NotExistError.
func errw(err error) error {
	switch {
	case err == nil:
		return nil
	case hasErrCode(err, errCodeBadObject):
		return errz.NotExist(err)
	default:
		return errz.Err(err)
	}
}

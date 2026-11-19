package databricks

import (
	"errors"
	"strings"

	dbsqlerr "github.com/databricks/databricks-sql-go/errors"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
)

// sqlStateTableNotFound is the SQLSTATE that Databricks reports for a missing
// table or view (error class TABLE_OR_VIEW_NOT_FOUND).
const sqlStateTableNotFound = "42P01"

// notExistClasses are the Spark/Databricks error classes that indicate a
// missing table, view, schema or catalog. Databricks puts the class in square
// brackets at the start of the message, e.g.:
//
//	[TABLE_OR_VIEW_NOT_FOUND] The table or view `x` cannot be found.
//
// The SQLSTATE is not enough on its own, because schema and catalog errors
// share the 42704 SQLSTATE with other "undefined object" errors.
var notExistClasses = []string{
	"[TABLE_OR_VIEW_NOT_FOUND]",
	"[SCHEMA_NOT_FOUND]",
	"[CATALOG_NOT_FOUND]",
	"[NO_SUCH_CATALOG_EXCEPTION]",
	"[NO_SUCH_SCHEMA_EXCEPTION]",
}

// errw wraps any error from the database. It should be called at every
// interaction with the database to ensure consistent error handling.
//
// A nil error is returned as nil. An error indicating that a table, view,
// schema or catalog does not exist is wrapped in driver.NotExistError, so
// that callers can detect it with errors.Is. Any other error is wrapped
// with errz.Err to capture a stack trace.
func errw(err error) error {
	switch {
	case err == nil:
		return nil
	case isNotExist(err):
		return driver.NewNotExistError(err)
	default:
		return errz.Err(err)
	}
}

// isNotExist returns true if err indicates that a table, view, schema or
// catalog does not exist.
func isNotExist(err error) bool {
	if err == nil {
		return false
	}

	if execErr, ok := errors.AsType[dbsqlerr.DBExecutionError](err); ok {
		if execErr.SqlState() == sqlStateTableNotFound {
			return true
		}
	}

	msg := err.Error()
	for _, class := range notExistClasses {
		if strings.Contains(msg, class) {
			return true
		}
	}

	return false
}

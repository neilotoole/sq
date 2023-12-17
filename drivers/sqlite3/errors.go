package sqlite3

import (
	"github.com/neilotoole/sq/libsq/driver"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// errw wraps any error from the db. It should be called at
// every interaction with the db. If err is nil, nil is returned.
// Certain errors will be wrapped in specific error types,
// e.g, errz.NotExistError.
func errw(err error) error {
	switch {
	case err == nil:
		return nil
	case strings.HasPrefix(err.Error(), "no such table:"):
		// The sqlite driver always returns sqlite3.ErrError(1), so
		// we need to search by string. Needs further investigation.
		return driver.NewNotExistError(err)
	default:
		return errz.Err(err)
	}
}

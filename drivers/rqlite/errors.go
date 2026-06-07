package rqlite

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
)

// errw wraps any error from the db. It should be called at every
// interaction with the db. If err is nil, nil is returned. Certain errors
// are wrapped in specific error types, e.g. errz.NotExistError.
func errw(err error) error {
	switch {
	case err == nil:
		return nil
	case strings.Contains(err.Error(), "no such table:"):
		// rqlite returns SQLite-formatted error messages over the wire,
		// so the "no such table:" prefix carries through verbatim.
		return driver.NewNotExistError(err)
	default:
		return errz.Err(err)
	}
}

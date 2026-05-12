package duckdb

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
)

// errw wraps any error from the db. It should be called at every interaction
// with the db. If err is nil, nil is returned. Certain errors are wrapped in
// specific error types, e.g. driver.NotExistError.
func errw(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case (strings.Contains(msg, "Catalog Error: Table") ||
		strings.Contains(msg, "Catalog Error: View") ||
		strings.Contains(msg, "Catalog Error: Schema")) &&
		strings.Contains(msg, "does not exist"):
		return driver.NewNotExistError(err)
	case strings.Contains(msg, "already exists"):
		return errz.Err(err)
	default:
		return errz.Err(err)
	}
}

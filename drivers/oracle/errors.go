package oracle

import "github.com/neilotoole/sq/drivers/oracle/orshared"

// errw wraps Oracle errors, converting them to SQ error types.
func errw(err error) error {
	return orshared.Wrap(err)
}

// hasErrCode checks if error matches Oracle error code.
func hasErrCode(err error, code int) bool {
	return orshared.HasErrCode(err, code)
}

// isErrTableNotExist checks for ORA-00942.
func isErrTableNotExist(err error) bool {
	return orshared.IsErrTableNotExist(err)
}

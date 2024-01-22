// Package lgm ("log message") contains constants for log messages.
// The start of each message should be capitalized, e.g. "Close DB"
// instead of "close db".
package lgm

const (
	CloseDB               = "Close DB"
	CloseConn             = "Close SQL connection"
	CloseDBRows           = "Close DB rows"
	CloseDBStmt           = "Close DB stmt"
	CloseHTTPResponseBody = "Close HTTP response body"
	CloseFileReader       = "Close file reader"
	CloseFileWriter       = "Close file writer"
	CtxDone               = "Context unexpectedly done"
	OpenSrc               = "NewReader source"
	ReadDBRows            = "Read DB rows"
	RemoveFile            = "Remove file"
	RowsAffected          = "Rows affected"
	TxRollback            = "Rollback DB tx"
	Unexpected            = "Unexpected"
)

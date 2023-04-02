// Package lgm ("log message") contains constants for log messages.
// The start of each message should be capitalized, e.g. "Close DB"
// instead of "close db".
package lgm

const (
	CloseDB         = "Close DB"
	CloseDBRows     = "Close DB rows"
	CloseDBStmt     = "Close DB stmt"
	CloseFileReader = "Close file reader"
	CtxDone         = "Context unexpectedly done"
	OpenSrc         = "Open source"
	ReadDBRows      = "Read DB rows"
	RowsAffected    = "Rows affected"
	TxRollback      = "Rollback DB tx"
	Unexpected      = "Unexpected"
)

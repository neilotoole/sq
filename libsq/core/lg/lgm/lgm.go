// Package lgm ("log message") contains constants for log messages.
// The start of each message should be capitalized, e.g. "Close db"
// instead of "close db".
package lgm

const (
	CloseDB         = "Close db"
	CloseDBRows     = "Close db rows"
	CloseDBStmt     = "Close db stmt"
	CloseFileReader = "Close file reader"
	TxRollback      = "Rollback db tx"
	CtxDone         = "Context unexpectedly done"
	ReadDBRows      = "Read db rows"
	Unexpected      = "Unexpected"
)

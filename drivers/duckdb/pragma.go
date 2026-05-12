package duckdb

import (
	"context"
	"database/sql"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// applyConnInitSettings applies once-per-connection settings such as
// disabling the interactive progress bar that DuckDB shows for long-running
// queries. These settings are silent on a non-TTY.
func applyConnInitSettings(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`SET enable_progress_bar = false`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return errz.Wrapf(errw(err), "apply DuckDB conn init: %s", stmt)
		}
	}
	return nil
}

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

// bundledExtensions are the in-tree DuckDB extensions that we statically
// link and require to be available without network access. They are loaded
// once per connection in loadExtensions so that queries can use them
// without explicit INSTALL/LOAD.
var bundledExtensions = []string{
	"json", "parquet", "icu", "fts", "httpfs", "excel",
	"inet", "autocomplete", "tpch", "tpcds",
}

// loadExtensions installs and loads each bundled extension on a connection.
// All extensions ship in the static libduckdb bundle; INSTALL is a no-op for
// already-present extensions. LOAD makes them available to subsequent queries.
func loadExtensions(ctx context.Context, db *sql.DB) error {
	for _, ext := range bundledExtensions {
		if _, err := db.ExecContext(ctx, "INSTALL "+ext); err != nil {
			return errz.Wrapf(errw(err), "INSTALL %s", ext)
		}
		if _, err := db.ExecContext(ctx, "LOAD "+ext); err != nil {
			return errz.Wrapf(errw(err), "LOAD %s", ext)
		}
	}
	return nil
}

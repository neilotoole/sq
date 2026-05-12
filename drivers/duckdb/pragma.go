package duckdb

import (
	"context"
	"database/sql"
	"sync"

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

// installOnce ensures bundled extensions are INSTALLed exactly once per
// process. INSTALL touches the on-disk extension cache; running it from
// multiple goroutines simultaneously fails on Windows ("Could not move
// file: Access is denied") because of the stricter file-handle semantics.
// LOAD itself is per-connection and runs every time.
var (
	installOnce    sync.Once
	installOnceErr error
)

// loadExtensions installs (once per process) and loads (once per connection)
// each bundled extension. All extensions ship in the static libduckdb bundle,
// so INSTALL just registers them in DuckDB's local cache; subsequent
// connections only need LOAD.
func loadExtensions(ctx context.Context, db *sql.DB) error {
	installOnce.Do(func() {
		for _, ext := range bundledExtensions {
			if _, err := db.ExecContext(ctx, "INSTALL "+ext); err != nil {
				installOnceErr = errz.Wrapf(errw(err), "INSTALL %s", ext)
				return
			}
		}
	})
	if installOnceErr != nil {
		return installOnceErr
	}
	for _, ext := range bundledExtensions {
		if _, err := db.ExecContext(ctx, "LOAD "+ext); err != nil {
			return errz.Wrapf(errw(err), "LOAD %s", ext)
		}
	}
	return nil
}

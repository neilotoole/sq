package duckdb

import (
	"context"
	"database/sql/driver"
	"sync"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// bundledExtensions are the in-tree DuckDB extensions that we statically
// link and require to be available without network access. They are loaded
// once per connection by connInitFn so that queries can use them without
// explicit INSTALL/LOAD.
var bundledExtensions = []string{
	"json", "parquet", "icu", "fts", "httpfs", "excel",
	"inet", "autocomplete", "tpch", "tpcds",
}

// Extension install state is process-wide. INSTALL touches the on-disk
// extension cache; running it from multiple goroutines simultaneously fails
// on Windows ("Could not move file: Access is denied") because of stricter
// file-handle semantics. Memoize only on success so a transient failure
// (e.g. disk full, antivirus) doesn't permanently poison the process.
var (
	installMu       sync.Mutex
	installComplete bool
)

// installExtensions runs INSTALL once per process on success. On failure
// the next caller retries.
func installExtensions(ctx context.Context, execer driver.ExecerContext) error {
	installMu.Lock()
	defer installMu.Unlock()
	if installComplete {
		return nil
	}
	for _, ext := range bundledExtensions {
		if _, err := execer.ExecContext(ctx, "INSTALL "+ext, nil); err != nil {
			return errz.Wrapf(errw(err), "INSTALL %s", ext)
		}
	}
	installComplete = true
	return nil
}

// connInitFn is the per-connection initializer for go-duckdb's NewConnector.
// It runs on every new pooled connection: INSTALL (once per process on
// success), LOAD each extension, and SET session settings. DuckDB's LOAD
// and SET are session-scoped, so they must run per-connection.
func connInitFn(execer driver.ExecerContext) error {
	ctx := context.Background()
	if err := installExtensions(ctx, execer); err != nil {
		return err
	}
	for _, ext := range bundledExtensions {
		if _, err := execer.ExecContext(ctx, "LOAD "+ext, nil); err != nil {
			return errz.Wrapf(errw(err), "LOAD %s", ext)
		}
	}
	// Suppress DuckDB's interactive progress bar; no visible effect on
	// non-TTY connections, but prevents stray output in interactive shells.
	if _, err := execer.ExecContext(ctx, "SET enable_progress_bar = false", nil); err != nil {
		return errz.Wrapf(errw(err), "SET enable_progress_bar")
	}
	return nil
}

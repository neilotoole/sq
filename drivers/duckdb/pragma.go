package duckdb

import (
	"context"
	"database/sql/driver"
	"sync"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// bundledExtensions are the in-tree DuckDB extensions statically linked via
// duckdb-go-bindings; they must be available without network access.
// connInitFn INSTALLs them once per process and LOADs them on every pooled
// connection, so queries can use them without explicit INSTALL/LOAD.
var bundledExtensions = []string{
	"json", "parquet", "icu", "fts", "httpfs", "excel",
	"inet", "autocomplete", "tpch", "tpcds",
}

// Extension install state is process-wide. INSTALL touches the on-disk
// extension cache; running it from multiple goroutines simultaneously fails
// on Windows ("Could not move file: Access is denied") because of stricter
// file-handle semantics. The mutex + bool pattern (deliberately NOT
// sync.Once) memoizes only on success, so a transient failure (e.g. disk
// full, antivirus) does not permanently poison the process.
var (
	installMu       sync.Mutex
	installComplete bool
)

// installExtensions runs INSTALL once per process on success. On failure
// the next caller retries.
func installExtensions(execer driver.ExecerContext) error {
	installMu.Lock()
	defer installMu.Unlock()
	if installComplete {
		return nil
	}
	for _, ext := range bundledExtensions {
		if _, err := execer.ExecContext(context.Background(), "INSTALL "+ext, nil); err != nil {
			return errz.Wrapf(errw(err), "INSTALL %s", ext)
		}
	}
	installComplete = true
	return nil
}

// connInitFn is the per-connection initializer for go-duckdb's NewConnector.
// It runs on every new pooled connection: INSTALL (memoized once per process
// on success), LOAD each bundled extension, and SET session settings.
// DuckDB's LOAD and SET are session-scoped, so they must run per-connection.
//
// go-duckdb's connector init callback does not propagate the caller's
// context, so this function uses context.Background() — the user's
// --timeout will not interrupt a hung INSTALL/LOAD/SET on first connection.
func connInitFn(execer driver.ExecerContext) error {
	ctx := context.Background()
	if err := installExtensions(execer); err != nil {
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

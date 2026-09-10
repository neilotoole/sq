package duckdb

import (
	"context"
	"database/sql/driver"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// connInitFn is the per-connection initializer for go-duckdb's NewConnector.
// It runs on every new pooled connection and applies session settings.
// DuckDB's SET is session-scoped, so it must run per-connection.
//
// Extensions are deliberately NOT installed or loaded here. Only a few
// extensions (json, parquet, icu, autocomplete, core_functions) are
// statically linked into the duckdb-go-bindings engine; the rest are
// separate shared libraries that DuckDB installs into its extension
// directory (~/.duckdb by default) and loads on demand via
// autoinstall_known_extensions and autoload_known_extensions, both of which
// default to true. Eagerly running INSTALL + LOAD for the full set on every
// open was very expensive: each LOAD reads and hashes the extension file to
// verify its signature (tens of MB per open), which on Windows cost
// seconds per open. It also made every open depend on the extension
// repository being reachable, so a fresh machine could not open a DuckDB
// file offline at all. See https://github.com/neilotoole/sq/issues/1151.
//
// go-duckdb's connector init callback does not propagate the caller's
// context, so this function uses context.Background().
func connInitFn(execer driver.ExecerContext) error {
	// Suppress DuckDB's interactive progress bar; no visible effect on
	// non-TTY connections, but prevents stray output in interactive shells.
	if _, err := execer.ExecContext(context.Background(), "SET enable_progress_bar = false", nil); err != nil {
		return errz.Wrapf(errw(err), "SET enable_progress_bar")
	}
	return nil
}

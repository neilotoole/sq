package duckdb_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/duckdb"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/tu"
)

// copyToTempDuckDB copies the shared sakila DuckDB fixture to a fresh
// temp file so the test can mutate (or assert non-mutation of) the
// file without touching the shared fixture used by other tests.
func copyToTempDuckDB(t *testing.T) string {
	t.Helper()
	srcPath := proj.Abs("drivers/duckdb/testdata/sakila.duckdb")
	dstPath := filepath.Join(t.TempDir(), "sakila.duckdb")

	in, err := os.Open(srcPath)
	require.NoError(t, err)
	defer in.Close()

	out, err := os.Create(dstPath)
	require.NoError(t, err)
	// Belt-and-braces: explicit Close after Copy catches deferred fsync
	// errors, while the deferred Close ensures the fd is released if
	// require.NoError(io.Copy) fails the test mid-flight (relevant on
	// Windows where an open handle blocks subsequent file ops).
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	require.NoError(t, err)
	require.NoError(t, out.Close())
	return dstPath
}

// TestReadOnly_Concurrent_TwoOpens opens the same DuckDB file via two
// grips concurrently with read-only contexts; both must succeed.
// Without RO, DuckDB's process-exclusive write lock would reject the
// second open.
func TestReadOnly_Concurrent_TwoOpens(t *testing.T) {
	t.Parallel()
	path := copyToTempDuckDB(t)

	prov := &duckdb.Provider{Log: lgt.New(t)}
	drvr, err := prov.DriverFor(testh.DuckDBType())
	require.NoError(t, err)

	openOne := func() error {
		ctx := context.Background()
		src := testh.MakeDuckDBSource("@ro_concurrent", path)
		grip, err := drvr.Open(ctx, src, driver.ModeReadOnly)
		if err != nil {
			return err
		}
		defer grip.Close()
		_, err = grip.DB(ctx)
		return err
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			errs <- openOne()
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err, "concurrent RO opens must both succeed")
	}
}

// TestReadOnly_FileChmod0444 verifies that an RO open works against a
// file the process has read-only access to. Without RO, DuckDB's
// open-time WAL touch fails with permission denied.
func TestReadOnly_FileChmod0444(t *testing.T) {
	tu.SkipReadOnlyFileUnenforceable(t)
	t.Parallel()
	path := copyToTempDuckDB(t)

	require.NoError(t, os.Chmod(path, 0o444))
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	prov := &duckdb.Provider{Log: lgt.New(t)}
	drvr, err := prov.DriverFor(testh.DuckDBType())
	require.NoError(t, err)

	ctx := context.Background()
	src := testh.MakeDuckDBSource("@ro_chmod", path)
	grip, err := drvr.Open(ctx, src, driver.ModeReadOnly)
	require.NoError(t, err, "RO open of 0444 file must succeed")
	defer grip.Close()
}

// TestReadOnly_URLAccessModeWins verifies that a user-supplied
// ?access_mode=READ_WRITE in the location overrides the RO context
// (the documented escape hatch). Concretely: the connection must be
// writable, so we execute a CREATE TABLE and confirm no error.
func TestReadOnly_URLAccessModeWins(t *testing.T) {
	t.Parallel()
	path := copyToTempDuckDB(t)

	prov := &duckdb.Provider{Log: lgt.New(t)}
	drvr, err := prov.DriverFor(testh.DuckDBType())
	require.NoError(t, err)

	src := testh.MakeDuckDBSource("@ro_url_override", path)
	src.Location += "?access_mode=READ_WRITE"

	ctx := context.Background()
	grip, err := drvr.Open(ctx, src, driver.ModeReadOnly)
	require.NoError(t, err)
	defer grip.Close()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	// A write operation must succeed: the URL's access_mode=READ_WRITE takes
	// precedence over the RO context.
	_, err = db.ExecContext(ctx, "CREATE TABLE _rw_probe (id INTEGER)")
	require.NoError(t, err,
		"write must succeed when access_mode=READ_WRITE is explicit in URL")
}

// TestReadOnly_Explicit_ConflictWithURL verifies the driver-level
// defense-in-depth guard: an explicit read-only open against a location
// that explicitly demands write access (access_mode=READ_WRITE) is
// refused, covering callers that bypass the CLI's preemptive conflict
// check. Implicit ModeReadOnly, by contrast, lets the URL win (see
// TestReadOnly_URLAccessModeWins).
func TestReadOnly_Explicit_ConflictWithURL(t *testing.T) {
	t.Parallel()
	path := copyToTempDuckDB(t)

	prov := &duckdb.Provider{Log: lgt.New(t)}
	drvr, err := prov.DriverFor(testh.DuckDBType())
	require.NoError(t, err)

	src := testh.MakeDuckDBSource("@ro_explicit_conflict", path)
	src.Location += "?access_mode=READ_WRITE"

	ctx := context.Background()
	_, err = drvr.Open(ctx, src, driver.ModeReadOnlyExplicit)
	require.Error(t, err,
		"explicit read-only must be refused when the location demands READ_WRITE")
	require.Contains(t, err.Error(), "access_mode=READ_WRITE")
}

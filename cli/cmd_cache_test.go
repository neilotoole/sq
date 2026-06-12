package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// setupPlaceholderCSVCache adds a CSV source with handle whose location is
// an ${env:envar} placeholder backed by the sakila actor fixture, ingests
// it by executing a query, and returns the TestRun plus the path of the
// now-populated ingest cache DB (which lives under the resolved-location
// hash: Grips.doOpen hands the doc driver a resolved clone of the source).
func setupPlaceholderCSVCache(t *testing.T, handle, envar string) (tr *testrun.TestRun, cacheDB string) {
	t.Helper()

	th := testh.New(t)
	realLoc := th.Source(sakila.CSVActor).Location
	t.Setenv(envar, realLoc) // Setenv also restores the var on test cleanup.

	phSrc := &source.Source{
		Handle:   handle,
		Type:     drivertype.CSV,
		Location: "${env:" + envar + "}",
	}

	tr = testrun.New(th.Context, t, nil)
	tr.Add(*phSrc)
	require.NoError(t, tr.Exec("--csv", handle+".data"))

	resolved := phSrc.Clone()
	resolved.Location = realLoc
	_, cacheDB, _, err := tr.Run.Files.CachePaths(resolved)
	require.NoError(t, err)
	require.True(t, ioz.FileAccessible(cacheDB),
		"ingest should have populated the cache under the resolved-location hash")

	return tr, cacheDB
}

// TestCmdCacheClear_Placeholder verifies that "sq cache clear @src" clears
// the actual ingest cache for a source whose location is a ${scheme:path}
// placeholder. The ingest cache dir is hashed from the resolved location,
// so a clear that hashed the raw config location would clear a nonexistent
// dir and silently leave the real cache intact. The clear is
// hash-independent: it removes every cache dir under the handle without
// resolving anything (see Files.CacheClearSourceAll).
//
// See: https://github.com/neilotoole/sq/issues/783.
func TestCmdCacheClear_Placeholder(t *testing.T) {
	const handle = "@csv_ph"
	tr, cacheDB := setupPlaceholderCSVCache(t, handle, "SQ_TEST_CSV_LOC")

	tr = testrun.New(tr.Context, t, tr)
	require.NoError(t, tr.Exec("cache", "clear", handle))
	require.False(t, ioz.FileAccessible(cacheDB),
		"cache clear should have removed the ingest cache DB")
}

// TestCmdCacheClear_Placeholder_RotatedSecret verifies that cache clear
// removes caches ingested under a previous value of the secret: a clear
// that hashed the secret's current value would miss (and silently orphan)
// the dir created when the secret had its old value.
func TestCmdCacheClear_Placeholder_RotatedSecret(t *testing.T) {
	const (
		handle = "@csv_ph_rot"
		envar  = "SQ_TEST_CSV_LOC_ROT"
	)
	tr, cacheDB := setupPlaceholderCSVCache(t, handle, envar)

	// Rotate the secret: the resolved location no longer matches the one
	// the cache was ingested under.
	t.Setenv(envar, filepath.Join(t.TempDir(), "other.csv"))

	tr = testrun.New(tr.Context, t, tr)
	require.NoError(t, tr.Exec("cache", "clear", handle))
	require.False(t, ioz.FileAccessible(cacheDB),
		"cache clear should remove caches ingested under previous secret values")
}

// TestCmdCacheClear_Placeholder_UnresolvableSecret verifies that cache
// clear works when the secret cannot be resolved at all. Clearing is a
// local filesystem operation; it must not depend on the secret backend
// being healthy (an unavailable backend is precisely when a user may
// want to clean up).
func TestCmdCacheClear_Placeholder_UnresolvableSecret(t *testing.T) {
	const (
		handle = "@csv_ph_gone"
		envar  = "SQ_TEST_CSV_LOC_GONE"
	)
	tr, cacheDB := setupPlaceholderCSVCache(t, handle, envar)

	// The secret is now unresolvable.
	require.NoError(t, os.Unsetenv(envar))

	tr = testrun.New(tr.Context, t, tr)
	require.NoError(t, tr.Exec("cache", "clear", handle),
		"cache clear must not require the secret to be resolvable")
	require.False(t, ioz.FileAccessible(cacheDB))
}

// Note: the nested-handle protection of cache clear (clearing @prod must
// not touch @prod/db/x's cache dirs, which live below @prod's dir) is
// covered at the files level by TestFiles_CacheClearSourceAll_*: such
// nesting can no longer be created via Collection.Add or sq mv, but may
// exist in legacy or hand-edited configs.

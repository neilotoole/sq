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

// TestCmdCacheClear_Placeholder verifies that "sq cache clear @src" clears
// the actual ingest cache for a source whose location is a ${scheme:path}
// placeholder. The ingest cache dir is hashed from the resolved location
// (Grips.doOpen hands the doc driver a resolved clone), so cache clear must
// hash the resolved location too; hashing the raw config location clears a
// nonexistent dir and silently leaves the real cache intact.
//
// See: https://github.com/neilotoole/sq/issues/783.
func TestCmdCacheClear_Placeholder(t *testing.T) {
	const handle = "@csv_ph"

	th := testh.New(t)
	realLoc := th.Source(sakila.CSVActor).Location
	t.Setenv("SQ_TEST_CSV_LOC", realLoc)

	phSrc := &source.Source{
		Handle:   handle,
		Type:     drivertype.CSV,
		Location: "${env:SQ_TEST_CSV_LOC}",
	}

	tr := testrun.New(th.Context, t, nil)
	tr.Add(*phSrc)

	// Query the source, ingesting it into the cache.
	require.NoError(t, tr.Exec("--csv", handle+".data"))

	resolved := phSrc.Clone()
	resolved.Location = realLoc
	_, cacheDB, _, err := tr.Run.Files.CachePaths(resolved)
	require.NoError(t, err)
	require.True(t, ioz.FileAccessible(cacheDB),
		"ingest should have populated the cache under the resolved-location hash")

	tr = testrun.New(th.Context, t, tr)
	require.NoError(t, tr.Exec("cache", "clear", handle))
	require.False(t, ioz.FileAccessible(cacheDB),
		"cache clear should have removed the ingest cache DB")
}

// TestCmdCacheClear_Placeholder_RotatedSecret verifies that cache clear
// removes caches ingested under a previous value of the secret. The cache
// dir leaf is keyed on a hash of the resolved location, so a clear that
// hashes the secret's current value would miss (and silently orphan) the
// dir created when the secret had its old value.
func TestCmdCacheClear_Placeholder_RotatedSecret(t *testing.T) {
	const (
		handle = "@csv_ph_rot"
		envar  = "SQ_TEST_CSV_LOC_ROT"
	)

	th := testh.New(t)
	realLoc := th.Source(sakila.CSVActor).Location
	t.Setenv(envar, realLoc)

	phSrc := &source.Source{
		Handle:   handle,
		Type:     drivertype.CSV,
		Location: "${env:" + envar + "}",
	}

	tr := testrun.New(th.Context, t, nil)
	tr.Add(*phSrc)
	require.NoError(t, tr.Exec("--csv", handle+".data"))

	resolved := phSrc.Clone()
	resolved.Location = realLoc
	_, cacheDB, _, err := tr.Run.Files.CachePaths(resolved)
	require.NoError(t, err)
	require.True(t, ioz.FileAccessible(cacheDB))

	// Rotate the secret: the resolved location no longer matches the one
	// the cache was ingested under.
	t.Setenv(envar, filepath.Join(t.TempDir(), "other.csv"))

	tr = testrun.New(th.Context, t, tr)
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

	th := testh.New(t)
	realLoc := th.Source(sakila.CSVActor).Location
	t.Setenv(envar, realLoc) // Setenv also restores the var on test cleanup.

	phSrc := &source.Source{
		Handle:   handle,
		Type:     drivertype.CSV,
		Location: "${env:" + envar + "}",
	}

	tr := testrun.New(th.Context, t, nil)
	tr.Add(*phSrc)
	require.NoError(t, tr.Exec("--csv", handle+".data"))

	resolved := phSrc.Clone()
	resolved.Location = realLoc
	_, cacheDB, _, err := tr.Run.Files.CachePaths(resolved)
	require.NoError(t, err)
	require.True(t, ioz.FileAccessible(cacheDB))

	// The secret is now unresolvable.
	require.NoError(t, os.Unsetenv(envar))

	tr = testrun.New(th.Context, t, tr)
	require.NoError(t, tr.Exec("cache", "clear", handle),
		"cache clear must not require the secret to be resolvable")
	require.False(t, ioz.FileAccessible(cacheDB))
}

// TestCmdCacheClear_NestedHandleCachePreserved verifies that clearing a
// source's cache doesn't touch the cache of a source whose handle nests
// under it: @prod and @prod/db/x can coexist (a handle is only checked
// against its immediate group), and @prod/db/x's cache dirs live below
// @prod's cache dir on disk.
func TestCmdCacheClear_NestedHandleCachePreserved(t *testing.T) {
	th := testh.New(t)
	loc := th.Source(sakila.CSVActor).Location

	parent := &source.Source{Handle: "@prod", Type: drivertype.CSV, Location: loc}
	nested := &source.Source{Handle: "@prod/db/x", Type: drivertype.CSV, Location: loc}

	tr := testrun.New(th.Context, t, nil)
	tr.Add(*parent, *nested)
	require.NoError(t, tr.Exec("--csv", parent.Handle+".data"))
	tr = testrun.New(th.Context, t, tr)
	require.NoError(t, tr.Exec("--csv", nested.Handle+".data"))

	_, parentDB, _, err := tr.Run.Files.CachePaths(parent)
	require.NoError(t, err)
	_, nestedDB, _, err := tr.Run.Files.CachePaths(nested)
	require.NoError(t, err)
	require.True(t, ioz.FileAccessible(parentDB))
	require.True(t, ioz.FileAccessible(nestedDB))

	tr = testrun.New(th.Context, t, tr)
	require.NoError(t, tr.Exec("cache", "clear", parent.Handle))
	require.False(t, ioz.FileAccessible(parentDB))
	require.True(t, ioz.FileAccessible(nestedDB),
		"clearing @prod must not clear @prod/db/x's cache")
}

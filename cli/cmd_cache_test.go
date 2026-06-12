package cli_test

import (
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

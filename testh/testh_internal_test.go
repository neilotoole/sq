package testh

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestScratchTableNamePatterns verifies the name patterns that DiffDB and the
// stale scratch-table sweep rely on. The sweep drops tables, so it is vital
// that staleScratchTableNameRe matches only harness-generated names: it must
// match every stringz.UniqTableName result, and must never match a sakila
// table or view name.
func TestScratchTableNamePatterns(t *testing.T) {
	for i := 0; i < 100; i++ {
		name := stringz.UniqTableName("actor")
		require.True(t, isScratchTableName(name), "UniqTableName %q", name)
		require.True(t, staleScratchTableNameRe.MatchString(name), "UniqTableName %q", name)

		// Oracle stores unquoted identifiers upper-case.
		upper := strings.ToUpper(name)
		require.True(t, isScratchTableName(upper), "upper-cased %q", upper)
		require.True(t, staleScratchTableNameRe.MatchString(upper), "upper-cased %q", upper)

		// UniqSuffix names (single underscore) are matched by the broad
		// DiffDB pattern, but deliberately NOT by the sweep pattern.
		suffixed := stringz.UniqSuffix("test_tbl")
		require.True(t, isScratchTableName(suffixed), "UniqSuffix %q", suffixed)
		require.False(t, staleScratchTableNameRe.MatchString(suffixed), "UniqSuffix %q", suffixed)
	}

	stable := sakila.AllTblsViews()
	stable = append(stable, "sales_by_film_category", "actor_info", "type_test")
	for _, name := range stable {
		require.False(t, isScratchTableName(name), "stable name %q", name)
		require.False(t, staleScratchTableNameRe.MatchString(name), "stable name %q", name)
		upper := strings.ToUpper(name)
		require.False(t, isScratchTableName(upper), "stable name %q", upper)
		require.False(t, staleScratchTableNameRe.MatchString(upper), "stable name %q", upper)
	}
}

package sakila_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestEmbedded verifies that the embedded SQL handles are exactly SQLite and
// DuckDB, and that IsEmbedded agrees with both Embedded and SQLAllExternal.
func TestEmbedded(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{sakila.SL3, sakila.Duck}, sakila.Embedded())

	for _, h := range sakila.Embedded() {
		assert.True(t, sakila.IsEmbedded(h), "%s should be embedded", h)
	}
	// Every external SQL source (including rqlite, which is SQLite-backed but
	// reached over the network) must NOT be classified as embedded.
	for _, h := range sakila.SQLAllExternal() {
		assert.False(t, sakila.IsEmbedded(h), "%s should not be embedded", h)
	}
	assert.False(t, sakila.IsEmbedded(sakila.RQ), "rqlite is external, not embedded")
}

// TestCrossSourceDests asserts the cross-source pairing invariants that keep
// the test matrix at {embedded}x{target} + self-inserts (see gh #964).
func TestCrossSourceDests(t *testing.T) {
	t.Parallel()

	// Embedded origins pair with every engine.
	for _, origin := range sakila.Embedded() {
		assert.Equal(t, sakila.SQLLatest(), sakila.CrossSourceDests(origin),
			"embedded origin %s should pair with all of SQLLatest", origin)
	}

	// External origins pair with exactly the embedded sources plus themselves.
	for _, origin := range sakila.SQLLatest() {
		if sakila.IsEmbedded(origin) {
			continue
		}
		dests := sakila.CrossSourceDests(origin)
		assert.ElementsMatch(t, append(sakila.Embedded(), origin), dests,
			"external origin %s should pair with embedded sources + itself", origin)
	}

	// Whole-matrix invariants over SQLLatest x CrossSourceDests.
	var sawExternalAsOrigin, sawExternalAsDest bool
	for _, origin := range sakila.SQLLatest() {
		for _, dest := range sakila.CrossSourceDests(origin) {
			// No external x external CROSS pair (the O(N^2) cells #964 drops).
			if !sakila.IsEmbedded(origin) && !sakila.IsEmbedded(dest) {
				assert.Equal(t, origin, dest,
					"external pair %s->%s must be a self-insert, not a cross pair", origin, dest)
			}
			// Every external engine is exercised against an embedded source in
			// both directions (origin and dest), preserving per-engine coverage.
			if !sakila.IsEmbedded(origin) && sakila.IsEmbedded(dest) {
				sawExternalAsOrigin = true
			}
			if sakila.IsEmbedded(origin) && !sakila.IsEmbedded(dest) {
				sawExternalAsDest = true
			}
		}
	}
	assert.True(t, sawExternalAsOrigin, "external engines should appear as origin paired with embedded")
	assert.True(t, sawExternalAsDest, "external engines should appear as dest paired with embedded")

	// The external self-insert diagonal is retained for every external engine.
	for _, h := range sakila.SQLLatest() {
		if sakila.IsEmbedded(h) {
			continue
		}
		assert.True(t, slices.Contains(sakila.CrossSourceDests(h), h),
			"external self-insert %s->%s should be retained", h, h)
	}
}

// TestSakila_SQL is a sanity check for Sakila SQL test sources.
func TestSakila_SQL(t *testing.T) { //nolint:tparallel
	// Verify that the latest-version aliases are as expected
	require.Equal(t, "@sakila_pg", sakila.Pg)
	require.Equal(t, "@sakila_my", sakila.My)
	require.Equal(t, "@sakila_ms", sakila.MS)
	require.Equal(t, "@sakila_ch", sakila.CH)
	require.Equal(t, "@sakila_or", sakila.Ora)
	require.Equal(t, "@sakila_rq", sakila.RQ)

	handles := sakila.SQLAll()
	for _, handle := range handles {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			sink, err := th.QuerySQL(src, nil, "SELECT * FROM actor")
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

// TestSakila_XLSX is a sanity check for Sakila XLSX test sources.
func TestSakila_XLSX(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH371ExcelSlowWin)

	handles := []string{sakila.XLSXSubset}
	// TODO: Append sakila.XLSX to handles when performance is reasonable
	//  enough not to break CI.

	for _, handle := range handles {
		t.Run(handle, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(handle)

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM actor")
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

// TestSakila_CSV is a sanity check for Sakila CSV/TSV test sources.
func TestSakila_CSV(t *testing.T) {
	t.Parallel()

	handles := []string{sakila.CSVActor, sakila.CSVActorNoHeader, sakila.TSVActor, sakila.TSVActorNoHeader}
	for _, handle := range handles {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			// Note table "data" instead of "actor", because CSV is monotable
			sink, err := th.QuerySQL(src, nil, "SELECT * FROM data")
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

package rqlite_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/rqlite"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestSchemaAndCatalogMethods exercises the schema/catalog/metadata-list
// methods against the bundled Sakila rqlite database. These paths are
// not hit by the higher-level metadata helpers, so they need direct
// driver-level coverage.
func TestSchemaAndCatalogMethods(t *testing.T) {
	tu.SkipShort(t, true)

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	require.Equal(t, src, grip.Source())

	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	drvr := grip.SQLDriver()
	ctx := th.Context

	t.Run("DBProperties", func(t *testing.T) {
		props, err := drvr.DBProperties(ctx, db)
		require.NoError(t, err)
		require.NotEmpty(t, props["sqlite_version"])
	})

	t.Run("CurrentSchema", func(t *testing.T) {
		schema, err := drvr.CurrentSchema(ctx, db)
		require.NoError(t, err)
		require.Equal(t, "main", schema)
	})

	t.Run("SchemaExists", func(t *testing.T) {
		exists, err := drvr.SchemaExists(ctx, db, "main")
		require.NoError(t, err)
		require.True(t, exists)

		exists, err = drvr.SchemaExists(ctx, db, "")
		require.NoError(t, err)
		require.False(t, exists)

		exists, err = drvr.SchemaExists(ctx, db, "no_such_schema")
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("ListSchemas", func(t *testing.T) {
		schemas, err := drvr.ListSchemas(ctx, db)
		require.NoError(t, err)
		require.Contains(t, schemas, "main")
	})

	t.Run("ListSchemaMetadata", func(t *testing.T) {
		schemas, err := drvr.ListSchemaMetadata(ctx, db)
		require.NoError(t, err)
		require.NotEmpty(t, schemas)
		var main *metadata.Schema
		for _, s := range schemas {
			if s.Name == "main" {
				main = s
				break
			}
		}
		require.NotNil(t, main)
		require.Equal(t, "default", main.Catalog)
	})

	t.Run("ListTableNames", func(t *testing.T) {
		// tables only
		names, err := drvr.ListTableNames(ctx, db, "", true, false)
		require.NoError(t, err)
		require.Contains(t, names, sakila.TblActor)

		// views only
		views, err := drvr.ListTableNames(ctx, db, "", false, true)
		require.NoError(t, err)
		require.NotContains(t, views, sakila.TblActor)

		// tables and views
		both, err := drvr.ListTableNames(ctx, db, "", true, true)
		require.NoError(t, err)
		require.Contains(t, both, sakila.TblActor)

		// neither -> empty
		none, err := drvr.ListTableNames(ctx, db, "", false, false)
		require.NoError(t, err)
		require.Empty(t, none)

		// schema-qualified
		qualified, err := drvr.ListTableNames(ctx, db, "main", true, false)
		require.NoError(t, err)
		require.Contains(t, qualified, sakila.TblActor)
	})

	t.Run("TableExists", func(t *testing.T) {
		exists, err := drvr.TableExists(ctx, db, sakila.TblActor)
		require.NoError(t, err)
		require.True(t, exists)

		exists, err = drvr.TableExists(ctx, db, "no_such_table")
		require.NoError(t, err)
		require.False(t, exists)
	})
}

// TestRenderFuncs_Integration exercises the SLQ string-function
// renderers (contains/startswith/endswith and their case-insensitive
// variants, plus like/ilike) end-to-end against the Sakila rqlite DB.
// These renderers are registered in Renderer() but are otherwise only
// covered by the cross-driver tests in the libsq package, which don't
// count toward this package's coverage.
func TestRenderFuncs_Integration(t *testing.T) {
	tu.SkipShort(t, true)

	th := testh.New(t)
	_ = th.Source(sakila.Rq)

	testCases := []struct {
		name  string
		query string
	}{
		{"contains", `@sakila_rq | .actor | where(contains(.first_name, "AN"))`},
		{"startswith", `@sakila_rq | .actor | where(startswith(.first_name, "PEN"))`},
		{"endswith", `@sakila_rq | .actor | where(endswith(.first_name, "AN"))`},
		{"endswith_empty", `@sakila_rq | .actor | where(endswith(.first_name, ""))`},
		{"icontains", `@sakila_rq | .actor | where(icontains(.first_name, "an"))`},
		{"istartswith", `@sakila_rq | .actor | where(istartswith(.first_name, "pen"))`},
		{"iendswith", `@sakila_rq | .actor | where(iendswith(.first_name, "an"))`},
		{"like", `@sakila_rq | .actor | where(like(.first_name, "PEN%"))`},
		{"ilike", `@sakila_rq | .actor | where(ilike(.first_name, "pen%"))`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sink, err := th.QuerySLQ(tc.query, nil)
			require.NoError(t, err)
			require.NotNil(t, sink)
		})
	}
}

// TestGetTblRowCounts_MissingTableFallback deterministically exercises
// the concurrent-DROP fallback in getTblRowCounts: when the batched
// UNION ALL COUNT(*) fails with "no such table", it falls back to
// per-table COUNTs (countTblsIndividually), recording -1 for any table
// that has vanished. Here a name that never existed stands in for one
// dropped mid-flight.
func TestGetTblRowCounts_MissingTableFallback(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	counts, err := rqlite.GetTblRowCounts(th.Context, db,
		[]string{sakila.TblActor, "no_such_table_zzz"})
	require.NoError(t, err)
	require.Len(t, counts, 2)
	require.Equal(t, int64(sakila.TblActorCount), counts[0])
	require.Equal(t, int64(-1), counts[1], "vanished table records -1")
}

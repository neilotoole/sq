package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

func pgIndexByName(idxs []*metadata.Index, name string) *metadata.Index {
	for _, idx := range idxs {
		if idx.Name == name {
			return idx
		}
	}
	return nil
}

// TestIndexes_ExpressionArity_Postgres verifies that a Postgres
// expression key (attnum=0 in pg_index.indkey) is preserved as an
// empty-string sentinel, and that an all-expression index is omitted.
func TestIndexes_ExpressionArity_Postgres(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("idx_arity")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" (a INT, b TEXT, c INT)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	_, err = db.ExecContext(th.Context,
		"CREATE INDEX ix_mixed ON "+tbl+" (a, lower(b), c)")
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context,
		"CREATE INDEX ix_allexpr ON "+tbl+" (lower(b))")
	require.NoError(t, err)

	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)

	mixed := pgIndexByName(md.Indexes, "ix_mixed")
	require.NotNil(t, mixed, "ix_mixed should be reported")
	require.Equal(t, []string{"a", "", "c"}, mixed.Columns,
		"the lower(b) key position must be the empty-string sentinel")
	require.Nil(t, pgIndexByName(md.Indexes, "ix_allexpr"),
		"an all-expression index must be omitted")
}

// TestIndexes_IncludeFilter_Postgres pins that INCLUDE/covering columns
// are excluded from Index.Columns (only key columns appear).
func TestIndexes_IncludeFilter_Postgres(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("idx_include")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" (k INT, extra TEXT)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	_, err = db.ExecContext(th.Context,
		"CREATE UNIQUE INDEX ix_inc ON "+tbl+" (k) INCLUDE (extra)")
	require.NoError(t, err)

	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)

	inc := pgIndexByName(md.Indexes, "ix_inc")
	require.NotNil(t, inc)
	require.Equal(t, []string{"k"}, inc.Columns,
		"INCLUDE column 'extra' must not appear in Index.Columns")
}

package sqlite3_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/vector"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/tu"
)

func vecSrc(t *testing.T, handle, file string) (*testh.Helper, *source.Source) {
	t.Helper()
	th := testh.New(t)
	src := th.Add(&source.Source{
		Handle:   handle,
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + tu.MustAbsFilepath("testdata", file),
	})
	return th, src
}

func colByName(t *testing.T, tblMeta *metadata.Table, name string) *metadata.Column {
	t.Helper()
	for _, col := range tblMeta.Columns {
		if col.Name == name {
			return col
		}
	}
	require.FailNow(t, "column not found", name)
	return nil
}

func TestVector_Vec0TableMetadata(t *testing.T) {
	th, src := vecSrc(t, "@vec", "vec.db")

	t.Run("doc_vec", func(t *testing.T) {
		tblMeta, err := th.TableMetadata(src, "doc_vec")
		require.NoError(t, err)
		require.Equal(t, sqlz.TableTypeVirtual, tblMeta.TableType)

		emb := colByName(t, tblMeta, "embedding")
		require.Equal(t, kind.Vector, emb.Kind)
		require.Equal(t, "float[4]", emb.ColumnType)
		require.Equal(t, &vector.Type{Dim: 4, Elem: vector.F32, Metric: "cosine"}, emb.Vector)

		require.True(t, colByName(t, tblMeta, "doc_id").PrimaryKey)
		require.Equal(t, "auxiliary", colByName(t, tblMeta, "note").Role)
		require.Equal(t, "metadata", colByName(t, tblMeta, "category").Role)
		require.Empty(t, emb.Role)
	})

	t.Run("bin_vec", func(t *testing.T) {
		tblMeta, err := th.TableMetadata(src, "bin_vec")
		require.NoError(t, err)
		require.Equal(t, &vector.Type{Dim: 4, Elem: vector.I8}, colByName(t, tblMeta, "e_i8").Vector)
		require.Equal(t, &vector.Type{Dim: 8, Elem: vector.Bit}, colByName(t, tblMeta, "e_bit").Vector)
	})

	t.Run("part_vec", func(t *testing.T) {
		tblMeta, err := th.TableMetadata(src, "part_vec")
		require.NoError(t, err)
		require.Equal(t, "partition_key", colByName(t, tblMeta, "user_id").Role)
	})

	t.Run("empty_vec", func(t *testing.T) {
		tblMeta, err := th.TableMetadata(src, "empty_vec")
		require.NoError(t, err)
		col := colByName(t, tblMeta, "e")
		require.Equal(t, "float[3]", col.ColumnType)
		require.Equal(t, kind.Vector, col.Kind)
	})

	t.Run("source_metadata_matches", func(t *testing.T) {
		srcMeta, err := th.SourceMetadata(src)
		require.NoError(t, err)
		for _, name := range []string{"doc_vec", "bin_vec", "part_vec", "empty_vec"} {
			want, err := th.TableMetadata(src, name)
			require.NoError(t, err)
			got := srcMeta.Table(name)
			require.NotNil(t, got, name)
			require.Equal(t, want.Columns, got.Columns, name)
		}
	})
}

func TestVector_LibSQLMetadata(t *testing.T) {
	th, src := vecSrc(t, "@libsql", "libsql.db")

	check := func(t *testing.T, tblMeta *metadata.Table) {
		t.Helper()
		e32 := colByName(t, tblMeta, "e32")
		require.Equal(t, kind.Vector, e32.Kind)
		require.Equal(t, "F32_BLOB(4)", e32.ColumnType)
		require.Equal(t, &vector.Type{Dim: 4, Elem: vector.F32}, e32.Vector)

		f := colByName(t, tblMeta, "f")
		require.Equal(t, kind.Vector, f.Kind)
		require.Equal(t, &vector.Type{Dim: 3, Elem: vector.F32}, f.Vector)

		require.Equal(t, kind.Int, colByName(t, tblMeta, "id").Kind)
		require.Nil(t, colByName(t, tblMeta, "id").Vector)
	}

	tblMeta, err := th.TableMetadata(src, "t")
	require.NoError(t, err)
	check(t, tblMeta)

	srcMeta, err := th.SourceMetadata(src)
	require.NoError(t, err)
	got := srcMeta.Table("t")
	require.NotNil(t, got)
	check(t, got)
	require.Equal(t, tblMeta.Columns, got.Columns)
}

func TestVTab_Unopenable(t *testing.T) {
	th, src := vecSrc(t, "@vtab_unopenable", "vtab_unopenable.db")

	srcMeta, err := th.SourceMetadata(src)
	require.NoError(t, err)

	var names []string
	for _, tbl := range srcMeta.Tables {
		names = append(names, tbl.Name)
	}
	require.Equal(t, []string{"fx_other", "fx_rescore", "plain"}, names)

	plain := srcMeta.Table("plain")
	require.Equal(t, int64(2), plain.RowCount)
	require.Empty(t, plain.Error)

	rescore := srcMeta.Table("fx_rescore")
	require.Equal(t, sqlz.TableTypeVirtual, rescore.TableType)
	require.NotEmpty(t, rescore.Error)
	require.Zero(t, rescore.RowCount)
	require.Len(t, rescore.Columns, 1)
	emb := colByName(t, rescore, "embedding")
	require.Equal(t, kind.Vector, emb.Kind)
	require.Equal(t, &vector.Type{Dim: 384, Elem: vector.F32, Metric: "cosine"}, emb.Vector)

	other := srcMeta.Table("fx_other")
	require.Equal(t, sqlz.TableTypeVirtual, other.TableType)
	require.Contains(t, other.Error, "no such module: vectorlite")
	require.Empty(t, other.Columns)

	for _, name := range []string{"fx_other", "fx_rescore"} {
		tblMeta, err := th.TableMetadata(src, name)
		require.NoError(t, err, name)
		require.Equal(t, srcMeta.Table(name).Error, tblMeta.Error, name)
		require.Equal(t, srcMeta.Table(name).Columns, tblMeta.Columns, name)
		require.Equal(t, sqlz.TableTypeVirtual, tblMeta.TableType, name)
	}

	tblMeta, err := th.TableMetadata(src, "plain")
	require.NoError(t, err)
	require.Empty(t, tblMeta.Error)
}

func TestVTab_UnopenableOnly(t *testing.T) {
	th, src := vecSrc(t, "@vtab_unopenable_only", "vtab_unopenable_only.db")

	srcMeta, err := th.SourceMetadata(src)
	require.NoError(t, err)
	require.Len(t, srcMeta.Tables, 1)
	require.Equal(t, "fx_other", srcMeta.Tables[0].Name)
	require.Contains(t, srcMeta.Tables[0].Error, "no such module: vectorlite")
}

// TestVTab_ContextCancelNotUnopenable verifies that a cancelled context
// fails metadata collection instead of marking healthy virtual tables
// as unopenable.
func TestVTab_ContextCancelNotUnopenable(t *testing.T) {
	th, src := vecSrc(t, "@sakila_fts5", "sakila_fts5.db")
	grip, err := th.Grips().Open(th.Context, src, driver.ModeReadWrite)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(th.Context)
	cancel()

	_, err = grip.SourceMetadata(ctx, false)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)

	_, err = grip.TableMetadata(ctx, "actor_fts")
	require.Error(t, err)
}

func TestLibSQL_RowCounts(t *testing.T) {
	th, src := vecSrc(t, "@lib", "libsql.db")

	srcMeta, err := th.SourceMetadata(src)
	require.NoError(t, err)
	require.Equal(t, int64(2), srcMeta.Table("t").RowCount)
	require.Equal(t, int64(1), srcMeta.Table("u").RowCount)

	tblMeta, err := th.TableMetadata(src, "t")
	require.NoError(t, err)
	require.Equal(t, int64(2), tblMeta.RowCount)
}

func TestLibSQL_SLQNotIndexed(t *testing.T) {
	th, src := vecSrc(t, "@lib", "libsql.db")

	sink, err := th.QuerySLQ(`@lib.t | .id`, nil)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 2)

	sink, err = th.QuerySLQ(`@lib.t | count`, nil)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 1)
	require.EqualValues(t, 2, sink.Recs[0][0])

	sink, err = th.QuerySLQ(`@lib.t:a | .a.id`, nil)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 2)

	qc := &libsq.QueryContext{Collection: th.NewCollection(src.Handle), Grips: th.Grips()}
	res, err := libsq.SLQ2SQL(th.Context, qc, `@lib.t:a | .a.id`)
	require.NoError(t, err)
	require.Contains(t, res.SQL, `AS "a" NOT INDEXED`)

	sink, err = th.QuerySLQ(`@lib.t | join(@lib.u, .id) | .t.id`, nil)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 1)

	res, err = libsq.SLQ2SQL(th.Context, qc, `@lib.t | join(@lib.u, .id) | .t.id`)
	require.NoError(t, err)
	require.Contains(t, res.SQL, `FROM "t" NOT INDEXED`)
	require.Contains(t, res.SQL, `JOIN "u" NOT INDEXED`)
}

func TestLibSQL_IndexMetadata(t *testing.T) {
	th, src := vecSrc(t, "@lib", "libsql.db")

	idxByName := func(tblMeta *metadata.Table, name string) *metadata.Index {
		for _, idx := range tblMeta.Indexes {
			if idx.Name == name {
				return idx
			}
		}
		return nil
	}

	tblMeta, err := th.TableMetadata(src, "t")
	require.NoError(t, err)
	idx := idxByName(tblMeta, "t_idx")
	require.NotNil(t, idx)
	require.Equal(t, []string{"e32"}, idx.Columns)
	require.False(t, idx.Unique)
	require.Equal(t, "t", idx.Table)
	require.Equal(t, &metadata.IndexVector{Method: "diskann", Metric: "cosine"}, idx.Vector)

	tblMeta, err = th.TableMetadata(src, "u")
	require.NoError(t, err)
	idx = idxByName(tblMeta, "u_idx")
	require.NotNil(t, idx)
	require.Equal(t, []string{"e"}, idx.Columns)
	require.Equal(t, map[string]string{
		"compress_neighbors": "float1bit",
		"max_neighbors":      "8",
	}, idx.Vector.Params)
}

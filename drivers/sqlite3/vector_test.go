package sqlite3_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/vector"
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

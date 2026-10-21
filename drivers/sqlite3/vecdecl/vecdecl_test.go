package vecdecl_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3/vecdecl"
	"github.com/neilotoole/sq/libsq/core/vector"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

func TestParseVec0(t *testing.T) {
	t.Run("mixed", func(t *testing.T) {
		cols, err := vecdecl.ParseVec0("CREATE VIRTUAL TABLE doc_vec USING vec0(" +
			"doc_id INTEGER PRIMARY KEY, " +
			"embedding float[4] distance_metric=cosine, category TEXT, +note TEXT)")
		require.NoError(t, err)
		require.Len(t, cols, 4)

		require.Equal(t, "doc_id", cols[0].Name)
		require.Equal(t, vecdecl.RolePrimaryKey, cols[0].Role)
		require.Equal(t, "INTEGER", cols[0].DeclType)
		require.Nil(t, cols[0].Vector)

		require.Equal(t, "embedding", cols[1].Name)
		require.Equal(t, vecdecl.RoleVector, cols[1].Role)
		require.Equal(t, "float[4]", cols[1].DeclType)
		require.Equal(t, &vector.Type{Dim: 4, Elem: vector.F32, Metric: "cosine"}, cols[1].Vector)

		require.Equal(t, "category", cols[2].Name)
		require.Equal(t, vecdecl.RoleMetadata, cols[2].Role)
		require.Equal(t, "TEXT", cols[2].DeclType)

		require.Equal(t, "note", cols[3].Name)
		require.Equal(t, vecdecl.RoleAuxiliary, cols[3].Role)
		require.Equal(t, "TEXT", cols[3].DeclType)
	})

	t.Run("elem_types", func(t *testing.T) {
		cols, err := vecdecl.ParseVec0("CREATE VIRTUAL TABLE v USING vec0(" +
			"a int8[4], b bit[8], c i8[2], d f32[3], e FLOAT[5])")
		require.NoError(t, err)
		require.Len(t, cols, 5)
		require.Equal(t, vector.Type{Dim: 4, Elem: vector.I8}, *cols[0].Vector)
		require.Equal(t, vector.Type{Dim: 8, Elem: vector.Bit}, *cols[1].Vector)
		require.Equal(t, vector.Type{Dim: 2, Elem: vector.I8}, *cols[2].Vector)
		require.Equal(t, vector.Type{Dim: 3, Elem: vector.F32}, *cols[3].Vector)
		require.Equal(t, vector.Type{Dim: 5, Elem: vector.F32}, *cols[4].Vector)
	})

	t.Run("roles", func(t *testing.T) {
		cols, err := vecdecl.ParseVec0("CREATE VIRTUAL TABLE v USING vec0(" +
			"user_id INTEGER PARTITION KEY, id TEXT PRIMARY KEY, e float[2], flag boolean)")
		require.NoError(t, err)
		require.Len(t, cols, 4)
		require.Equal(t, vecdecl.RolePartitionKey, cols[0].Role)
		require.Equal(t, "user_id", cols[0].Name)
		require.Equal(t, vecdecl.RolePrimaryKey, cols[1].Role)
		require.Equal(t, "id", cols[1].Name)
		require.Equal(t, vecdecl.RoleMetadata, cols[3].Role)
		require.Equal(t, "boolean", cols[3].DeclType)
	})

	t.Run("indexed_by", func(t *testing.T) {
		cols, err := vecdecl.ParseVec0("CREATE VIRTUAL TABLE vec_history USING vec0(" +
			"embedding FLOAT[384] distance_metric=cosine INDEXED BY rescore(quantizer=bit, oversample=50))")
		require.NoError(t, err)
		require.Len(t, cols, 1)
		require.Equal(t, vecdecl.RoleVector, cols[0].Role)
		require.Equal(t, "FLOAT[384]", cols[0].DeclType)
		require.Equal(t, "rescore(quantizer=bit, oversample=50)", cols[0].IndexedBy)
		require.Equal(t, vector.Type{Dim: 384, Elem: vector.F32, Metric: "cosine"}, *cols[0].Vector)
	})

	t.Run("quoted_and_case", func(t *testing.T) {
		cols, err := vecdecl.ParseVec0(`create virtual table "my vecs" USING VEC0 ( "Emb"  float[3] )`)
		require.NoError(t, err)
		require.Len(t, cols, 1)
		require.Equal(t, "Emb", cols[0].Name)
		require.Equal(t, vector.Type{Dim: 3, Elem: vector.F32}, *cols[0].Vector)
	})

	t.Run("bracket_backtick_quotes", func(t *testing.T) {
		cols, err := vecdecl.ParseVec0("CREATE VIRTUAL TABLE v USING vec0([a b] float[2] DISTANCE_METRIC=L2, `c` text)")
		require.NoError(t, err)
		require.Len(t, cols, 2)
		require.Equal(t, "a b", cols[0].Name)
		require.Equal(t, "l2", cols[0].Vector.Metric)
		require.Equal(t, "c", cols[1].Name)
	})

	t.Run("table_option_skipped", func(t *testing.T) {
		cols, err := vecdecl.ParseVec0("CREATE VIRTUAL TABLE v USING vec0(e float[4], chunk_size=8)")
		require.NoError(t, err)
		require.Len(t, cols, 1)
		require.Equal(t, "e", cols[0].Name)
	})

	t.Run("line_comment_apostrophe", func(t *testing.T) {
		cols, err := vecdecl.ParseVec0("CREATE VIRTUAL TABLE v USING vec0(\n" +
			"  doc_id INTEGER PRIMARY KEY, -- the doc's id\n" +
			"  embedding float[2]\n)")
		require.NoError(t, err)
		require.Len(t, cols, 2)
		require.Equal(t, "doc_id", cols[0].Name)
		require.Equal(t, vecdecl.RolePrimaryKey, cols[0].Role)
		require.Equal(t, "embedding", cols[1].Name)
		require.Equal(t, vector.Type{Dim: 2, Elem: vector.F32}, *cols[1].Vector)
	})

	t.Run("comments_with_commas", func(t *testing.T) {
		cols, err := vecdecl.ParseVec0("CREATE VIRTUAL TABLE /* the, table */ v USING vec0(\n" +
			"  e float[2] /* dims, metric */ distance_metric=cosine, -- first, vector\n" +
			"  category TEXT -- trailing, comment\n" +
			")")
		require.NoError(t, err)
		require.Len(t, cols, 2)
		require.Equal(t, "e", cols[0].Name)
		require.Equal(t, vector.Type{Dim: 2, Elem: vector.F32, Metric: "cosine"}, *cols[0].Vector)
		require.Equal(t, "category", cols[1].Name)
		require.Equal(t, vecdecl.RoleMetadata, cols[1].Role)
		require.Equal(t, "TEXT", cols[1].DeclType)
	})

	t.Run("comment_markers_in_quotes", func(t *testing.T) {
		cols, err := vecdecl.ParseVec0(`CREATE VIRTUAL TABLE v USING vec0("a--b" float[2], "c/*d" text)`)
		require.NoError(t, err)
		require.Len(t, cols, 2)
		require.Equal(t, "a--b", cols[0].Name)
		require.Equal(t, "c/*d", cols[1].Name)
	})

	t.Run("errors", func(t *testing.T) {
		for _, in := range []string{
			"CREATE VIRTUAL TABLE t USING fts5(a, b)",
			"CREATE TABLE t(a int)",
			"",
			"CREATE VIRTUAL TABLE v USING vec0(e float[4",
			"CREATE VIRTUAL TABLE v USING vec0(e float[x])",
		} {
			require.NotPanics(t, func() {
				_, err := vecdecl.ParseVec0(in)
				require.Error(t, err, in)
			})
		}
	})
}

func TestVirtualTableModule(t *testing.T) {
	require.Equal(t, "fts5", vecdecl.VirtualTableModule("CREATE VIRTUAL TABLE t USING fts5(a, b)"))
	require.Equal(t, "vec0", vecdecl.VirtualTableModule("CREATE VIRTUAL TABLE t USING vec0(e float[2])"))
	require.Equal(t, "vec0", vecdecl.VirtualTableModule(`create virtual table "x y" using VEC0(e float[2])`))
	require.Equal(t, "vectorlite", vecdecl.VirtualTableModule(
		"CREATE VIRTUAL TABLE t USING vectorlite(e float32[2], hnsw(max_elements=1))",
	))
	require.Equal(t, "vec0", vecdecl.VirtualTableModule(
		"CREATE VIRTUAL TABLE t -- a comment\nUSING /* another */ vec0(e float[2])",
	))
	require.Equal(t, "", vecdecl.VirtualTableModule("CREATE TABLE t(a int)"))
	require.Equal(t, "", vecdecl.VirtualTableModule(""))
}

func TestParseLibSQLType(t *testing.T) {
	tests := []struct {
		in   string
		want vector.Type
		ok   bool
	}{
		{"F32_BLOB(4)", vector.Type{Dim: 4, Elem: vector.F32}, true},
		{"float32(4)", vector.Type{Dim: 4, Elem: vector.F32}, true},
		{"F64_BLOB(4)", vector.Type{Dim: 4, Elem: vector.F64}, true},
		{"FLOAT64(4)", vector.Type{Dim: 4, Elem: vector.F64}, true},
		{"F16_BLOB(4)", vector.Type{Dim: 4, Elem: vector.F16}, true},
		{"FLOAT16(4)", vector.Type{Dim: 4, Elem: vector.F16}, true},
		{"FB16_BLOB(4)", vector.Type{Dim: 4, Elem: vector.BF16}, true},
		{"FLOATB16(4)", vector.Type{Dim: 4, Elem: vector.BF16}, true},
		{"F8_BLOB(4)", vector.Type{Dim: 4, Elem: vector.F8}, true},
		{"FLOAT8(4)", vector.Type{Dim: 4, Elem: vector.F8}, true},
		{"F1BIT_BLOB(12)", vector.Type{Dim: 12, Elem: vector.Bit}, true},
		{"FLOAT1BIT(12)", vector.Type{Dim: 12, Elem: vector.Bit}, true},
		{"f32_blob( 8 )", vector.Type{Dim: 8, Elem: vector.F32}, true},
		{"BLOB", vector.Type{}, false},
		{"FLOAT", vector.Type{}, false},
		{"F32_BLOB", vector.Type{}, false},
		{"F32_BLOB(0)", vector.Type{}, false},
		{"F32_BLOB(-1)", vector.Type{}, false},
		{"F32_BLOB(x)", vector.Type{}, false},
		{"VARCHAR(4)", vector.Type{}, false},
		{"", vector.Type{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := vecdecl.ParseLibSQLType(tc.in)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseLibSQLVectorIndex(t *testing.T) {
	col, iv, ok := vecdecl.ParseLibSQLVectorIndex(
		"CREATE INDEX t_idx ON t(libsql_vector_idx(e32, 'metric=cosine', 'max_neighbors=8'))",
	)
	require.True(t, ok)
	require.Equal(t, "e32", col)
	require.Equal(t, &metadata.IndexVector{
		Method: "diskann",
		Metric: "cosine",
		Params: map[string]string{"max_neighbors": "8"},
	}, iv)

	col, iv, ok = vecdecl.ParseLibSQLVectorIndex(`CREATE INDEX "i" ON "t" (LIBSQL_VECTOR_IDX("Emb"))`)
	require.True(t, ok)
	require.Equal(t, "Emb", col)
	require.Equal(t, &metadata.IndexVector{Method: "diskann"}, iv)
	require.Nil(t, iv.Params)

	col, iv, ok = vecdecl.ParseLibSQLVectorIndex("CREATE INDEX i ON t(libsql_vector_idx(/* note */ emb, " +
		"-- metric\n'metric=l2'))")
	require.True(t, ok)
	require.Equal(t, "emb", col)
	require.Equal(t, &metadata.IndexVector{Method: "diskann", Metric: "l2"}, iv)

	_, _, ok = vecdecl.ParseLibSQLVectorIndex("CREATE INDEX i ON t(a) -- libsql_vector_idx(b)")
	require.False(t, ok)

	col, iv, ok = vecdecl.ParseLibSQLVectorIndex("CREATE INDEX i ON t(a, b)")
	require.False(t, ok)
	require.Empty(t, col)
	require.Nil(t, iv)

	_, _, ok = vecdecl.ParseLibSQLVectorIndex("")
	require.False(t, ok)
}

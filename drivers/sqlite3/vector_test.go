package sqlite3_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/core/vector"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
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

	const (
		rescoreDDL = `CREATE VIRTUAL TABLE fx_rescore USING vec0(embedding FLOAT[384] ` +
			`distance_metric=cosine INDEXED BY rescore(quantizer=bit, oversample=50))`
		otherDDL = `CREATE VIRTUAL TABLE fx_other USING vectorlite(my_embedding float32[3], ` +
			`hnsw(max_elements=100))`
	)

	rescore := srcMeta.Table("fx_rescore")
	require.Equal(t, sqlz.TableTypeVirtual, rescore.TableType)
	require.Equal(t,
		`vec0 column "embedding": unsupported index type: rescore(quantizer=bit, oversample=50)`,
		rescore.Error)
	require.Equal(t, rescoreDDL, rescore.Definition)
	require.Zero(t, rescore.RowCount)
	require.Len(t, rescore.Columns, 1)
	emb := colByName(t, rescore, "embedding")
	require.Equal(t, kind.Vector, emb.Kind)
	require.Equal(t, &vector.Type{Dim: 384, Elem: vector.F32, Metric: "cosine"}, emb.Vector)

	other := srcMeta.Table("fx_other")
	require.Equal(t, sqlz.TableTypeVirtual, other.TableType)
	require.Contains(t, other.Error, "no such module: vectorlite")
	require.Equal(t, otherDDL, other.Definition)
	require.Empty(t, other.Columns)

	for _, name := range []string{"fx_other", "fx_rescore"} {
		tblMeta, err := th.TableMetadata(src, name)
		require.NoError(t, err, name)
		require.Equal(t, srcMeta.Table(name).Error, tblMeta.Error, name)
		require.Equal(t, srcMeta.Table(name).Definition, tblMeta.Definition, name)
		require.Equal(t, srcMeta.Table(name).Columns, tblMeta.Columns, name)
		require.Equal(t, sqlz.TableTypeVirtual, tblMeta.TableType, name)
	}

	tblMeta, err := th.TableMetadata(src, "plain")
	require.NoError(t, err)
	require.Empty(t, tblMeta.Error)
	require.Empty(t, tblMeta.Definition)
	require.Empty(t, plain.Definition)
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

func TestLibSQL_SLQNotIndexedCase(t *testing.T) {
	th, _ := vecSrc(t, "@lib", "libsql.db")
	sink, err := th.QuerySLQ(`@lib.T | count`, nil)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 1)
	require.EqualValues(t, 2, sink.Recs[0][0])
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

// vecJSON asserts that val is a vector.Vector and returns its JSON form.
func vecJSON(t *testing.T, val any) string {
	t.Helper()
	v, ok := val.(vector.Vector)
	require.True(t, ok, "want vector.Vector but got %T", val)
	return string(v.AppendJSON(nil))
}

// colIndex returns the index of the named column in sink's record meta.
func colIndex(t *testing.T, sink *testh.RecordSink, name string) int {
	t.Helper()
	for i, fm := range sink.RecMeta {
		if fm.Name() == name {
			return i
		}
	}
	require.FailNow(t, "result column not found", name)
	return -1
}

func TestVector_DecodeVec0SLQ(t *testing.T) {
	th, _ := vecSrc(t, "@vec", "vec.db")

	t.Run("select_star", func(t *testing.T) {
		sink, err := th.QuerySLQ(`@vec.doc_vec | .[0]`, nil)
		require.NoError(t, err)
		require.Len(t, sink.Recs, 1)
		i := colIndex(t, sink, "embedding")
		require.Equal(t, kind.Vector, sink.RecMeta[i].Kind())
		require.Equal(t, "[0.9,0.1,0,0]", vecJSON(t, sink.Recs[0][i]))
		require.Equal(t, "animal", sink.Recs[0][colIndex(t, sink, "category")])
		require.Equal(t, "n1", sink.Recs[0][colIndex(t, sink, "note")])
	})

	t.Run("cols", func(t *testing.T) {
		sink, err := th.QuerySLQ(`@vec.doc_vec | .doc_id, .embedding`, nil)
		require.NoError(t, err)
		require.Len(t, sink.Recs, 3)
		require.EqualValues(t, 1, sink.Recs[0][0])
		require.Equal(t, "[0.9,0.1,0,0]", vecJSON(t, sink.Recs[0][1]))
		require.Equal(t, "[0.8,0.2,0,0]", vecJSON(t, sink.Recs[1][1]))
		require.Equal(t, "[0,0,0.9,0.1]", vecJSON(t, sink.Recs[2][1]))
	})

	t.Run("alias", func(t *testing.T) {
		sink, err := th.QuerySLQ(`@vec.doc_vec | .embedding:e`, nil)
		require.NoError(t, err)
		require.Len(t, sink.Recs, 3)
		require.Equal(t, "e", sink.RecMeta[0].Name())
		require.Equal(t, kind.Vector, sink.RecMeta[0].Kind())
		require.Equal(t, "[0.9,0.1,0,0]", vecJSON(t, sink.Recs[0][0]))
	})

	t.Run("int8_and_bit", func(t *testing.T) {
		sink, err := th.QuerySLQ(`@vec.bin_vec`, nil)
		require.NoError(t, err)
		require.Len(t, sink.Recs, 1)
		require.Equal(t, "[1,2,3,4]", vecJSON(t, sink.Recs[0][colIndex(t, sink, "e_i8")]))
		require.Equal(t, "[0,0,0,0,1,1,1,1]", vecJSON(t, sink.Recs[0][colIndex(t, sink, "e_bit")]))
	})

	t.Run("partition_key", func(t *testing.T) {
		sink, err := th.QuerySLQ(`@vec.part_vec | .user_id, .e`, nil)
		require.NoError(t, err)
		require.Len(t, sink.Recs, 2)
		require.EqualValues(t, 2, sink.Recs[1][0])
		require.Equal(t, "[3,4]", vecJSON(t, sink.Recs[1][1]))
	})

	t.Run("empty", func(t *testing.T) {
		sink, err := th.QuerySLQ(`@vec.empty_vec`, nil)
		require.NoError(t, err)
		require.Empty(t, sink.Recs)
		require.Equal(t, kind.Vector, sink.RecMeta[colIndex(t, sink, "e")].Kind())
	})

	t.Run("plain_table", func(t *testing.T) {
		sink, err := th.QuerySLQ(`@vec.docs | .title`, nil)
		require.NoError(t, err)
		require.Len(t, sink.Recs, 3)
		require.Equal(t, "cat", sink.Recs[0][0])
	})

	t.Run("join", func(t *testing.T) {
		// Joins don't get vec0 hints; the query must still succeed.
		sink, err := th.QuerySLQ(
			`@vec.doc_vec:d | join(@vec.docs:x, .d.doc_id == .x.id) | .x.title, .d.embedding`, nil,
		)
		require.NoError(t, err)
		require.Len(t, sink.Recs, 3)
	})
}

func TestVector_Vec0SQL(t *testing.T) {
	th, src := vecSrc(t, "@vec", "vec.db")

	// sq sql can't tell which result columns are vec0 vectors, so they stay
	// bytes.
	sink, err := th.QuerySQL(src, nil, `SELECT embedding FROM doc_vec`)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 3)
	require.NotEqual(t, kind.Vector, sink.RecMeta[0].Kind())
	require.IsType(t, []byte{}, sink.Recs[0][0])

	sink, err = th.QuerySQL(src, nil,
		`SELECT doc_id, distance FROM doc_vec WHERE embedding MATCH '[0.85,0.15,0,0]' AND k = 2`)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 2)
	require.EqualValues(t, 1, sink.Recs[0][0])
	require.EqualValues(t, 2, sink.Recs[1][0])
}

func TestVector_DecodeLibSQL(t *testing.T) {
	th, src := vecSrc(t, "@lib", "libsql.db")

	check := func(t *testing.T, sink *testh.RecordSink) {
		t.Helper()
		require.Len(t, sink.Recs, 2)
		e32, e64 := colIndex(t, sink, "e32"), colIndex(t, sink, "e64")
		e16, f := colIndex(t, sink, "e16"), colIndex(t, sink, "f")
		for _, i := range []int{e32, e64, e16, f} {
			require.Equal(t, kind.Vector, sink.RecMeta[i].Kind(), sink.RecMeta[i].Name())
		}

		require.Equal(t, "[1,2,-3.5,0.25]", vecJSON(t, sink.Recs[0][e32]))
		// Row 2 holds JSON text, as libSQL stores text unchanged.
		require.Equal(t, "[0.5,0.5,0.5,0.5]", vecJSON(t, sink.Recs[1][e32]))
		require.Equal(t, "[1,2,-3.5,0.25]", vecJSON(t, sink.Recs[0][e64]))
		require.Nil(t, sink.Recs[1][e64])
		require.Equal(t, "[1,2,-3.5,0.25]", vecJSON(t, sink.Recs[0][e16]))
		require.Equal(t, "[1,2,3]", vecJSON(t, sink.Recs[0][f]))
	}

	t.Run("slq", func(t *testing.T) {
		sink, err := th.QuerySLQ(`@lib.t`, nil)
		require.NoError(t, err)
		check(t, sink)
	})

	t.Run("sql", func(t *testing.T) {
		sink, err := th.QuerySQL(src, nil, `SELECT * FROM t`)
		require.NoError(t, err)
		check(t, sink)
	})
}

func TestVector_DecodeError(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)
	tbl := stringz.UniqTableName("vec_corrupt")
	th.ExecSQL(src, `CREATE TABLE `+tbl+`(id INTEGER PRIMARY KEY, e F32_BLOB(4))`)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })
	th.ExecSQL(src, `INSERT INTO `+tbl+`(id, e) VALUES (1, X'0000803F000000400000000000000000')`)
	// A 3-byte value can't be an F32_BLOB(4).
	th.ExecSQL(src, `INSERT INTO `+tbl+`(id, e) VALUES (2, X'010203')`)

	// In sq sql, the table isn't known.
	_, err := th.QuerySQL(src, nil, `SELECT id, e FROM `+tbl+` ORDER BY id`)
	require.Error(t, err)
	require.Contains(t, err.Error(), `column "e", row 2: invalid f32 vector`)
	require.NotContains(t, err.Error(), `table "`)

	// A single-table SLQ query knows the table.
	_, err = th.QuerySLQ(src.Handle+`.`+tbl+` | .e`, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), `table "`+tbl+`", column "e", row 2: invalid f32 vector`)

	// A join doesn't attribute result columns to a table.
	_, err = th.QuerySLQ(src.Handle+`.`+tbl+`:a | join(`+src.Handle+`.`+tbl+`:b, .a.id == .b.id) | .a.e`, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), `column "e", row 2: invalid f32 vector`)
	require.NotContains(t, err.Error(), `table "`)
}

// TestVector_LibSQLTypeNumericValue verifies that a numeric value in a
// plain SQLite column whose declared type matches a libSQL vector type, such
// as FLOAT8(10), comes back unchanged instead of failing the query. Such a
// value can't be a libSQL vector.
func TestVector_LibSQLTypeNumericValue(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)
	tbl := stringz.UniqTableName("float8")
	th.ExecSQL(src, `CREATE TABLE `+tbl+`(id INTEGER PRIMARY KEY, r FLOAT8(10), i FLOAT32(2))`)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })
	th.ExecSQL(src, `INSERT INTO `+tbl+`(id, r, i) VALUES (1, 1.5, 7), (2, NULL, NULL)`)

	check := func(t *testing.T, sink *testh.RecordSink) {
		t.Helper()
		require.Len(t, sink.Recs, 2)
		require.Equal(t, 1.5, sink.Recs[0][colIndex(t, sink, "r")])
		require.EqualValues(t, 7, sink.Recs[0][colIndex(t, sink, "i")])
		require.Nil(t, sink.Recs[1][colIndex(t, sink, "r")])
	}

	t.Run("sql", func(t *testing.T) {
		sink, err := th.QuerySQL(src, nil, `SELECT id, r, i FROM `+tbl+` ORDER BY id`)
		require.NoError(t, err)
		check(t, sink)
	})

	t.Run("slq", func(t *testing.T) {
		sink, err := th.QuerySLQ(src.Handle+`.`+tbl+` | .id, .r, .i`, nil)
		require.NoError(t, err)
		check(t, sink)
	})
}

// TestVector_UnopenableVec0Query verifies that a query against a vec0 table
// that SQLite can't open reports the query's error, not an error from
// collecting the table's vector hints.
func TestVector_UnopenableVec0Query(t *testing.T) {
	th, _ := vecSrc(t, "@vtab_unopenable", "vtab_unopenable.db")
	_, err := th.QuerySLQ(`@vtab_unopenable.fx_rescore`, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SQL query against @vtab_unopenable failed")
	require.Contains(t, err.Error(), "vec0 constructor error")
}

// TestVector_Vec0UnparsedDDL verifies that a vec0 table whose DDL sq can't
// parse (but sqlite-vec accepts) is still queryable, with no vector hints.
func TestVector_Vec0UnparsedDDL(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)
	tbl := stringz.UniqTableName("vec_unparsed")
	// sqlite-vec matches the element type by prefix, so it accepts
	// "float32[2]" as a float vector, but sq's DDL parser rejects the
	// unknown element type.
	th.ExecSQL(src, `CREATE VIRTUAL TABLE `+tbl+` USING vec0(doc_id INTEGER PRIMARY KEY,
embedding float32[2])`)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })
	th.ExecSQL(src, `INSERT INTO `+tbl+`(doc_id, embedding) VALUES (1, '[1,2]')`)

	sink, err := th.QuerySLQ(src.Handle+`.`+tbl+` | .doc_id`, nil)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 1)
	require.EqualValues(t, 1, sink.Recs[0][0])

	sink, err = th.QuerySLQ(src.Handle+`.`+tbl, nil)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 1)
	require.IsType(t, []byte{}, sink.Recs[0][colIndex(t, sink, "embedding")])
}

// TestVector_Vec0CommentedDDL verifies that SQL comments in a vec0
// declaration, including an apostrophe or a comma inside a comment, don't
// stop the vector columns from decoding.
func TestVector_Vec0CommentedDDL(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)
	tbl := stringz.UniqTableName("vec_commented")
	th.ExecSQL(src, `CREATE VIRTUAL TABLE `+tbl+` USING vec0(
doc_id INTEGER PRIMARY KEY, -- the doc's id, unique
embedding float[2] /* dims, metric */
)`)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })
	th.ExecSQL(src, `INSERT INTO `+tbl+`(doc_id, embedding) VALUES (1, '[1,2]')`)

	tblMeta, err := th.TableMetadata(src, tbl)
	require.NoError(t, err)
	require.Len(t, tblMeta.Columns, 2)
	require.True(t, colByName(t, tblMeta, "doc_id").PrimaryKey)
	require.Equal(t, &vector.Type{Dim: 2, Elem: vector.F32}, colByName(t, tblMeta, "embedding").Vector)

	sink, err := th.QuerySLQ(src.Handle+`.`+tbl, nil)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 1)
	require.Equal(t, "[1,2]", vecJSON(t, sink.Recs[0][colIndex(t, sink, "embedding")]))
}

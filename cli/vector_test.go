package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
)

// vectorFixtureSrc returns a SQLite source for a vector fixture database in
// drivers/sqlite3/testdata.
func vectorFixtureSrc(handle, file string) source.Source {
	return source.Source{
		Handle:   handle,
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + proj.Abs("drivers/sqlite3/testdata/"+file),
	}
}

// execVector runs sq with args against the @vec (sqlite-vec) and @lib
// (libSQL) fixture sources, and returns the TestRun.
func execVector(t *testing.T, args ...string) *testrun.TestRun {
	t.Helper()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil).Add(
		vectorFixtureSrc("@vec", "vec.db"),
		vectorFixtureSrc("@lib", "libsql.db"),
	)
	require.NoError(t, tr.Exec(args...))
	return tr
}

func TestCmdSLQ_Vector(t *testing.T) {
	t.Run("vec0_select_star_json", func(t *testing.T) {
		tr := execVector(t, "-j", "@vec.doc_vec | .[0]")
		got := tr.BindSliceMap()
		require.Len(t, got, 1)
		require.Equal(t, []any{0.9, 0.1, 0.0, 0.0}, got[0]["embedding"])
		require.EqualValues(t, 1, got[0]["doc_id"])
	})

	t.Run("vec0_cols_json", func(t *testing.T) {
		tr := execVector(t, "-j", "@vec.doc_vec | .doc_id, .embedding")
		got := tr.BindSliceMap()
		require.Len(t, got, 3)
		require.Equal(t, []any{0.8, 0.2, 0.0, 0.0}, got[1]["embedding"])
	})

	t.Run("vec0_alias_json", func(t *testing.T) {
		tr := execVector(t, "-j", "@vec.doc_vec | .embedding:e")
		got := tr.BindSliceMap()
		require.Len(t, got, 3)
		require.Equal(t, []any{0.9, 0.1, 0.0, 0.0}, got[0]["e"])
	})

	t.Run("vec0_text", func(t *testing.T) {
		tr := execVector(t, "--text", "@vec.doc_vec")
		require.Contains(t, tr.OutString(), "[4 × f32]")
	})

	t.Run("vec0_int8_bit_json", func(t *testing.T) {
		tr := execVector(t, "-j", "@vec.bin_vec")
		got := tr.BindSliceMap()
		require.Len(t, got, 1)
		require.Equal(t, []any{1.0, 2.0, 3.0, 4.0}, got[0]["e_i8"])
		require.Equal(t, []any{0.0, 0.0, 0.0, 0.0, 1.0, 1.0, 1.0, 1.0}, got[0]["e_bit"])
	})

	t.Run("libsql_json", func(t *testing.T) {
		tr := execVector(t, "-j", "@lib.t")
		checkLibSQLJSON(t, tr.BindSliceMap())
	})
}

// TestCmdInspect_VectorText verifies that verbose text inspect shows a vec0
// vector column's metric and the roles of its other columns.
func TestCmdInspect_VectorText(t *testing.T) {
	tr := execVector(t, "inspect", "--text", "-v", "@vec.doc_vec")
	out := tr.OutString()
	require.Regexp(t, `\sembedding\s+float\[4\] cosine\s`, out)
	require.Regexp(t, `\scategory\s+TEXT \(metadata\)\s`, out)
	require.Regexp(t, `\snote\s+TEXT \(auxiliary\)\s`, out)

	tr = execVector(t, "inspect", "--text", "-v", "@vec.part_vec")
	require.Regexp(t, `\suser_id\s+\S+ \(partition_key\)\s`, tr.OutString())
}

func TestCmdSQL_Vector(t *testing.T) {
	t.Run("vec0_stays_bytes", func(t *testing.T) {
		tr := execVector(t, "sql", "-j", "--src", "@vec", "SELECT embedding FROM doc_vec")
		got := tr.BindSliceMap()
		require.Len(t, got, 3)
		require.Equal(t, "ZmZmP83MzD0AAAAAAAAAAA==", got[0]["embedding"])
	})

	t.Run("vec0_knn", func(t *testing.T) {
		tr := execVector(t, "sql", "-j", "--src", "@vec",
			"SELECT doc_id, distance FROM doc_vec WHERE embedding MATCH '[0.85,0.15,0,0]' AND k = 2")
		got := tr.BindSliceMap()
		require.Len(t, got, 2)
		require.EqualValues(t, 1, got[0]["doc_id"])
		require.EqualValues(t, 2, got[1]["doc_id"])
	})

	t.Run("libsql_json", func(t *testing.T) {
		tr := execVector(t, "sql", "-j", "--src", "@lib", "SELECT * FROM t")
		checkLibSQLJSON(t, tr.BindSliceMap())
	})
}

// checkLibSQLJSON checks the JSON output of "SELECT * FROM t" against the
// libsql.db fixture.
func checkLibSQLJSON(t *testing.T, got []map[string]any) {
	t.Helper()
	require.Len(t, got, 2)
	require.Equal(t, []any{1.0, 2.0, -3.5, 0.25}, got[0]["e32"])
	// Row 2's e32 holds JSON text, which libSQL stores unchanged.
	require.Equal(t, []any{0.5, 0.5, 0.5, 0.5}, got[1]["e32"])
	require.Nil(t, got[1]["e64"])
	require.Equal(t, []any{1.0, 2.0, -3.5, 0.25}, got[0]["e16"])
	require.Equal(t, []any{1.0, 2.0, 3.0}, got[0]["f"])
}

// TestCmdSLQ_VectorCopy verifies that vector columns copy as their original
// bytes, for a cross-source join and for --insert.
func TestCmdSLQ_VectorCopy(t *testing.T) {
	t.Run("join_csv", func(t *testing.T) {
		th := testh.New(t)
		csvSrc := th.Source(sakila.CSVActor)
		tr := testrun.New(th.Context, t, nil).Add(vectorFixtureSrc("@lib", "libsql.db"), *csvSrc)
		err := tr.Exec("-j",
			"@lib.t:t | join("+csvSrc.Handle+".data:a, .t.id == .a.actor_id) | .t.id, .t.e32, .a.first_name")
		require.NoError(t, err)
		got := tr.BindSliceMap()
		require.Len(t, got, 2)
		require.Equal(t, "PENELOPE", got[0]["first_name"])
		// The join copies t into a scratch database, where e32 is a BLOB.
		require.Equal(t, "AACAPwAAAEAAAGDAAACAPg==", got[0]["e32"])
	})

	t.Run("insert", func(t *testing.T) {
		th := testh.New(t)
		libSrc := vectorFixtureSrc("@lib", "libsql.db")
		destSrc := th.Source(sakila.SL3)
		destTbl := stringz.UniqTableName("vec_copy")
		t.Cleanup(func() { th.DropTable(destSrc, tablefq.From(destTbl)) })

		tr := testrun.New(th.Context, t, nil).Add(libSrc, *destSrc)
		err := tr.Exec("slq", "--insert="+destSrc.Handle+"."+destTbl, "@lib.t | .id, .e32")
		require.NoError(t, err)

		th.Add(&libSrc)
		want, err := th.QuerySQL(&libSrc, nil, "SELECT hex(e32) FROM t ORDER BY id")
		require.NoError(t, err)
		got, err := th.QuerySQL(destSrc, nil,
			"SELECT hex(e32), typeof(e32) FROM "+destTbl+" ORDER BY id")
		require.NoError(t, err)
		require.Len(t, got.Recs, 2)
		for i := range got.Recs {
			require.Equal(t, want.Recs[i][0], got.Recs[i][0], "row %d", i+1)
		}
		require.Equal(t, "blob", got.Recs[0][1])
	})
}

package rqlite_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/rqlite"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestSmoke exercises Open/Ping plus a basic SELECT against the
// sakiladb/rqlite container. The test is skipped under `go test -short`
// or when SQ_TEST_SRC__SAKILA_RQ is unset (the standard pattern for
// network-backed sakila sources).
func TestSmoke(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	require.Equal(t, drivertype.Rqlite, src.Type)

	sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+sakila.TblActor)
	require.NoError(t, err)
	require.Equal(t, sakila.TblActorCount, len(sink.Recs))
}

// TestSourceMetadata verifies that getSourceMetadata returns the
// expected shape: rqlite driver, "main" schema, and the right
// table/view counts (16 tables, 5 views in the bundled Sakila).
func TestSourceMetadata(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)

	md, err := grip.SourceMetadata(th.Context, false)
	require.NoError(t, err)
	require.Equal(t, drivertype.Rqlite, md.Driver)
	require.Equal(t, "main", md.Schema)
	require.Equal(t, "default", md.Catalog)
	require.NotEmpty(t, md.DBVersion, "expected SQLite version from rqlite")
	// The strict baseline is 16 tables; parallel write-path tests
	// create extra transient tables that may still be live when the
	// metadata query runs. Assert the lower bound rather than equality.
	require.GreaterOrEqual(t, md.TableCount, int64(16))
	require.Equal(t, int64(5), md.ViewCount)
	// rqlite's HTTP API doesn't expose a database file size, so the
	// driver leaves Source.Size as nil (gh744). Asserting nil prevents a
	// regression to the int64 zero value, which would render as "0.0B".
	require.Nil(t, md.Size, "rqlite source size should not be reported")
}

// TestTableMetadata_Actor verifies the per-table metadata path:
// column kinds, primary-key flag, and row count for the actor table.
func TestTableMetadata_Actor(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)

	tbl, err := grip.TableMetadata(th.Context, sakila.TblActor)
	require.NoError(t, err)
	require.Equal(t, sakila.TblActor, tbl.Name)
	require.Equal(t, int64(sakila.TblActorCount), tbl.RowCount)

	gotKinds := make([]kind.Kind, len(tbl.Columns))
	for i, col := range tbl.Columns {
		gotKinds[i] = col.Kind
	}
	// actor: actor_id (decimal due to NUMERIC affinity), first_name,
	// last_name (text), last_update (datetime). sakila.TblActorColKinds
	// returns kind.Int for actor_id; the SQLite-on-rqlite shape uses
	// NUMERIC → decimal, so we assert the column kinds explicitly here
	// rather than reusing the shared helper.
	require.Equal(t, []kind.Kind{kind.Decimal, kind.Text, kind.Text, kind.Datetime}, gotKinds)
	require.True(t, tbl.Columns[0].PrimaryKey, "actor_id should be primary key")
}

func TestCreateTable(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "actor_w_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(
		tblName,
		[]string{"id", "name", "ts"},
		[]kind.Kind{kind.Int, kind.Text, kind.Datetime},
	)
	tblDef.PKColName = "id"

	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	got, err := grip.TableMetadata(th.Context, tblName)
	require.NoError(t, err)
	require.Equal(t, tblName, got.Name)
	require.Len(t, got.Columns, 3)
	require.Equal(t, kind.Int, got.Columns[0].Kind)
	require.Equal(t, kind.Text, got.Columns[1].Kind)
	require.Equal(t, kind.Datetime, got.Columns[2].Kind)
}

func TestAlterTableRename(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	oldName := "rename_old_" + uniq
	newName := "rename_new_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: oldName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: newName}, true)
	})

	tblDef := schema.NewTable(oldName, []string{"id"}, []kind.Kind{kind.Int})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	require.NoError(t, drvr.AlterTableRename(th.Context, db, oldName, newName))

	exists, err := drvr.TableExists(th.Context, db, newName)
	require.NoError(t, err)
	require.True(t, exists)
	exists, err = drvr.TableExists(th.Context, db, oldName)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestAlterTableAddColumn(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "addcol_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName, []string{"id"}, []kind.Kind{kind.Int})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	require.NoError(t, drvr.AlterTableAddColumn(th.Context, db, tblName, "name", kind.Text))
	require.NoError(t, drvr.AlterTableAddColumn(th.Context, db, tblName, "ts", kind.Datetime))

	md, err := grip.TableMetadata(th.Context, tblName)
	require.NoError(t, err)
	require.Len(t, md.Columns, 3)
	require.Equal(t, "name", md.Columns[1].Name)
	require.Equal(t, kind.Text, md.Columns[1].Kind)
	require.Equal(t, "ts", md.Columns[2].Name)
	require.Equal(t, kind.Datetime, md.Columns[2].Kind)
}

func TestAlterTableRenameColumn(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "renamecol_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName, []string{"id", "first_name"}, []kind.Kind{kind.Int, kind.Text})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	require.NoError(t, drvr.AlterTableRenameColumn(th.Context, db, tblName, "first_name", "given_name"))

	md, err := grip.TableMetadata(th.Context, tblName)
	require.NoError(t, err)
	colNames := make([]string, len(md.Columns))
	for i, c := range md.Columns {
		colNames[i] = c.Name
	}
	require.Equal(t, []string{"id", "given_name"}, colNames)
}

func TestTruncate_NoReset(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "trunc_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName, []string{"id", "name"}, []kind.Kind{kind.Int, kind.Text})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	for i := 1; i <= 3; i++ {
		_, err = db.ExecContext(th.Context,
			fmt.Sprintf(`INSERT INTO %q (id, name) VALUES (?, ?)`, tblName), i, "x")
		require.NoError(t, err)
	}

	affected, err := drvr.Truncate(th.Context, src, tblName, false)
	require.NoError(t, err)
	require.Equal(t, int64(3), affected)

	var count int64
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT COUNT(*) FROM %q`, tblName)).Scan(&count))
	require.Equal(t, int64(0), count)
}

func TestTruncate_Reset(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "trunc_reset_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName, []string{"id", "name"}, []kind.Kind{kind.Int, kind.Text})
	tblDef.PKColName = "id"
	tblDef.AutoIncrement = true
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	// Insert 3 rows so sqlite_sequence has data for this table.
	for i := 0; i < 3; i++ {
		_, err = db.ExecContext(th.Context,
			fmt.Sprintf(`INSERT INTO %q (name) VALUES (?)`, tblName), "x")
		require.NoError(t, err)
	}

	affected, err := drvr.Truncate(th.Context, src, tblName, true)
	require.NoError(t, err)
	require.Equal(t, int64(3), affected)

	// Insert again; the new id should be 1, not 4.
	res, err := db.ExecContext(th.Context,
		fmt.Sprintf(`INSERT INTO %q (name) VALUES (?)`, tblName), "y")
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	require.Equal(t, int64(1), id, "AUTOINCREMENT counter should have been reset")
}

// TestAlterTruncate_EmbeddedQuoteIdentifier reproduces gh821: the
// AlterTableRename, AlterTableRenameColumn, AlterTableAddColumn, and Truncate
// paths used %q for SQL identifier quoting, which emits Go backslash escaping
// that SQLite rejects for names containing a double quote (e.g. a we"ird table,
// creatable from a CSV header). Each path must use SQL double-quote escaping.
func TestAlterTruncate_EmbeddedQuoteIdentifier(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	tblName := `we"ird_` + uniq
	newName := `we"ird2_` + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: newName}, true)
	})

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf("CREATE TABLE %s (id INTEGER)", stringz.DoubleQuote(tblName)))
	require.NoError(t, err)

	// AlterTableAddColumn: add a column whose name also contains a quote.
	const colName = `na"me`
	require.NoError(t, drvr.AlterTableAddColumn(th.Context, db, tblName, colName, kind.Text))

	// AlterTableRenameColumn: rename the quoted column to another quoted name.
	const renamedCol = `re"named`
	require.NoError(t, drvr.AlterTableRenameColumn(th.Context, db, tblName, colName, renamedCol))

	md, err := grip.TableMetadata(th.Context, tblName)
	require.NoError(t, err)
	require.Len(t, md.Columns, 2)
	require.Equal(t, renamedCol, md.Columns[1].Name)

	// Truncate: insert a row, then delete all rows via the truncate path.
	_, err = db.ExecContext(th.Context,
		fmt.Sprintf("INSERT INTO %s (id) VALUES (1)", stringz.DoubleQuote(tblName)))
	require.NoError(t, err)
	affected, err := drvr.Truncate(th.Context, src, tblName, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	// AlterTableRename: rename the quoted table to another quoted name.
	require.NoError(t, drvr.AlterTableRename(th.Context, db, tblName, newName))
	exists, err := drvr.TableExists(th.Context, db, newName)
	require.NoError(t, err)
	require.True(t, exists)
	exists, err = drvr.TableExists(th.Context, db, tblName)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestCopyTable_StructureOnly(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	dstName := "actor_copy_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
	})

	affected, err := drvr.CopyTable(th.Context, db,
		tablefq.T{Table: sakila.TblActor}, tablefq.T{Table: dstName}, false)
	require.NoError(t, err)
	require.Equal(t, int64(0), affected)

	md, err := grip.TableMetadata(th.Context, dstName)
	require.NoError(t, err)
	require.Equal(t, dstName, md.Name)
	require.Equal(t, int64(0), md.RowCount)

	src2 := th.Source(sakila.Rq)
	srcMd, err := th.Open(src2).TableMetadata(th.Context, sakila.TblActor)
	require.NoError(t, err)
	require.Len(t, md.Columns, len(srcMd.Columns))
}

func TestCopyTable_WithData(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	dstName := "actor_data_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
	})

	affected, err := drvr.CopyTable(th.Context, db,
		tablefq.T{Table: sakila.TblActor}, tablefq.T{Table: dstName}, true)
	require.NoError(t, err)
	require.Equal(t, int64(sakila.TblActorCount), affected)

	md, err := grip.TableMetadata(th.Context, dstName)
	require.NoError(t, err)
	require.Equal(t, int64(sakila.TblActorCount), md.RowCount)
}

func TestAlterTableColumnKinds(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "kinds_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName, []string{"a", "b"}, []kind.Kind{kind.Int, kind.Text})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`INSERT INTO %q (a, b) VALUES (?, ?)`, tblName), 42, "hello")
	require.NoError(t, err)

	// Swap kinds: a INTEGER -> TEXT, b TEXT -> INTEGER.
	require.NoError(t, drvr.AlterTableColumnKinds(th.Context, db, tblName,
		[]string{"a", "b"}, []kind.Kind{kind.Text, kind.Int}))

	md, err := grip.TableMetadata(th.Context, tblName)
	require.NoError(t, err)
	require.Equal(t, tblName, md.Name)
	require.Equal(t, kind.Text, md.Columns[0].Kind)
	require.Equal(t, kind.Int, md.Columns[1].Kind)

	// Row data should round-trip; sqlite is permissive about typing.
	var gotA, gotB string
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT a, b FROM %q`, tblName)).Scan(&gotA, &gotB))
	require.Equal(t, "42", gotA)
	require.Equal(t, "hello", gotB)
}

// TestAlterTableColumnKinds_PreservesAutoincrementSeq reproduces gh757: the
// table-rebuild dance in AlterTableColumnKinds dropped the original table,
// which removed its sqlite_sequence row, so AUTOINCREMENT restarted from
// MAX(rowid)+1 instead of seq+1. With rows previously deleted from the high
// end, the next insert silently reused their ids. Mirrors the sqlite3
// driver test.
func TestAlterTableColumnKinds_PreservesAutoincrementSeq(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "seqtbl_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (id INTEGER PRIMARY KEY AUTOINCREMENT, val INTEGER NOT NULL)`, tblName,
	))
	require.NoError(t, err)

	for i := 1; i <= 10; i++ {
		_, err = db.ExecContext(th.Context,
			fmt.Sprintf(`INSERT INTO %q (val) VALUES (?)`, tblName), i)
		require.NoError(t, err)
	}
	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`DELETE FROM %q WHERE id > 5`, tblName))
	require.NoError(t, err)

	var seq, maxID int64
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT seq FROM sqlite_sequence WHERE name=?`, tblName).Scan(&seq))
	require.Equal(t, int64(10), seq)
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT MAX(id) FROM %q`, tblName)).Scan(&maxID))
	require.Equal(t, int64(5), maxID)

	require.NoError(t, drvr.AlterTableColumnKinds(th.Context, db, tblName,
		[]string{"val"}, []kind.Kind{kind.Text}))

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`INSERT INTO %q (val) VALUES ('post-alter')`, tblName))
	require.NoError(t, err)

	var newID int64
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT id FROM %q WHERE val = 'post-alter'`, tblName)).Scan(&newID))
	require.Equal(t, int64(11), newID,
		"AUTOINCREMENT must continue from the preserved sequence (seq+1), not MAX(rowid)+1")
}

// TestAlterTableColumnKinds_QuotedIdentifier reproduces gh752: a column
// declared with any of SQLite's four legal identifier-quoting styles
// (double-quote, single-quote, backtick, square brackets) used to fail the
// lookup in AlterTableColumnKinds when the parser's strip-quotes step only
// handled double-quotes. The shared sqlparser now strips all four styles.
func TestAlterTableColumnKinds_QuotedIdentifier(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	testCases := []struct {
		name    string
		colDecl string
	}{
		{name: "double_quote", colDecl: `"age" INTEGER NOT NULL`},
		{name: "single_quote", colDecl: `'age' INTEGER NOT NULL`},
		{name: "backtick", colDecl: "`age` INTEGER NOT NULL"},
		{name: "square_brackets", colDecl: `[age] INTEGER NOT NULL`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(sakila.Rq)
			grip := th.Open(src)
			drvr := grip.SQLDriver()
			db, err := grip.DB(th.Context)
			require.NoError(t, err)

			tblName := "qident_" + stringz.Uniq8()
			t.Cleanup(func() {
				_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
			})

			_, err = db.ExecContext(th.Context,
				fmt.Sprintf(`CREATE TABLE %q (%s)`, tblName, tc.colDecl))
			require.NoError(t, err)

			err = drvr.AlterTableColumnKinds(th.Context, db, tblName,
				[]string{"age"}, []kind.Kind{kind.Text})
			require.NoError(t, err)

			md, err := grip.TableMetadata(th.Context, tblName)
			require.NoError(t, err)
			require.Equal(t, kind.Text, md.Column("age").Kind)
		})
	}
}

// TestAlterTableColumnKinds_ColumnNamePrefixesType reproduces gh750: a
// column whose name shares its declared-case prefix with the type token
// (e.g. `text_data text`) caused the old
// `strings.Replace(colDef.Raw, colDef.RawType, wantType, 1)` to clobber
// the name's prefix instead of the actual type. The offset-based rewrite
// targets the parsed RawType position directly, so the name is
// preserved. Mirrors the sqlite3 driver test.
func TestAlterTableColumnKinds_ColumnNamePrefixesType(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	testCases := []struct {
		name    string
		colDecl string
		colName string
	}{
		{
			name:    "lowercase_prefix",
			colDecl: `text_data text NOT NULL`,
			colName: "text_data",
		},
		{
			name:    "uppercase_prefix",
			colDecl: `TEXT_DATA TEXT NOT NULL`,
			colName: "TEXT_DATA",
		},
		{
			name:    "bracket_quoted_prefix",
			colDecl: `[TEXT_DATA] TEXT NOT NULL`,
			colName: "TEXT_DATA",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(sakila.Rq)
			grip := th.Open(src)
			drvr := grip.SQLDriver()
			db, err := grip.DB(th.Context)
			require.NoError(t, err)

			tblName := "collide_" + stringz.Uniq8()
			t.Cleanup(func() {
				_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
			})

			_, err = db.ExecContext(th.Context,
				fmt.Sprintf(`CREATE TABLE %q (%s)`, tblName, tc.colDecl))
			require.NoError(t, err)

			err = drvr.AlterTableColumnKinds(th.Context, db, tblName,
				[]string{tc.colName}, []kind.Kind{kind.Int})
			require.NoError(t, err)

			md, err := grip.TableMetadata(th.Context, tblName)
			require.NoError(t, err)
			require.Len(t, md.Columns, 1, "the column should still exist after alter")
			require.Equal(t, tc.colName, md.Columns[0].Name,
				"the column name must survive the alter; substring-replace would have clobbered it")
			require.Equal(t, kind.Int, md.Column(tc.colName).Kind)
		})
	}
}

// TestCopyTable_TableIdentInDefaultLiteral verifies that CopyTable
// rewrites only the table identifier and leaves a substring-matching
// occurrence inside a column DEFAULT expression untouched. Mirrors the
// sqlite3 driver test.
func TestCopyTable_TableIdentInDefaultLiteral(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	srcName := "actor_" + stringz.Uniq8()
	dstName := "actor_bak_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: srcName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
	})

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`CREATE TABLE %q (id INTEGER, tag TEXT DEFAULT '%s_tag')`,
			srcName, srcName))
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: srcName}, tablefq.T{Table: dstName}, false)
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`INSERT INTO %q (id) VALUES (1)`, dstName))
	require.NoError(t, err)

	var tag string
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT tag FROM %q WHERE id=1`, dstName)).Scan(&tag))
	require.Equal(t, srcName+"_tag", tag,
		"the DEFAULT literal must not be rewritten by the table-identifier substitution")
}

func TestPrepareInsertStmt(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "prepins_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName,
		[]string{"id", "name"}, []kind.Kind{kind.Int, kind.Text})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	// PrepareInsertStmt requires a single-conn db.
	conn, err := db.Conn(th.Context)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	execer, err := drvr.PrepareInsertStmt(th.Context, conn, tblName,
		[]string{"id", "name"}, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = execer.Close() })

	affected, err := execer.Exec(th.Context, int64(1), "a")
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	var count int64
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT COUNT(*) FROM %q`, tblName)).Scan(&count))
	require.Equal(t, int64(1), count)
}

func TestBatchInsert(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "batchins_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	// 4 columns -> batchSize = MaxBatchValues(500) / 4 = 125.
	// 1500 records => 12 batches, exercising the goroutine flush path.
	tblDef := schema.NewTable(
		tblName,
		[]string{"a", "b", "c", "d"},
		[]kind.Kind{kind.Int, kind.Text, kind.Text, kind.Datetime},
	)
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	conn, err := db.Conn(th.Context)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	bi, err := drvr.NewBatchInsert(th.Context, "batch ingest", conn, src, tblName,
		[]string{"a", "b", "c", "d"})
	require.NoError(t, err)

	const total = 1500
	go func() {
		defer close(bi.RecordCh)
		for i := 0; i < total; i++ {
			rec := []any{int64(i), "b", "c", "2026-01-01T00:00:00"}
			if mErr := bi.Munge(rec); mErr != nil {
				t.Errorf("munge failed: %v", mErr)
				return
			}
			bi.RecordCh <- rec
		}
	}()

	for biErr := range bi.ErrCh {
		require.NoError(t, biErr)
	}
	require.Equal(t, int64(total), bi.Written())

	var count int64
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT COUNT(*) FROM %q`, tblName)).Scan(&count))
	require.Equal(t, int64(total), count)
}

func TestPrepareUpdateStmt(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "prepupd_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName,
		[]string{"id", "name"}, []kind.Kind{kind.Int, kind.Text})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`INSERT INTO %q (id, name) VALUES (?, ?)`, tblName), 1, "before")
	require.NoError(t, err)

	conn, err := db.Conn(th.Context)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	execer, err := drvr.PrepareUpdateStmt(th.Context, conn, tblName,
		[]string{"name"}, "id = ?")
	require.NoError(t, err)
	t.Cleanup(func() { _ = execer.Close() })

	affected, err := execer.Exec(th.Context, "after", int64(1))
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	var got string
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT name FROM %q WHERE id=1`, tblName)).Scan(&got))
	require.Equal(t, "after", got)
}

func TestAlterTableColumnKinds_MismatchedLength(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	err = drvr.AlterTableColumnKinds(th.Context, db, sakila.TblActor,
		[]string{"a", "b"}, []kind.Kind{kind.Int})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mismatched count")
}

// TestConsistencyLevels_Smoke verifies that gorqlite accepts each of
// the four ?level=... URL parameters without breaking the connection
// and that a basic SELECT still works. This is a smoke test, not a
// real consistency verification: a single-node sakiladb image cannot
// exercise the cluster-level semantics that "linearizable" / "strong"
// imply. See gh738 for a future cluster-aware test.
func TestConsistencyLevels_Smoke(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	base := th.Source(sakila.Rq)

	levels := []string{"none", "weak", "linearizable", "strong"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse(base.Location)
			require.NoError(t, err)
			q := u.Query()
			q.Set("level", level)
			u.RawQuery = q.Encode()

			src := &source.Source{
				Handle:   base.Handle + "_" + level,
				Type:     base.Type,
				Location: u.String(),
				Options:  base.Options,
			}

			provider := &rqlite.Provider{Log: lg.FromContext(th.Context)}
			drvr, err := provider.DriverFor(drivertype.Rqlite)
			require.NoError(t, err)

			grip, err := drvr.Open(th.Context, src, driver.ModeReadWrite)
			require.NoError(t, err)
			t.Cleanup(func() { _ = grip.Close() })

			db, err := grip.DB(th.Context)
			require.NoError(t, err)

			var count int64
			require.NoError(t, db.QueryRowContext(th.Context,
				`SELECT COUNT(*) FROM `+sakila.TblActor).Scan(&count))
			require.Equal(t, int64(sakila.TblActorCount), count)
		})
	}
}

func TestAlterTableColumnKinds_UnknownColumn(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "unkcol_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName, []string{"a"}, []kind.Kind{kind.Int})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	err = drvr.AlterTableColumnKinds(th.Context, db, tblName,
		[]string{"nonexistent"}, []kind.Kind{kind.Text})
	require.Error(t, err)
	require.Contains(t, err.Error(), "column")
}

// TestOpen_DefaultsPort confirms that the driver injects port 4001
// when the source location omits a port. Without the injection,
// gorqlite/stdlib would try Go's net/http default (80) and fail
// with connection refused.
func TestOpen_DefaultsPort(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	base := th.Source(sakila.Rq)

	u, err := url.Parse(base.Location)
	require.NoError(t, err)
	// Strip the port: rebuild Host without the ":4001" suffix.
	u.Host = u.Hostname()
	noPortLoc := u.String()
	require.NotContains(t, noPortLoc, ":4001",
		"port should be stripped for this test")

	src := &source.Source{
		Handle:   base.Handle + "_noport",
		Type:     base.Type,
		Location: noPortLoc,
		Options:  base.Options,
	}

	provider := &rqlite.Provider{Log: lg.FromContext(th.Context)}
	drvr, err := provider.DriverFor(drivertype.Rqlite)
	require.NoError(t, err)
	grip, err := drvr.Open(th.Context, src, driver.ModeReadWrite)
	require.NoError(t, err, "should auto-default port 4001")
	t.Cleanup(func() { _ = grip.Close() })

	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.QueryRowContext(th.Context,
		"SELECT COUNT(*) FROM "+sakila.TblActor).Scan(&count))
	require.Equal(t, int64(sakila.TblActorCount), count)
}

// TestWriteAtomic_PerStatementError exercises the per-statement error
// wrap inside writeAtomic. We trigger it by pre-creating the
// destination table so the subsequent CopyTable(copyData=true) batch's
// stmt 1 (CREATE TABLE) fails inside the /db/execute call.
func TestWriteAtomic_PerStatementError(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	dstName := "writeatomic_err_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
	})

	// Pre-create the destination so the CopyTable's CREATE fails.
	preDef := schema.NewTable(dstName, []string{"x"}, []kind.Kind{kind.Int})
	require.NoError(t, drvr.CreateTable(th.Context, db, preDef))

	// CopyTable(copyData=true) issues [CREATE dstName, INSERT INTO
	// dstName SELECT * FROM actor, companion DDL...] as one atomic
	// batch (the companion count depends on actor's indexes and
	// triggers, so the batch size isn't asserted). The CREATE fails
	// because dstName already exists, and writeAtomic should surface
	// "statement 1/N failed" with the underlying cause.
	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: sakila.TblActor},
		tablefq.T{Table: dstName},
		true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "statement 1/",
		"writeAtomic should identify which statement in the batch failed")
	require.Contains(t, err.Error(), "already exists",
		"the underlying cause should be preserved")
}

// TestCoerce_NumericAffinityWholeNumber verifies that a NUMERIC-declared
// column holding an integer value surfaces as a decimal.Decimal, not an
// int64. This pins the cross-driver contract for NUMERIC affinity:
// mattn/go-sqlite3 and Postgres both surface a NUMERIC column as a decimal
// regardless of whether the stored value is whole, so rqlite matches that
// rather than demoting whole values to int64 (issue #839). The CREATE TABLE
// is issued by hand rather than via schema.NewTable because the latter emits
// INTEGER, not NUMERIC, so would not exercise the coercion path. This guards
// against future upstream Sakila schema fixes that would otherwise silently
// retire the existing Sakila-driven coverage.
func TestCoerce_NumericAffinityWholeNumber(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "coerce_numint_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`CREATE TABLE %q (id numeric NOT NULL PRIMARY KEY, name VARCHAR(64) NOT NULL)`,
			tblName))
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`INSERT INTO %q (id, name) VALUES (?, ?)`, tblName), 42, "alpha")
	require.NoError(t, err)

	sink, err := th.QuerySQL(src, nil, fmt.Sprintf(`SELECT id, name FROM %q`, tblName))
	require.NoError(t, err)
	require.Len(t, sink.Recs, 1)
	require.Len(t, sink.Recs[0], 2)

	gotID, ok := sink.Recs[0][0].(decimal.Decimal)
	require.True(t, ok, "expected decimal.Decimal for integer-valued NUMERIC column, got %T", sink.Recs[0][0])
	require.True(t, gotID.Equal(decimal.NewFromInt(42)), "expected 42, got %s", gotID.String())
	require.Equal(t, "alpha", sink.Recs[0][1])
}

// TestCoerce_NumericAffinityDecimal verifies that NUMERIC-declared
// columns holding non-integer values surface as decimal.Decimal.
// Pairs with TestCoerce_NumericAffinityWholeNumber, which covers the
// integer-valued case; both surface as decimal (see issue #839).
func TestCoerce_NumericAffinityDecimal(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "coerce_numdec_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`CREATE TABLE %q (id INTEGER NOT NULL PRIMARY KEY, price numeric NOT NULL)`,
			tblName))
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`INSERT INTO %q (id, price) VALUES (?, ?)`, tblName), 1, 19.99)
	require.NoError(t, err)

	sink, err := th.QuerySQL(src, nil, fmt.Sprintf(`SELECT id, price FROM %q`, tblName))
	require.NoError(t, err)
	require.Len(t, sink.Recs, 1)
	require.Len(t, sink.Recs[0], 2)

	// id is INTEGER affinity, so it round-trips as int64.
	gotID, ok := sink.Recs[0][0].(int64)
	require.True(t, ok, "expected int64 for INTEGER column, got %T", sink.Recs[0][0])
	require.Equal(t, int64(1), gotID)

	// price is NUMERIC affinity with a non-integer value, so it
	// surfaces as decimal.Decimal.
	gotPrice, ok := sink.Recs[0][1].(decimal.Decimal)
	require.True(t, ok, "expected decimal.Decimal for non-integer NUMERIC column, got %T", sink.Recs[0][1])
	require.True(t, gotPrice.Equal(decimal.NewFromFloat(19.99)),
		"expected 19.99, got %s", gotPrice.String())
}

// TestCoerce_RealAffinityFloat verifies that REAL-declared columns
// remain float64 with no demotion. This pins the negative case for
// the coercion logic: future tightening of coerceFloat64 must not
// swallow Float-affinity columns.
func TestCoerce_RealAffinityFloat(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "coerce_real_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`CREATE TABLE %q (id INTEGER NOT NULL PRIMARY KEY, pi REAL NOT NULL)`,
			tblName))
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`INSERT INTO %q (id, pi) VALUES (?, ?)`, tblName), 1, 3.14)
	require.NoError(t, err)

	sink, err := th.QuerySQL(src, nil, fmt.Sprintf(`SELECT pi FROM %q`, tblName))
	require.NoError(t, err)
	require.Len(t, sink.Recs, 1)
	require.Len(t, sink.Recs[0], 1)

	gotPi, ok := sink.Recs[0][0].(float64)
	require.True(t, ok, "expected float64 for REAL column, got %T", sink.Recs[0][0])
	require.InDelta(t, 3.14, gotPi, 1e-9)
}

// TestCopyTable_PreservesFKs verifies that CopyTable carries the source
// table's FOREIGN KEY constraints across to the destination. Uses
// sakila.TblFilmActor as the source because it has a composite PK and
// two outgoing FKs (to actor and film), exercising both single and
// composite-FK preservation through the faithful-DDL rewrite path.
func TestCopyTable_PreservesFKs(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	dstName := "film_actor_fk_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
	})

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: sakila.TblFilmActor}, tablefq.T{Table: dstName}, false)
	require.NoError(t, err)

	srcMd, err := grip.TableMetadata(th.Context, sakila.TblFilmActor)
	require.NoError(t, err)
	dstMd, err := grip.TableMetadata(th.Context, dstName)
	require.NoError(t, err)

	require.NotNil(t, srcMd.FK, "sanity: source film_actor should carry FKs")
	require.NotNil(t, dstMd.FK, "destination should carry FKs after CopyTable")
	require.Len(t, dstMd.FK.Outgoing, len(srcMd.FK.Outgoing),
		"FK count should match source")

	fkKey := func(fk *metadata.ForeignKey) string {
		return fmt.Sprintf("%s|%s->%s",
			fk.RefTable,
			strings.Join(fk.Columns, ","),
			strings.Join(fk.RefColumns, ","))
	}
	srcKeys := map[string]bool{}
	for _, fk := range srcMd.FK.Outgoing {
		srcKeys[fkKey(fk)] = true
	}
	for _, fk := range dstMd.FK.Outgoing {
		require.True(t, srcKeys[fkKey(fk)],
			"dest FK %s not present in source set", fkKey(fk))
	}
}

// TestCopyTable_RewritesSelfFK is the rqlite half of gh759: when a
// source table carries a self-referential FOREIGN KEY (REFERENCES
// <src>(...)), the destination's REFERENCES must name the destination,
// not the source. Otherwise the destination's FKs resolve against the
// source row set, which is the bug.
//
// The structural assertion (TableMetadata.FK.Outgoing[0].RefTable ==
// dstName) is the load-bearing check. The DDL string check is a
// redundant cross-check. FK runtime enforcement isn't exercised here
// because rqlite's stateless HTTP transport doesn't reliably carry
// per-connection PRAGMA foreign_keys across separate requests; the
// sqlite3 sibling test covers that runtime axis.
//
// Table-driven across the cardinal self-FK shapes so the two drivers
// stay in step (gh737 established the rqlite faithful-DDL baseline
// this builds on).
func TestCopyTable_RewritesSelfFK(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	testCases := []struct {
		name string
		// ddl is the source CREATE TABLE; %[1]q is the source table
		// name, double-quoted via the %q verb.
		ddl string
	}{
		{
			name: "column_constraint",
			ddl:  `CREATE TABLE %[1]q (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES %[1]q(id))`,
		},
		{
			name: "table_constraint",
			ddl: `CREATE TABLE %[1]q (id INTEGER PRIMARY KEY, parent_id INTEGER, ` +
				`FOREIGN KEY (parent_id) REFERENCES %[1]q(id))`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			th := testh.New(t)
			src := th.Source(sakila.Rq)
			grip := th.Open(src)
			drvr := grip.SQLDriver()
			db, err := grip.DB(th.Context)
			require.NoError(t, err)

			uniq := stringz.Uniq8()
			srcName := "actor_self_fk_" + uniq
			dstName := "actor_self_fk_bak_" + uniq
			t.Cleanup(func() {
				_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
				_ = drvr.DropTable(th.Context, db, tablefq.T{Table: srcName}, true)
			})

			_, err = db.ExecContext(th.Context, fmt.Sprintf(tc.ddl, srcName))
			require.NoError(t, err)

			_, err = drvr.CopyTable(th.Context, db,
				tablefq.T{Table: srcName}, tablefq.T{Table: dstName}, false)
			require.NoError(t, err)

			md, err := grip.TableMetadata(th.Context, dstName)
			require.NoError(t, err)
			require.NotNil(t, md.FK, "destination should carry an FK after CopyTable")
			require.Len(t, md.FK.Outgoing, 1, "destination should have exactly one outgoing FK")
			require.Equal(t, dstName, md.FK.Outgoing[0].RefTable,
				"FK target must be rewritten to the destination, not left pointing at the source")

			var destDDL string
			require.NoError(t, db.QueryRowContext(th.Context,
				`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, dstName).Scan(&destDDL))
			require.True(t,
				strings.Contains(destDDL, fmt.Sprintf(`REFERENCES %q(`, dstName)) ||
					strings.Contains(destDDL, fmt.Sprintf(`REFERENCES %s(`, dstName)),
				"destination DDL REFERENCES clause must name the destination, got: %s", destDDL)
			require.False(t,
				strings.Contains(destDDL, fmt.Sprintf(`REFERENCES %q(`, srcName)) ||
					strings.Contains(destDDL, fmt.Sprintf(`REFERENCES %s(`, srcName)),
				"destination DDL REFERENCES clause must not still name the source, got: %s", destDDL)
		})
	}
}

// TestCopyTable_LeavesCrossFKsAlone is the rqlite half of the cross-FK
// preservation invariant: REFERENCES whose target is not the source
// table must survive the copy untouched, even when the same table also
// carries a self-FK that does get rewritten. A regression that
// inverted the predicate would silently re-point cross-table FKs at
// the destination, a worse bug than the original gh759. Mirrored on
// both drivers.
func TestCopyTable_LeavesCrossFKsAlone(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	parentName := "parent_xfk_" + uniq
	srcName := "actor_xfk_" + uniq
	dstName := "actor_xfk_bak_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: srcName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: parentName}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (id INTEGER PRIMARY KEY)`, parentName,
	))
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (`+
			`id INTEGER PRIMARY KEY, `+
			`parent_id INTEGER REFERENCES %q(id), `+
			`other_id INTEGER REFERENCES %q(id))`,
		srcName, srcName, parentName,
	))
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: srcName}, tablefq.T{Table: dstName}, false)
	require.NoError(t, err)

	md, err := grip.TableMetadata(th.Context, dstName)
	require.NoError(t, err)
	require.NotNil(t, md.FK)
	require.Len(t, md.FK.Outgoing, 2, "destination should have two outgoing FKs")

	refTables := map[string]bool{}
	for _, fk := range md.FK.Outgoing {
		refTables[fk.RefTable] = true
	}
	require.True(t, refTables[dstName],
		"self-FK must be rewritten to the destination, got refs %v", refTables)
	require.True(t, refTables[parentName],
		"cross-table FK to %q must be preserved, got refs %v", parentName, refTables)
	require.False(t, refTables[srcName],
		"no FK should still name the source after copy, got refs %v", refTables)
}

// TestCopyTable_MultipleSelfFKs verifies every self-FK in a source
// table is rewritten, not just the first; guards against a regression
// that returned after the first match.
func TestCopyTable_MultipleSelfFKs(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	srcName := "actor_multi_fk_" + uniq
	dstName := "actor_multi_fk_bak_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: srcName}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (`+
			`id INTEGER PRIMARY KEY, `+
			`parent_id INTEGER REFERENCES %q(id), `+
			`buddy_id INTEGER REFERENCES %q(id))`,
		srcName, srcName, srcName,
	))
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: srcName}, tablefq.T{Table: dstName}, false)
	require.NoError(t, err)

	md, err := grip.TableMetadata(th.Context, dstName)
	require.NoError(t, err)
	require.NotNil(t, md.FK)
	require.Len(t, md.FK.Outgoing, 2, "destination should have two outgoing FKs")
	for _, fk := range md.FK.Outgoing {
		require.Equal(t, dstName, fk.RefTable,
			"every self-FK must be rewritten to the destination, got %v", fk)
	}
}

// TestCopyTable_CompositeSelfFK verifies composite-column
// FOREIGN KEY(a, b) REFERENCES self(x, y) is rewritten to point at the
// destination. Mirrors the sqlite3 sibling.
func TestCopyTable_CompositeSelfFK(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	srcName := "link_composite_" + uniq
	dstName := "link_composite_bak_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: srcName}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (`+
			`a INTEGER, b INTEGER, x INTEGER, y INTEGER, `+
			`PRIMARY KEY (x, y), `+
			`FOREIGN KEY(a, b) REFERENCES %q(x, y))`,
		srcName, srcName,
	))
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: srcName}, tablefq.T{Table: dstName}, false)
	require.NoError(t, err)

	md, err := grip.TableMetadata(th.Context, dstName)
	require.NoError(t, err)
	require.NotNil(t, md.FK)
	require.Len(t, md.FK.Outgoing, 1)
	require.Equal(t, dstName, md.FK.Outgoing[0].RefTable,
		"composite self-FK must be rewritten to the destination")
}

// TestCopyTable_CaseMismatchSelfFK verifies the driver's case-insensitive
// match predicate: a source table named `actor_...` whose REFERENCES
// target is written as `Actor_...` is still rewritten. SQLite identifier
// comparison is ASCII case-insensitive for unquoted refs; the driver
// uses strings.EqualFold. A regression to case-sensitive matching would
// leave the destination FK pointing at the source.
func TestCopyTable_CaseMismatchSelfFK(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	srcName := "actor_case_" + uniq
	upperFKTarget := strings.ToUpper(srcName[:1]) + srcName[1:]
	dstName := "actor_case_bak_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: srcName}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %s (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES %s(id))`,
		srcName, upperFKTarget,
	))
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: srcName}, tablefq.T{Table: dstName}, false)
	require.NoError(t, err)

	md, err := grip.TableMetadata(th.Context, dstName)
	require.NoError(t, err)
	require.NotNil(t, md.FK)
	require.Len(t, md.FK.Outgoing, 1)
	require.True(t, strings.EqualFold(md.FK.Outgoing[0].RefTable, dstName),
		"FK target must be rewritten to the destination despite case mismatch, got %q",
		md.FK.Outgoing[0].RefTable)
	require.False(t, strings.EqualFold(md.FK.Outgoing[0].RefTable, srcName),
		"FK target must not still name the source after rewrite, got %q",
		md.FK.Outgoing[0].RefTable)
}

// TestCopyTable_SchemaQualifiedDest is the rqlite parity for the
// sqlite3 sibling: a destination with an explicit schema must rewrite
// the CREATE TABLE identifier with the dotted "schema"."table" form
// but the FK REFERENCES target as a bare table token. SQLite's
// foreign_table grammar rule is a single any_name; the runtime
// rejects "schema"."tbl" as a REFERENCES target with a syntax error,
// so a regression would surface at CopyTable's exec rather than via
// the structural metadata assertion.
func TestCopyTable_SchemaQualifiedDest(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	srcName := "actor_schema_" + uniq
	dstName := "actor_schema_bak_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: srcName}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES %q(id))`,
		srcName, srcName,
	))
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: srcName},
		tablefq.T{Schema: "main", Table: dstName},
		false)
	require.NoError(t, err,
		"CopyTable to a schema-qualified destination must succeed; "+
			"the runtime rejects \"main\".\"%s\" as a REFERENCES target",
		dstName)

	md, err := grip.TableMetadata(th.Context, dstName)
	require.NoError(t, err)
	require.NotNil(t, md.FK)
	require.Len(t, md.FK.Outgoing, 1)
	require.Equal(t, dstName, md.FK.Outgoing[0].RefTable,
		"FK target must be the bare destination table, not a schema-qualified form")
}

// TestAlterTableColumnKinds_PreservesFKs verifies that the
// alter-rebuild dance carries the source table's FOREIGN KEY
// constraints across. Uses an ad-hoc parent/child fixture because
// AlterTableColumnKinds is destructive and must not touch the shared
// Sakila tables.
func TestAlterTableColumnKinds_PreservesFKs(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	parentName := "parent_fk_" + uniq
	childName := "child_fk_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: childName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: parentName}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (id INTEGER PRIMARY KEY)`, parentName,
	))
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (`+
			`id INTEGER PRIMARY KEY, parent_id INTEGER, payload TEXT, `+
			`FOREIGN KEY (parent_id) REFERENCES %q(id))`,
		childName, parentName,
	))
	require.NoError(t, err)

	err = drvr.AlterTableColumnKinds(th.Context, db, childName,
		[]string{"payload"}, []kind.Kind{kind.Int})
	require.NoError(t, err)

	md, err := grip.TableMetadata(th.Context, childName)
	require.NoError(t, err)
	require.NotNil(t, md.FK, "child should still carry FK after AlterTableColumnKinds")
	require.Len(t, md.FK.Outgoing, 1,
		"child should retain exactly one FK after AlterTableColumnKinds")
	require.Equal(t, parentName, md.FK.Outgoing[0].RefTable)
}

// TestColumnTypes_EmptyTable verifies that the rqlite-sq wrapper
// driver (drivers/rqlite/sqldriver.go) populates DatabaseTypeName for
// empty result sets. Without the wrapper, gorqlite/stdlib returns the
// empty string for ColumnTypeDatabaseTypeName, which demotes every
// column kind to kind.Unknown and breaks TableColumnTypes for empty
// schemas. This test guards the empty-table path that TableColumnTypes
// relies on for fresh tables.
func TestColumnTypes_EmptyTable(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "coltypes_empty_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	// Mix the affinities so the wrapper has to emit a non-empty type
	// string for each one. NUMERIC is included even though sq's helper
	// maps it to kind.Decimal: the test point is that the wrapper
	// surfaces the declared type, not what kindFromDBTypeName does
	// with it.
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (
			id INTEGER NOT NULL PRIMARY KEY,
			name TEXT,
			ts DATETIME,
			n NUMERIC,
			r REAL,
			b BOOLEAN,
			blob BLOB
		)`, tblName,
	))
	require.NoError(t, err)

	// TableColumnTypes runs through a *sql.Conn so the path matches the
	// one PrepareInsertStmt / NewBatchInsert exercise in production.
	conn, err := db.Conn(th.Context)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	colTypes, err := drvr.TableColumnTypes(th.Context, conn, tblName, nil)
	require.NoError(t, err)
	require.Len(t, colTypes, 7, "expected 7 columns for the declared schema")

	// Expected declared type per column (case-insensitive: SQLite's
	// affinity rules permit any casing on input, and we make no
	// promises about the round-trip casing).
	wantTypes := []string{"INTEGER", "TEXT", "DATETIME", "NUMERIC", "REAL", "BOOLEAN", "BLOB"}
	for i, ct := range colTypes {
		require.NotEmpty(t, ct.DatabaseTypeName(),
			"col %d (%s) must have a non-empty DatabaseTypeName: empty type breaks kind resolution",
			i, ct.Name())
		require.Equal(t, strings.ToUpper(wantTypes[i]), strings.ToUpper(ct.DatabaseTypeName()),
			"col %d (%s) declared type mismatch", i, ct.Name())
	}

	// RecordMeta should map each declared type to its expected
	// non-Unknown kind, end-to-end. This is the assertion that ties
	// the wrapper's DatabaseTypeName back to the rest of the driver.
	recMeta, _, err := drvr.RecordMeta(th.Context, colTypes, nil)
	require.NoError(t, err)
	require.Equal(t, []kind.Kind{
		kind.Int,
		kind.Text,
		kind.Datetime,
		kind.Decimal, // NUMERIC affinity maps to kind.Decimal per kindFromDBTypeName.
		kind.Float,
		kind.Bool,
		kind.Bytes,
	}, recMeta.Kinds())
}

// TestCopyTable_PreservesUniqueConstraints verifies that UNIQUE
// column constraints survive the CopyTable rewrite. Uses an ad-hoc
// source table because Sakila's UNIQUEs are mostly expressed as
// indexes, not column-level constraints. The duplicate-insert step
// also catches the case where the parser produces a CREATE TABLE
// that is syntactically valid but loses the constraint semantics.
func TestCopyTable_PreservesUniqueConstraints(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	srcName := "uniq_src_" + uniq
	dstName := "uniq_dst_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: srcName}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (id INTEGER PRIMARY KEY, email TEXT UNIQUE NOT NULL)`,
		srcName,
	))
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: srcName}, tablefq.T{Table: dstName}, false)
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`INSERT INTO %q (id, email) VALUES (1, 'a@a.com')`, dstName,
	))
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`INSERT INTO %q (id, email) VALUES (2, 'a@a.com')`, dstName,
	))
	require.Error(t, err, "duplicate email should violate UNIQUE on copied table")
}

// TestCopyTable_PreservesDefaultExpression verifies that a column's
// exact DEFAULT expression survives the rewrite. The pre-faithful
// implementation substituted canned per-kind defaults (e.g.
// DEFAULT 0 for any numeric column), so a fresh INSERT against the
// destination would see 0 instead of 50000.
func TestCopyTable_PreservesDefaultExpression(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	srcName := "dflt_src_" + uniq
	dstName := "dflt_dst_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: srcName}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (id INTEGER PRIMARY KEY, salary REAL DEFAULT 50000)`, srcName,
	))
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: srcName}, tablefq.T{Table: dstName}, false)
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`INSERT INTO %q (id) VALUES (1)`, dstName,
	))
	require.NoError(t, err)

	var got float64
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT salary FROM %q WHERE id=1`, dstName)).Scan(&got))
	require.InDelta(t, 50000.0, got, 0.0001,
		"DEFAULT expression should round-trip exactly, not be replaced by per-kind canned default")
}

// TestCopyTable_PreservesAutoIncrement verifies that AUTOINCREMENT
// survives the DDL rewrite. The test does not assert sqlite_sequence
// continuity (a known follow-up): after CopyTable, the destination
// has fresh sqlite_sequence state, but AUTOINCREMENT is still in
// effect, so a new INSERT picks up MAX(rowid)+1, which is greater
// than the count of pre-existing rows.
func TestCopyTable_PreservesAutoIncrement(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	srcName := "ai_src_" + uniq
	dstName := "ai_dst_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: srcName}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)`, srcName,
	))
	require.NoError(t, err)

	for _, name := range []string{"a", "b", "c"} {
		_, err = db.ExecContext(th.Context, fmt.Sprintf(
			`INSERT INTO %q (name) VALUES (?)`, srcName,
		), name)
		require.NoError(t, err)
	}

	affected, err := drvr.CopyTable(th.Context, db,
		tablefq.T{Table: srcName}, tablefq.T{Table: dstName}, true)
	require.NoError(t, err)
	require.Equal(t, int64(3), affected)

	res, err := db.ExecContext(th.Context, fmt.Sprintf(
		`INSERT INTO %q (name) VALUES ('x')`, dstName,
	))
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	require.Greater(t, id, int64(3),
		"AUTOINCREMENT should still be in effect; new id should advance past pre-existing rowids")

	var dstDDL string
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, dstName).Scan(&dstDDL))
	require.Contains(t, strings.ToUpper(dstDDL), "AUTOINCREMENT",
		"destination CREATE TABLE should preserve AUTOINCREMENT keyword")
}

// TestCopyTable_PreservesCompositePK verifies that a composite
// PRIMARY KEY survives the rewrite. Uses sakila.TblFilmActor whose
// PK is (actor_id, film_id). The duplicate-insert step exercises
// the constraint semantics, not just the metadata round-trip.
func TestCopyTable_PreservesCompositePK(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	dstName := "fa_pk_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
	})

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: sakila.TblFilmActor}, tablefq.T{Table: dstName}, false)
	require.NoError(t, err)

	md, err := grip.TableMetadata(th.Context, dstName)
	require.NoError(t, err)

	var pkCols []string
	for _, c := range md.Columns {
		if c.PrimaryKey {
			pkCols = append(pkCols, c.Name)
		}
	}
	require.ElementsMatch(t, []string{"actor_id", "film_id"}, pkCols,
		"destination should retain composite PRIMARY KEY")

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`INSERT INTO %q (actor_id, film_id, last_update) VALUES (1, 1, CURRENT_TIMESTAMP)`,
		dstName,
	))
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`INSERT INTO %q (actor_id, film_id, last_update) VALUES (1, 1, CURRENT_TIMESTAMP)`,
		dstName,
	))
	require.Error(t, err, "duplicate composite PK should be rejected")
}

// TestCopyTable_PreservesCheckConstraints verifies that table-level
// CHECK constraints survive the CopyTable rewrite. The godoc on
// CopyTable lists CHECK as preserved; this test pins that promise
// via a semantic check (insert a row that violates the CHECK and
// expect failure on the destination).
func TestCopyTable_PreservesCheckConstraints(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	srcName := "chk_src_" + uniq
	dstName := "chk_dst_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: srcName}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (id INTEGER PRIMARY KEY, age INTEGER NOT NULL CHECK (age >= 0))`,
		srcName,
	))
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: srcName}, tablefq.T{Table: dstName}, false)
	require.NoError(t, err)

	// Sanity: a valid insert succeeds.
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`INSERT INTO %q (id, age) VALUES (1, 5)`, dstName,
	))
	require.NoError(t, err)

	// CHECK violation: negative age should be rejected on the destination.
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`INSERT INTO %q (id, age) VALUES (2, -1)`, dstName,
	))
	require.Error(t, err,
		"CHECK (age >= 0) should be preserved and reject negative ages")
}

// TestAlterTableColumnKinds_PreservesUniqueAndDefault verifies that
// UNIQUE and a non-trivial DEFAULT expression both survive an
// AlterTableColumnKinds rebuild. The kind swap on email is a no-op
// (TEXT -> TEXT) and is purely to exercise the rewrite path; the
// goal is to confirm that constraints unrelated to the swapped
// column are untouched.
func TestAlterTableColumnKinds_PreservesUniqueAndDefault(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "ud_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`CREATE TABLE %q (id INTEGER PRIMARY KEY, email TEXT UNIQUE, salary REAL DEFAULT 50000)`,
		tblName,
	))
	require.NoError(t, err)

	err = drvr.AlterTableColumnKinds(th.Context, db, tblName,
		[]string{"email"}, []kind.Kind{kind.Text})
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`INSERT INTO %q (id, email) VALUES (1, 'a@a.com')`, tblName,
	))
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`INSERT INTO %q (id, email) VALUES (2, 'a@a.com')`, tblName,
	))
	require.Error(t, err, "UNIQUE on email should survive the rebuild")

	var salary float64
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT salary FROM %q WHERE id=1`, tblName)).Scan(&salary))
	require.InDelta(t, 50000.0, salary, 0.0001,
		"DEFAULT 50000 on salary should survive the rebuild")
}

// TestTableMetadata_ProblematicTableNames reproduces gh777 for the rqlite
// driver: getTableMetadata interpolated the table name with Go's %q,
// including into string-literal position. SQLite resolves a double-quoted
// token as an identifier first, so a table named after a sqlite_master
// column (name, type, sql) turned WHERE name = "name" into a tautology,
// and a table name containing a double quote was emitted with Go backslash
// escaping, which SQLite rejects outright. Mirrors the sqlite3 driver test.
//
// Deliberately NOT parallel: the test creates tables with fixed names
// (name, type, sql, we"ird) in the shared Sakila rqlite database, so it
// can't safely interleave with other tests touching the same server.
func TestTableMetadata_ProblematicTableNames(t *testing.T) {
	tu.SkipShort(t, true)

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblNames := []string{"name", "type", "sql", `we"ird`, "shadowed"}
	// The fixed-name tables live in the shared database, so drop any
	// leftovers from an aborted earlier run before creating: the test
	// must be self-healing.
	dropAll := func() {
		_, _ = db.ExecContext(th.Context, `DROP TRIGGER IF EXISTS "shadowed"`)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: "aab_other"}, true)
		for _, tblName := range tblNames {
			_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
		}
	}
	dropAll()
	t.Cleanup(dropAll)

	// A trigger may share a table's name in sqlite_master. Create the
	// trigger before the same-named table so the trigger row precedes the
	// table row: without the type filter in the metadata query, the
	// trigger row shadowed the table and the metadata was misreported.
	_, err = db.ExecContext(th.Context, `CREATE TABLE aab_other (x INTEGER)`)
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context,
		`CREATE TRIGGER "shadowed" AFTER INSERT ON aab_other BEGIN SELECT 1; END`)
	require.NoError(t, err)

	for _, tblName := range tblNames {
		quoted := stringz.DoubleQuote(tblName)
		_, err = db.ExecContext(th.Context, fmt.Sprintf(
			"CREATE TABLE %s (id INTEGER PRIMARY KEY, val TEXT)", quoted,
		))
		require.NoError(t, err)
		_, err = db.ExecContext(th.Context, fmt.Sprintf(
			"INSERT INTO %s (val) VALUES ('a'), ('b')", quoted,
		))
		require.NoError(t, err)
	}

	for _, tblName := range tblNames {
		t.Run(tu.Name(tblName), func(t *testing.T) {
			md, err := grip.TableMetadata(th.Context, tblName)
			require.NoError(t, err)
			require.Equal(t, tblName, md.Name)
			require.Equal(t, int64(2), md.RowCount)
			require.Equal(t, sqlz.TableTypeTable, md.TableType)
			require.Len(t, md.Columns, 2)
			require.Equal(t, "id", md.Columns[0].Name)
			require.Equal(t, "val", md.Columns[1].Name)
		})
	}
}

// TestAlterTableColumnKinds_ForeignKeyEnforcement reproduces gh776: the
// table-rebuild dance in AlterTableColumnKinds carried PRAGMA
// foreign_keys=off inside its transaction-wrapped batch, where SQLite
// specifies the pragma is a no-op. On a node enforcing foreign keys, the
// rebuild's DROP TABLE step failed with "FOREIGN KEY constraint failed"
// whenever the altered table was referenced by another table's FK. The
// fix issues the pragma off/restore as separate non-transactional
// requests around the (still atomic) rebuild batch.
//
// The standard test server runs without -fk, so the test enables
// enforcement on the node's write connection via a non-transactional
// pragma, exactly as a -fk node would have it at boot.
//
// Deliberately NOT parallel: the foreign_keys pragma applies to the
// node's shared write connection, so toggling it would affect writes
// from concurrently running tests.
func TestAlterTableColumnKinds_ForeignKeyEnforcement(t *testing.T) {
	tu.SkipShort(t, true)

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	parentTbl := "fkparent_" + stringz.Uniq8()
	childTbl := "fkchild_" + stringz.Uniq8()
	t.Cleanup(func() {
		// Restore the node default (the test server runs without -fk),
		// then drop child before parent.
		_ = rqlite.ExecNonTx(th.Context, db, "PRAGMA foreign_keys=off")
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: childTbl}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: parentTbl}, true)
	})

	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		"CREATE TABLE %s (id INTEGER PRIMARY KEY, val INTEGER NOT NULL)",
		stringz.DoubleQuote(parentTbl),
	))
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		"CREATE TABLE %s (id INTEGER PRIMARY KEY, pid INTEGER NOT NULL REFERENCES %s(id))",
		stringz.DoubleQuote(childTbl), stringz.DoubleQuote(parentTbl),
	))
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		"INSERT INTO %s (id, val) VALUES (1, 42)", stringz.DoubleQuote(parentTbl),
	))
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		"INSERT INTO %s (id, pid) VALUES (1, 1)", stringz.DoubleQuote(childTbl),
	))
	require.NoError(t, err)

	// Enable FK enforcement on the node's write connection. This must go
	// through the non-transactional path: a pragma in a
	// transaction-wrapped request is a no-op.
	require.NoError(t, rqlite.ExecNonTx(th.Context, db, "PRAGMA foreign_keys=on"))

	// Sanity check: enforcement is live, so a dangling child insert fails.
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		"INSERT INTO %s (id, pid) VALUES (99, 999)", stringz.DoubleQuote(childTbl),
	))
	require.Error(t, err)
	require.Contains(t, err.Error(), "FOREIGN KEY constraint failed")

	// The rebuild. Pre-fix, this failed at the DROP TABLE step with
	// "FOREIGN KEY constraint failed" because the in-batch pragma never
	// disabled enforcement.
	require.NoError(t, drvr.AlterTableColumnKinds(th.Context, db, parentTbl,
		[]string{"val"}, []kind.Kind{kind.Text}))

	// Kind changed, and data in both tables survived the rebuild.
	md, err := grip.TableMetadata(th.Context, parentTbl)
	require.NoError(t, err)
	require.Equal(t, kind.Text, md.Column("val").Kind)

	var parentVal string
	require.NoError(t, db.QueryRowContext(th.Context,
		"SELECT val FROM "+stringz.DoubleQuote(parentTbl)+" WHERE id = 1").
		Scan(&parentVal))
	require.Equal(t, "42", parentVal)
	var childCount int64
	require.NoError(t, db.QueryRowContext(th.Context,
		"SELECT COUNT(*) FROM "+stringz.DoubleQuote(childTbl)).Scan(&childCount))
	require.Equal(t, int64(1), childCount)

	// AlterTableColumnKinds restores the foreign_keys pragma to the
	// node's configured default: the write connection's live pragma
	// state is not readable over the HTTP API (PRAGMA foreign_keys via
	// /db/query is served by the read-only pool), so the boot default is
	// the restore target. This test server's default is off, so a
	// dangling insert succeeding here proves the restore ran. On a node
	// actually running -fk, the same restore re-enables enforcement.
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		"INSERT INTO %s (id, pid) VALUES (100, 999)", stringz.DoubleQuote(childTbl),
	))
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		"DELETE FROM %s WHERE id = 100", stringz.DoubleQuote(childTbl),
	))
	require.NoError(t, err)

	// Re-enable enforcement: the child's FK must still be wired to the
	// rebuilt parent (the DROP/RENAME preserved the relationship).
	require.NoError(t, rqlite.ExecNonTx(th.Context, db, "PRAGMA foreign_keys=on"))
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		"INSERT INTO %s (id, pid) VALUES (101, 999)", stringz.DoubleQuote(childTbl),
	))
	require.Error(t, err)
	require.Contains(t, err.Error(), "FOREIGN KEY constraint failed")
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		"INSERT INTO %s (id, pid) VALUES (2, 1)", stringz.DoubleQuote(childTbl),
	))
	require.NoError(t, err)
}

// TestCopyTable_CopiesIndexesAndTriggers is the rqlite half of gh758:
// CopyTable carries the source table's companion objects (indexes and
// triggers, separate sqlite_master rows) across to the destination,
// renamed to "<orig-name>_<dest-table>", riding the same writeAtomic
// batch as the CREATE and INSERT. The UNIQUE constraint's automatic
// index has NULL sql in sqlite_master and must be skipped by the
// companion rewrite. Trigger timing is load-bearing: the copied
// trigger must not fire on the rows being copied, only on subsequent
// inserts into the destination. Mirrors the sqlite3 sibling test.
func TestCopyTable_CopiesIndexesAndTriggers(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	srcName := "companion_src_" + uniq
	logName := "companion_log_" + uniq
	dstName := "companion_dst_" + uniq
	idxName := "idx_name_" + uniq
	trgName := "trg_bi_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: srcName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: logName}, true)
	})

	for _, stmt := range []string{
		fmt.Sprintf(`CREATE TABLE %q (id INTEGER PRIMARY KEY, name TEXT, email TEXT UNIQUE)`,
			srcName),
		fmt.Sprintf(`CREATE TABLE %q (id INTEGER PRIMARY KEY, msg TEXT)`, logName),
		fmt.Sprintf(`CREATE INDEX %q ON %q (name)`, idxName, srcName),
		fmt.Sprintf(`CREATE TRIGGER %q BEFORE INSERT ON %q BEGIN
			INSERT INTO %q (msg) VALUES (NEW.name);
		END`, trgName, srcName, logName),
		fmt.Sprintf(`INSERT INTO %q (name, email) VALUES ('alice', 'a@x.com'), ('bob', 'b@x.com')`,
			srcName),
	} {
		_, err = db.ExecContext(th.Context, stmt)
		require.NoError(t, err)
	}

	copied, err := drvr.CopyTable(th.Context, db,
		tablefq.T{Table: srcName}, tablefq.T{Table: dstName}, true)
	require.NoError(t, err)
	require.Equal(t, int64(2), copied)

	// The copied trigger must not have fired on the copied rows: the
	// source inserts logged 2 rows, and the copy must not add more.
	var logCount int64
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT count(*) FROM %q`, logName)).Scan(&logCount))
	require.Equal(t, int64(2), logCount,
		"copied trigger must not fire on the rows being copied")

	// The explicit index is carried over, renamed, and re-targeted.
	var idxSQL string
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT sql FROM sqlite_master WHERE type='index' AND name=? AND tbl_name=?`,
		idxName+"_"+dstName, dstName).Scan(&idxSQL))
	require.Contains(t, idxSQL, fmt.Sprintf("%q", dstName))
	require.NotContains(t, idxSQL, fmt.Sprintf("ON %s ", srcName))

	// Exactly one explicit (non-NULL sql) index on the destination: the
	// UNIQUE constraint's automatic index has NULL sql and must not have
	// been duplicated by the companion copy.
	var explicitIdxCount int64
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT count(*) FROM sqlite_master WHERE type='index' AND tbl_name=?
			AND sql IS NOT NULL`, dstName).Scan(&explicitIdxCount))
	require.Equal(t, int64(1), explicitIdxCount)

	// The trigger is carried over, renamed, and re-targeted; its body's
	// cross-table reference (the log table) is left untouched.
	var trgSQL string
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT sql FROM sqlite_master WHERE type='trigger' AND name=? AND tbl_name=?`,
		trgName+"_"+dstName, dstName).Scan(&trgSQL))
	require.Contains(t, trgSQL, fmt.Sprintf(`ON %q`, dstName))
	require.Contains(t, trgSQL, logName,
		"cross-table body reference must be preserved")

	// The copied trigger's side effect fires on insert into the destination.
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		`INSERT INTO %q (name, email) VALUES ('dave', 'd@x.com')`, dstName,
	))
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT count(*) FROM %q WHERE msg='dave'`, logName)).Scan(&logCount))
	require.Equal(t, int64(1), logCount,
		"copied trigger must fire on insert into the destination")

	// The source's companions are untouched: original names, original table.
	var srcCompanionCount int64
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT count(*) FROM sqlite_master WHERE tbl_name=? AND name IN (?, ?)`,
		srcName, idxName, trgName).Scan(&srcCompanionCount))
	require.Equal(t, int64(2), srcCompanionCount)
}

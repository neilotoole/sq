package sqlite3_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/sqlite3/sqlparser"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

func TestSmoke(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.SL3)

	sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+sakila.TblFilm)
	require.NoError(t, err)
	require.Equal(t, sakila.TblFilmCount, len(sink.Recs))
}

func TestQueryEmptyTable(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.SL3)

	// Get an empty table by copying an existing one
	tblName := th.CopyTable(true, src, tablefq.From(sakila.TblFilm), tablefq.T{}, false)
	require.Equal(t, int64(0), th.RowCount(src, tblName))

	sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+tblName)
	require.NoError(t, err)
	require.Equal(t, 0, len(sink.Recs))
}

// TestExhibitDriverColumnTypesBehavior shows the unusual
// behavior of SQLite wrt column types. The following is observed:
//
//  1. If rows.ColumnTypes is invoked prior to rows.Next being
//     invoked, the column ScanType will be nil.
//
//     UPDATE: ^^ As of mattn/go-sqlite3@v1.14.16 (and probably earlier)
//     this behavior seems to have changed.
//
//  2. The values returned by rows.ColumnTypes can change after
//     each call to rows.Next. This is because of SQLite's dynamic
//     typing: any value can be stored in any column.
//
// The second fact is potentially problematic for sq, as sq expects
// that the values of a column are each of the same type. Thus, sq
// will likely encounter problems dealing with SQLite tables
// that have mixed data types in columns.
func TestExhibitDriverColumnTypesBehavior(t *testing.T) {
	t.Parallel()

	th, src, _, _, db := testh.NewWith(t, sakila.SL3)
	t.Log("using source: " + src.Location)

	tblName := stringz.UniqTableName("scan_test")
	createStmt := "CREATE TABLE " + tblName + " (col1 REAL)"
	insertStmt := "INSERT INTO " + tblName + " VALUES(?)"
	query := "SELECT * FROM " + tblName

	// Create the table
	th.ExecSQL(src, createStmt)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	// 1. Demonstrate that ColumnType.ScanType now correctly returns
	//    a valid value when rows.ColumnTypes is invoked prior to the first
	//    invocation of rows.Next. In earlier versions of the driver,
	//    it was necessary to invoke rows.Next first.
	rows1, err := db.Query(query)
	require.NoError(t, err)
	defer rows1.Close()

	colTypes, err := rows1.ColumnTypes()
	require.NoError(t, err)
	require.Equal(t, colTypes[0].Name(), "col1")
	scanType := colTypes[0].ScanType()
	require.Equal(t, sqlz.RTypeNullFloat64, scanType)

	require.False(t, rows1.Next()) // no rows yet since table is empty
	colTypes, err = rows1.ColumnTypes()
	require.Error(t, err, "ColumnTypes returns an error because the Next call closed rows")
	require.Nil(t, colTypes)

	// 2. In earlier versions of mattn/sqlite3, a column's scan type
	//    could be different for each row. Now, it seems that the
	//    returned scan type is the type of the column in the first
	//    non-null row. This does result in some weirdness: the reported
	//    scan type could be sql.NullFloat64, but passing *interface{} to
	//    row.Scan could result the variable being populated
	//    with, e.g. a string. Such is the SQLite life.

	// Insert values of various types
	_, err = db.Exec(insertStmt, nil)
	require.NoError(t, err)
	_, err = db.Exec(insertStmt, fixt.Float)
	require.NoError(t, err)
	_, err = db.Exec(insertStmt, fixt.Text)
	require.NoError(t, err)

	rows2, err := db.Query(query)
	require.NoError(t, err)
	defer rows2.Close()
	colTypes, err = rows2.ColumnTypes()
	require.NoError(t, err)
	scanType = colTypes[0].ScanType()
	require.Equal(t, sqlz.RTypeNullFloat64, scanType)

	var got any

	// 1st data row (nil)
	require.True(t, rows2.Next())
	require.NoError(t, rows2.Scan(&got))
	require.True(t, nil == got)
	colTypes, err = rows2.ColumnTypes()
	require.NoError(t, err)
	scanType = colTypes[0].ScanType()
	t.Logf("%s", scanType)
	require.Equal(t, sqlz.RTypeNullFloat64, scanType)

	// 2nd data row (float64)
	require.True(t, rows2.Next())
	require.NoError(t, rows2.Scan(&got))
	_, ok := got.(float64)
	require.True(t, ok)
	colTypes, err = rows2.ColumnTypes()
	require.NoError(t, err)
	scanType = colTypes[0].ScanType()
	t.Logf("%s", scanType)
	require.Equal(t, sqlz.RTypeNullFloat64.String(), scanType.String())

	// 3nd data row (string)
	require.True(t, rows2.Next())
	require.NoError(t, rows2.Scan(&got))

	_, ok = got.(string)
	require.True(t, ok, "a string was returned to us")
	colTypes, err = rows2.ColumnTypes()
	require.NoError(t, err)
	scanType = colTypes[0].ScanType()
	t.Log(scanType.String())
	require.NotNil(t, scanType, "scan type should be non-nil because the value is not nil")
	require.Equal(t, sqlz.RTypeNullFloat64.String(), scanType.String(),
		"the scan type is still sql.NullFloat64, even though the returned value is string")

	require.False(t, rows2.Next(), "should be end of rows")
	require.Nil(t, rows2.Err())
}

func TestDriver_CreateTable_NotNullDefault(t *testing.T) {
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.SL3)

	tblName := stringz.UniqTableName(t.Name())
	colNames, colKinds := fixt.ColNamePerKind(drvr.Dialect().IntBool, false, false)

	tblDef := schema.NewTable(tblName, colNames, colKinds)
	for _, colDef := range tblDef.Cols {
		colDef.NotNull = true
		colDef.HasDefault = true
	}

	err := drvr.CreateTable(th.Context, db, tblDef)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	th.InsertDefaultRow(src, tblName)

	sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+tblName)
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	require.Equal(t, len(colNames), len(sink.RecMeta))
	for i := range sink.Recs[0] {
		require.NotNil(t, sink.Recs[0][i])
	}
}

func TestPathFromLocation(t *testing.T) {
	testCases := []struct {
		loc     string
		want    string
		wantErr bool
	}{
		{loc: "sqlite3:///test.db", want: "/test.db"},
		{loc: "postgres:///test.db", wantErr: true},
		{loc: `sqlite3://C:/dir/sakila.db`, want: `C:/dir/sakila.db`},
		// gh720: connection-string parameters must be stripped.
		{loc: "sqlite3:///test.db?mode=ro", want: "/test.db"},
		{loc: "sqlite3:///test.db?cache=shared&mode=rw", want: "/test.db"},
		{loc: "sqlite3:///test.db?immutable=1", want: "/test.db"},
	}

	for _, tc := range testCases {
		t.Run(tc.loc, func(t *testing.T) {
			src := &source.Source{
				Handle:   "@h1",
				Type:     drivertype.SQLite,
				Location: tc.loc,
			}

			got, err := sqlite3.PathFromLocation(src)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			want := filepath.FromSlash(tc.want) // for win/unix testing interoperability
			require.Equal(t, want, got)
		})
	}
}

func TestMungeLocation(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	cwd = filepath.ToSlash(cwd)

	root, err := filepath.Abs("/")
	require.NoError(t, err)
	t.Log(root)
	root = filepath.ToSlash(root)

	cwdWant := "sqlite3://" + cwd + "/sakila.db"

	t.Log("cwdWant:", cwdWant)
	t.Log("root:", root)
	testCases := []struct {
		in        string
		want      string
		onlyForOS string
		wantErr   bool
	}{
		{
			in:      "",
			wantErr: true,
		},
		{
			in:   "sqlite3:///path/to/sakila.db",
			want: "sqlite3://" + root + "path/to/sakila.db",
		},
		{
			in:   "sqlite3://sakila.db",
			want: cwdWant,
		},
		{
			in:   "sqlite3:sakila.db",
			want: cwdWant,
		},
		{
			in:   "sqlite3:/sakila.db",
			want: "sqlite3://" + root + "sakila.db",
		},
		{
			in:   "sakila.db",
			want: cwdWant,
		},
		{
			in:   "./sakila.db",
			want: cwdWant,
		},
		{
			in:   "/path/to/sakila.db",
			want: "sqlite3://" + root + "path/to/sakila.db",
		},
		// gh720: connection-string parameters must round-trip cleanly.
		{
			in:   "sqlite3:///path/to/sakila.db?mode=ro",
			want: "sqlite3://" + root + "path/to/sakila.db?mode=ro",
		},
		{
			in:   "sqlite3:///path/to/sakila.db?cache=shared&mode=rw",
			want: "sqlite3://" + root + "path/to/sakila.db?cache=shared&mode=rw",
		},
		{
			in:   "sakila.db?mode=ro",
			want: cwdWant + "?mode=ro",
		},
		{
			in:   "sqlite3:sakila.db?immutable=1",
			want: cwdWant + "?immutable=1",
		},
		{
			in:   `C:/Users/neil/work/sq/drivers/sqlite3/testdata/sakila.db`,
			want: `sqlite3://C:/Users/neil/work/sq/drivers/sqlite3/testdata/sakila.db`,
			// The current impl of MungeLocation relies upon OS-specific functions
			// in pkg filepath. Thus, we skip this test on non-Windows OSes.
			// MungeLocation could probably be rewritten to be OS-independent?
			onlyForOS: "windows",
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			if tc.onlyForOS != "" && tc.onlyForOS != runtime.GOOS {
				t.Skipf("Skipping because this test is only for OS {%s}, but have {%s}",
					tc.onlyForOS, runtime.GOOS)
				return
			}

			got, err := sqlite3.MungeLocation(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSQLQuery_Whitespace(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.SL3Whitespace)

	sink, err := th.QuerySQL(src, nil, `SELECT * FROM "film actor"`)
	require.NoError(t, err)
	require.Equal(t, sakila.TblFilmActorCount, len(sink.Recs))

	sink, err = th.QuerySQL(src, nil, `SELECT * FROM "actor"`)
	require.NoError(t, err)
	require.Equal(t, "first name", sink.RecMeta[1].Name())
	require.Equal(t, "first name", sink.RecMeta[1].MungedName())
	require.Equal(t, "last name", sink.RecMeta[2].Name())
	require.Equal(t, "last name", sink.RecMeta[2].MungedName())
}

func TestDriveri_AlterTableColumnKinds(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@test",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + tu.TempFile(t, "test.db"),
	}

	ogTbl := &schema.Table{
		Name:          "og_table",
		PKColName:     "",
		AutoIncrement: false,
		Cols:          nil,
	}

	ogColName := &schema.Column{
		Name:    "name",
		Table:   ogTbl,
		Kind:    kind.Text,
		NotNull: true,
	}
	ogColAge := &schema.Column{
		Name:    "age",
		Table:   ogTbl,
		Kind:    kind.Int,
		NotNull: true,
	}
	ogColWeight := &schema.Column{
		Name:    "weight",
		Table:   ogTbl,
		Kind:    kind.Int,
		NotNull: true,
	}

	ogTbl.Cols = []*schema.Column{ogColName, ogColAge, ogColWeight}
	grip := th.Open(src)

	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	drvr := grip.SQLDriver()

	err = drvr.CreateTable(th.Context, db, ogTbl)
	require.NoError(t, err)

	gotTblMeta, err := grip.TableMetadata(th.Context, ogTbl.Name)
	require.NoError(t, err)
	require.Equal(t, 3, len(gotTblMeta.Columns))
	require.Equal(t, kind.Int, gotTblMeta.Column("age").Kind)
	require.Equal(t, kind.Int, gotTblMeta.Column("weight").Kind)

	alterColNames := []string{"age", "weight"}
	alterColKinds := []kind.Kind{kind.Text, kind.Float}

	err = drvr.AlterTableColumnKinds(th.Context, db, ogTbl.Name, alterColNames, alterColKinds)
	require.NoError(t, err)

	gotTblMeta, err = grip.TableMetadata(th.Context, ogTbl.Name)
	require.NoError(t, err)
	require.Equal(t, 3, len(gotTblMeta.Columns))
	require.Equal(t, kind.Text.String(), gotTblMeta.Column("age").Kind.String())
	require.Equal(t, kind.Float.String(), gotTblMeta.Column("weight").Kind.String())
}

// TestDriveri_AlterTableColumnKinds_QuotedIdentifier reproduces gh752: a
// column declared with a non-double-quote SQLite identifier-quote style
// (square brackets, backticks, or single quotes) used to fail the lookup in
// AlterTableColumnKinds because sqlparser.ColDef.Name only stripped
// double-quotes. The shared parser now strips all four legal styles.
func TestDriveri_AlterTableColumnKinds_QuotedIdentifier(t *testing.T) {
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
			th := testh.New(t)
			src := &source.Source{
				Handle:   "@test",
				Type:     drivertype.SQLite,
				Location: "sqlite3://" + tu.TempFile(t, "test.db"),
			}

			grip := th.Open(src)
			db, err := grip.DB(th.Context)
			require.NoError(t, err)
			drvr := grip.SQLDriver()

			_, err = db.ExecContext(th.Context, `CREATE TABLE example (`+tc.colDecl+`)`)
			require.NoError(t, err)

			err = drvr.AlterTableColumnKinds(th.Context, db, "example",
				[]string{"age"}, []kind.Kind{kind.Text})
			require.NoError(t, err)

			md, err := grip.TableMetadata(th.Context, "example")
			require.NoError(t, err)
			require.Equal(t, kind.Text.String(), md.Column("age").Kind.String())
		})
	}
}

// TestDriveri_AlterTableColumnKinds_ColumnNamePrefixesType reproduces
// gh750: a column whose name shares its declared-case prefix with the
// type token (e.g. `text_data text`) caused the old
// `strings.Replace(colDef.Raw, colDef.RawType, wantType, 1)` to clobber
// the name's prefix instead of the actual type. The offset-based rewrite
// targets the parsed RawType position directly, so the name is
// preserved.
func TestDriveri_AlterTableColumnKinds_ColumnNamePrefixesType(t *testing.T) {
	testCases := []struct {
		name    string
		colDecl string
		colName string
	}{
		{
			// Name and type both lowercase, name is the type's prefix.
			name:    "lowercase_prefix",
			colDecl: `text_data text NOT NULL`,
			colName: "text_data",
		},
		{
			// Name and type both uppercase, name is the type's prefix.
			name:    "uppercase_prefix",
			colDecl: `TEXT_DATA TEXT NOT NULL`,
			colName: "TEXT_DATA",
		},
		{
			// Square-bracket quoted name that contains the type token.
			name:    "bracket_quoted_prefix",
			colDecl: `[TEXT_DATA] TEXT NOT NULL`,
			colName: "TEXT_DATA",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			th := testh.New(t)
			src := &source.Source{
				Handle:   "@test",
				Type:     drivertype.SQLite,
				Location: "sqlite3://" + tu.TempFile(t, "test.db"),
			}

			grip := th.Open(src)
			db, err := grip.DB(th.Context)
			require.NoError(t, err)
			drvr := grip.SQLDriver()

			_, err = db.ExecContext(th.Context, `CREATE TABLE collide (`+tc.colDecl+`)`)
			require.NoError(t, err)

			err = drvr.AlterTableColumnKinds(th.Context, db, "collide",
				[]string{tc.colName}, []kind.Kind{kind.Int})
			require.NoError(t, err)

			md, err := grip.TableMetadata(th.Context, "collide")
			require.NoError(t, err)
			require.Len(t, md.Columns, 1, "the column should still exist after alter")
			require.Equal(t, tc.colName, md.Columns[0].Name,
				"the column name must survive the alter; substring-replace would have clobbered it")
			require.Equal(t, kind.Int.String(), md.Column(tc.colName).Kind.String())
		})
	}
}

// TestDriveri_AlterTableColumnKinds_PreservesAutoincrementSeq reproduces
// gh757: the table-rebuild dance in AlterTableColumnKinds dropped the
// original table, which removed its sqlite_sequence row, so AUTOINCREMENT
// restarted from MAX(rowid)+1 instead of seq+1. With rows previously
// deleted from the high end, the next insert silently reused their ids.
func TestDriveri_AlterTableColumnKinds_PreservesAutoincrementSeq(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@test",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + tu.TempFile(t, "test.db"),
	}

	grip := th.Open(src)
	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	drvr := grip.SQLDriver()

	_, err = db.ExecContext(th.Context,
		`CREATE TABLE seq_tbl (id INTEGER PRIMARY KEY AUTOINCREMENT, val INTEGER NOT NULL)`)
	require.NoError(t, err)

	for i := 1; i <= 10; i++ {
		_, err = db.ExecContext(th.Context, `INSERT INTO seq_tbl (val) VALUES (?)`, i)
		require.NoError(t, err)
	}
	_, err = db.ExecContext(th.Context, `DELETE FROM seq_tbl WHERE id > 5`)
	require.NoError(t, err)

	var seq, maxID int64
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT seq FROM sqlite_sequence WHERE name='seq_tbl'`).Scan(&seq))
	require.Equal(t, int64(10), seq)
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT MAX(id) FROM seq_tbl`).Scan(&maxID))
	require.Equal(t, int64(5), maxID)

	require.NoError(t, drvr.AlterTableColumnKinds(th.Context, db, "seq_tbl",
		[]string{"val"}, []kind.Kind{kind.Text}))

	res, err := db.ExecContext(th.Context, `INSERT INTO seq_tbl (val) VALUES ('post-alter')`)
	require.NoError(t, err)
	newID, err := res.LastInsertId()
	require.NoError(t, err)
	require.Equal(t, int64(11), newID,
		"AUTOINCREMENT must continue from the preserved sequence (seq+1), not MAX(rowid)+1")
}

// TestDriveri_AlterTableColumnKinds_AutoincrementSeqEmptiedTable covers the
// gh757 fix for a fully-emptied AUTOINCREMENT table: every row is deleted
// before the rebuild, so the copy moves zero rows, yet the preserved
// sequence must still dictate the next id. Guards against a restore
// implementation that only works when the copy carries rows across.
func TestDriveri_AlterTableColumnKinds_AutoincrementSeqEmptiedTable(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@test",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + tu.TempFile(t, "test.db"),
	}

	grip := th.Open(src)
	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	drvr := grip.SQLDriver()

	_, err = db.ExecContext(th.Context,
		`CREATE TABLE seq_tbl (id INTEGER PRIMARY KEY AUTOINCREMENT, val INTEGER NOT NULL)`)
	require.NoError(t, err)

	for i := 1; i <= 10; i++ {
		_, err = db.ExecContext(th.Context, `INSERT INTO seq_tbl (val) VALUES (?)`, i)
		require.NoError(t, err)
	}
	_, err = db.ExecContext(th.Context, `DELETE FROM seq_tbl`)
	require.NoError(t, err)

	var seq int64
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT seq FROM sqlite_sequence WHERE name='seq_tbl'`).Scan(&seq))
	require.Equal(t, int64(10), seq)

	require.NoError(t, drvr.AlterTableColumnKinds(th.Context, db, "seq_tbl",
		[]string{"val"}, []kind.Kind{kind.Text}))

	res, err := db.ExecContext(th.Context, `INSERT INTO seq_tbl (val) VALUES ('post-alter')`)
	require.NoError(t, err)
	newID, err := res.LastInsertId()
	require.NoError(t, err)
	require.Equal(t, int64(11), newID,
		"AUTOINCREMENT must continue from the preserved sequence even when the table was emptied")
}

// TestDriveri_AlterTableColumnKinds_AutoincrementSeqNeighborUntouched covers
// two gh757 hazards in one: altering a plain (non-AUTOINCREMENT) table must
// succeed when the database's sqlite_sequence table exists but has no row
// for the altered table, and the rebuild must not disturb the sqlite_sequence
// rows of neighboring tables.
func TestDriveri_AlterTableColumnKinds_AutoincrementSeqNeighborUntouched(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@test",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + tu.TempFile(t, "test.db"),
	}

	grip := th.Open(src)
	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	drvr := grip.SQLDriver()

	_, err = db.ExecContext(th.Context,
		`CREATE TABLE seqful (id INTEGER PRIMARY KEY AUTOINCREMENT, val INTEGER NOT NULL)`)
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context, `INSERT INTO seqful (val) VALUES (1), (2), (3)`)
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context, `CREATE TABLE plain (val INTEGER NOT NULL)`)
	require.NoError(t, err)

	require.NoError(t, drvr.AlterTableColumnKinds(th.Context, db, "plain",
		[]string{"val"}, []kind.Kind{kind.Text}),
		"altering a table with no sqlite_sequence row must succeed")

	var seqfulSeq int64
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT seq FROM sqlite_sequence WHERE name='seqful'`).Scan(&seqfulSeq))
	require.Equal(t, int64(3), seqfulSeq,
		"the neighboring table's sqlite_sequence row must survive the rebuild untouched")

	var n int
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT COUNT(*) FROM sqlite_sequence WHERE name='plain'`).Scan(&n))
	require.Equal(t, 0, n, "no sqlite_sequence row should be invented for a plain table")
}

// TestDriveri_CopyTable_TableIdentInDefaultLiteral verifies that CopyTable
// rewrites only the table identifier and leaves a substring-matching
// occurrence inside a column DEFAULT expression untouched. Regression
// safeguard for gh750's offset-based identifier rewrite.
func TestDriveri_CopyTable_TableIdentInDefaultLiteral(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@test",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + tu.TempFile(t, "test.db"),
	}

	grip := th.Open(src)
	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	drvr := grip.SQLDriver()

	// Source table "actor" with a column whose DEFAULT literal contains
	// the substring "actor".
	_, err = db.ExecContext(th.Context,
		`CREATE TABLE actor (id INTEGER, tag TEXT DEFAULT 'actor_tag')`)
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context, `INSERT INTO actor (id) VALUES (1)`)
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: "actor"}, tablefq.T{Table: "actor_bak"}, true)
	require.NoError(t, err)

	// Destination table exists; the DEFAULT literal's 'actor_tag'
	// substring must have been preserved (not rewritten to 'actor_bak_tag').
	_, err = db.ExecContext(th.Context, `INSERT INTO actor_bak (id) VALUES (2)`)
	require.NoError(t, err)
	var tag string
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT tag FROM actor_bak WHERE id=2`).Scan(&tag))
	require.Equal(t, "actor_tag", tag,
		"the DEFAULT literal must not be rewritten by the table-identifier substitution")
}

// openSqliteForFKTest opens a fresh sqlite3 source with a connection
// pinned to a single underlying connection. Pinning is load-bearing for
// FK enforcement assertions: PRAGMA foreign_keys is per-connection in
// SQLite, and without SetMaxOpenConns(1) subsequent ExecContext calls
// might land on a different pool connection where the PRAGMA was never
// set.
func openSqliteForFKTest(t *testing.T) (*testh.Helper, *sql.DB, driver.SQLDriver) {
	t.Helper()
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@test",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + tu.TempFile(t, "test.db"),
	}
	grip := th.Open(src)
	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)

	_, err = db.ExecContext(th.Context, `PRAGMA foreign_keys = ON`)
	require.NoError(t, err)
	return th, db, grip.SQLDriver()
}

// assertSelfFKRewritten parses destDDL via the sqlparser and asserts
// every REFERENCES target's dequoted name equals dstName (and not
// srcName), regardless of which of SQLite's four identifier-quote styles
// (double-quote, single-quote, backtick, square brackets) the source
// DDL carried. A naive substring check would silently miss a leftover
// source ref written in a quote style not covered by the check.
//
// Only valid for tests whose source DDL contains self-FKs and no
// cross-table FKs.
func assertSelfFKRewritten(t *testing.T, destDDL, srcName, dstName string) {
	t.Helper()
	refs, err := sqlparser.ExtractForeignTableRefsFromCreateTableStmt(destDDL)
	require.NoError(t, err, "destination DDL must parse: %s", destDDL)
	require.NotEmpty(t, refs,
		"destination DDL should still carry at least one REFERENCES clause: %s", destDDL)
	for _, r := range refs {
		require.False(t, strings.EqualFold(r.Table, srcName),
			"REFERENCES must no longer name the source %q (got %q in %s)",
			srcName, r.RawTable, destDDL)
		require.True(t, strings.EqualFold(r.Table, dstName),
			"REFERENCES must name the destination %q (got %q in %s)",
			dstName, r.RawTable, destDDL)
	}
}

// TestDriveri_CopyTable_RewritesSelfFK verifies gh759: a CREATE TABLE
// with a self-referential FOREIGN KEY (REFERENCES <src>(...)) must have
// the FK target rewritten to the destination, so the destination's FKs
// resolve against itself rather than the source.
func TestDriveri_CopyTable_RewritesSelfFK(t *testing.T) {
	testCases := []struct {
		name string
		// ddl is the source CREATE TABLE; %[1]s is the source table name,
		// %[2]s is the FK-target identifier form (so the case_mismatch case
		// can declare the source as `actor` but REFERENCE `Actor`).
		ddl        string
		fkTargetFn func(srcName string) string // produces the REFERENCES target as it appears in ddl.
	}{
		{
			name:       "column_constraint_unquoted",
			ddl:        `CREATE TABLE %[1]s (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES %[2]s(id))`,
			fkTargetFn: func(s string) string { return s },
		},
		{
			name:       "column_constraint_quoted",
			ddl:        `CREATE TABLE "%[1]s" (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES "%[2]s"(id))`,
			fkTargetFn: func(s string) string { return s },
		},
		{
			name: "table_constraint",
			ddl: `CREATE TABLE %[1]s (id INTEGER PRIMARY KEY, parent_id INTEGER, ` +
				`FOREIGN KEY(parent_id) REFERENCES %[2]s(id))`,
			fkTargetFn: func(s string) string { return s },
		},
		{
			// Source is `actor`, FK target is `Actor`; the driver predicate
			// matches them case-insensitively per SQLite's identifier rules.
			name:       "case_mismatch_fk_target",
			ddl:        `CREATE TABLE %[1]s (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES %[2]s(id))`,
			fkTargetFn: func(s string) string { return strings.ToUpper(s[:1]) + s[1:] },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			th, db, drvr := openSqliteForFKTest(t)

			const srcName, dstName = "actor", "actor_bak"
			_, err := db.ExecContext(th.Context,
				fmt.Sprintf(tc.ddl, srcName, tc.fkTargetFn(srcName)))
			require.NoError(t, err)
			// Seed the source with id=1 so we can verify the destination FK
			// does NOT resolve to it after the copy.
			_, err = db.ExecContext(th.Context,
				fmt.Sprintf(`INSERT INTO %q (id) VALUES (1)`, srcName))
			require.NoError(t, err)

			_, err = drvr.CopyTable(th.Context, db,
				tablefq.T{Table: srcName}, tablefq.T{Table: dstName}, false)
			require.NoError(t, err)

			var destDDL string
			require.NoError(t, db.QueryRowContext(th.Context,
				`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, dstName).Scan(&destDDL))
			assertSelfFKRewritten(t, destDDL, srcName, dstName)

			// Runtime check: inserting a row whose parent_id points at id=1
			// must fail, because id=1 lives only in the source. If the FK
			// still pointed at the source (the gh759 bug), the insert would
			// succeed.
			_, err = db.ExecContext(th.Context,
				fmt.Sprintf(`INSERT INTO %q (id, parent_id) VALUES (10, 1)`, dstName))
			require.Error(t, err,
				"FK must resolve to the destination; an id present only in the source must not satisfy it")

			// Sanity: a self-referential insert against the destination's own
			// row resolves cleanly.
			_, err = db.ExecContext(th.Context,
				fmt.Sprintf(`INSERT INTO %q (id) VALUES (2)`, dstName))
			require.NoError(t, err)
			_, err = db.ExecContext(th.Context,
				fmt.Sprintf(`INSERT INTO %q (id, parent_id) VALUES (3, 2)`, dstName))
			require.NoError(t, err)
		})
	}
}

// TestDriveri_CopyTable_LeavesCrossFKsAlone verifies the load-bearing
// invariant of gh759: REFERENCES whose target is NOT the source table
// must be left untouched. A regression that inverted the predicate
// would silently re-point cross-table FKs at the destination, a worse
// bug than the original.
func TestDriveri_CopyTable_LeavesCrossFKsAlone(t *testing.T) {
	th, db, drvr := openSqliteForFKTest(t)

	_, err := db.ExecContext(th.Context,
		`CREATE TABLE parent (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context,
		`CREATE TABLE actor (`+
			`id INTEGER PRIMARY KEY, `+
			`parent_id INTEGER REFERENCES actor(id), `+
			`other_id INTEGER REFERENCES parent(id))`)
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: "actor"}, tablefq.T{Table: "actor_bak"}, false)
	require.NoError(t, err)

	var destDDL string
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, "actor_bak").Scan(&destDDL))

	require.True(t,
		strings.Contains(destDDL, `REFERENCES actor_bak(`) ||
			strings.Contains(destDDL, `REFERENCES "actor_bak"(`),
		"self-FK must be rewritten to actor_bak, got: %s", destDDL)
	require.True(t,
		strings.Contains(destDDL, `REFERENCES parent(`) ||
			strings.Contains(destDDL, `REFERENCES "parent"(`),
		"cross-table FK to parent must be preserved, got: %s", destDDL)
}

// TestDriveri_CopyTable_MultipleSelfFKs verifies every self-FK in a
// table with more than one REFERENCES <src>(...) is rewritten; guards
// against a regression that returned after the first match.
func TestDriveri_CopyTable_MultipleSelfFKs(t *testing.T) {
	th, db, drvr := openSqliteForFKTest(t)

	_, err := db.ExecContext(th.Context,
		`CREATE TABLE actor (`+
			`id INTEGER PRIMARY KEY, `+
			`parent_id INTEGER REFERENCES actor(id), `+
			`buddy_id INTEGER REFERENCES actor(id))`)
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: "actor"}, tablefq.T{Table: "actor_bak"}, false)
	require.NoError(t, err)

	var destDDL string
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, "actor_bak").Scan(&destDDL))

	require.Equal(t, 2,
		strings.Count(destDDL, `REFERENCES actor_bak(`)+
			strings.Count(destDDL, `REFERENCES "actor_bak"(`),
		"both self-FKs must point at actor_bak, got: %s", destDDL)
	require.False(t,
		strings.Contains(destDDL, `REFERENCES actor(`) ||
			strings.Contains(destDDL, `REFERENCES "actor"(`),
		"no self-FK may still point at the source, got: %s", destDDL)
}

// TestDriveri_CopyTable_CompositeSelfFK verifies composite-column
// FOREIGN KEY(a, b) REFERENCES self(x, y) is rewritten to point at the
// destination, with the column lists left intact.
func TestDriveri_CopyTable_CompositeSelfFK(t *testing.T) {
	th, db, drvr := openSqliteForFKTest(t)

	_, err := db.ExecContext(th.Context,
		`CREATE TABLE link (`+
			`a INTEGER, b INTEGER, x INTEGER, y INTEGER, `+
			`PRIMARY KEY (x, y), `+
			`FOREIGN KEY(a, b) REFERENCES link(x, y))`)
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: "link"}, tablefq.T{Table: "link_bak"}, false)
	require.NoError(t, err)

	var destDDL string
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, "link_bak").Scan(&destDDL))

	require.True(t,
		strings.Contains(destDDL, `REFERENCES link_bak(x, y)`) ||
			strings.Contains(destDDL, `REFERENCES "link_bak"(x, y)`),
		"composite FK must be rewritten to link_bak with parent columns preserved, got: %s",
		destDDL)
	require.False(t, strings.Contains(destDDL, `REFERENCES link(`),
		"composite FK must no longer point at link, got: %s", destDDL)
}

// TestDriveri_CopyTable_SchemaQualifiedDest verifies that a destination
// with an explicit schema (e.g. "main"."actor_bak") rewrites the CREATE
// TABLE identifier with the dotted form but the self-FK REFERENCES
// target as a bare table token. SQLite's foreign_table grammar rule is
// a single any_name; emitting "schema"."tbl" as a REFERENCES target
// produces a syntax error from the runtime.
//
// The load-bearing catch is the first require.NoError on CopyTable: a
// regression that reused the schema-qualified destQuoted on the FK
// edits would surface as a CREATE TABLE syntax error before any of the
// downstream assertions run. The structural assertSelfFKRewritten and
// the NotContains "main"."actor_bak" assertion lock the post-fix form;
// the runtime FK enforcement at the end confirms the rewritten FK
// still binds.
func TestDriveri_CopyTable_SchemaQualifiedDest(t *testing.T) {
	th, db, drvr := openSqliteForFKTest(t)

	_, err := db.ExecContext(th.Context,
		`CREATE TABLE actor (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES actor(id))`)
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context, `INSERT INTO actor (id) VALUES (1)`)
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: "actor"},
		tablefq.T{Schema: "main", Table: "actor_bak"},
		false)
	require.NoError(t, err, "CopyTable to a schema-qualified destination must succeed")

	var destDDL string
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, "actor_bak").Scan(&destDDL))
	assertSelfFKRewritten(t, destDDL, "actor", "actor_bak")
	require.NotContains(t, destDDL, `REFERENCES "main"."actor_bak"`,
		"REFERENCES target must be unqualified per the SQLite grammar, got: %s", destDDL)

	// Runtime: FK still enforces against the destination.
	_, err = db.ExecContext(th.Context,
		`INSERT INTO actor_bak (id, parent_id) VALUES (10, 1)`)
	require.Error(t, err, "FK must enforce against the destination row set")
}

// TestDriveri_CopyTable_PreservesOnDeleteCascade is the runtime sibling
// of the sqlparser-level PreservesActionClauses test: verifies that a
// self FK with ON DELETE CASCADE round-trips textually through the
// rewrite and still fires against the destination table at runtime.
// Catches a regression class where the rewrite mangles the action clause
// in some way that survives the parser's eyes but breaks SQLite's.
func TestDriveri_CopyTable_PreservesOnDeleteCascade(t *testing.T) {
	th, db, drvr := openSqliteForFKTest(t)

	_, err := db.ExecContext(th.Context,
		`CREATE TABLE actor (`+
			`id INTEGER PRIMARY KEY, `+
			`parent_id INTEGER REFERENCES actor(id) ON DELETE CASCADE)`)
	require.NoError(t, err)

	_, err = drvr.CopyTable(th.Context, db,
		tablefq.T{Table: "actor"}, tablefq.T{Table: "actor_bak"}, false)
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context, `INSERT INTO actor_bak (id) VALUES (1)`)
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context,
		`INSERT INTO actor_bak (id, parent_id) VALUES (2, 1)`)
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context,
		`INSERT INTO actor_bak (id, parent_id) VALUES (3, 2)`)
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context, `DELETE FROM actor_bak WHERE id=1`)
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRowContext(th.Context,
		`SELECT COUNT(*) FROM actor_bak`).Scan(&count))
	require.Equal(t, 0, count,
		"ON DELETE CASCADE must fire on the destination: deleting id=1 should cascade to id=2 and id=3")
}

// TestBusyTimeoutDefault verifies gh699: every sqlite3 connection gets a
// default busy_timeout so that concurrent access waits for locks instead
// of immediately failing with SQLITE_BUSY ("database is locked"). A
// user-supplied _busy_timeout conn param must still win.
func TestBusyTimeoutDefault(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		connStr string
		want    int
	}{
		{name: "default", connStr: "", want: 5000},
		{name: "user_override", connStr: "?_busy_timeout=1234", want: 1234},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			dbPath := tu.TempFile(t, "gh699.db")

			loc, err := sqlite3.MungeLocation("sqlite3://" + filepath.ToSlash(dbPath) + tc.connStr)
			require.NoError(t, err)

			src := &source.Source{
				Handle:   "@gh699_" + tc.name,
				Type:     drivertype.SQLite,
				Location: loc,
			}

			grip := th.Open(src)
			db, err := grip.DB(th.Context)
			require.NoError(t, err)

			var got int
			require.NoError(t, db.QueryRowContext(th.Context, "PRAGMA busy_timeout").Scan(&got))
			require.Equal(t, tc.want, got)
		})
	}
}

// TestSourceMetadata_LocationWithConnParams reproduces gh720: a
// source-level metadata read must succeed when the source location
// carries a connection-string suffix (e.g. ?mode=ro). The previous
// behavior was os.Stat returning "no such file or directory" because
// PathFromLocation passed through the literal "path?mode=ro" string.
func TestSourceMetadata_LocationWithConnParams(t *testing.T) {
	t.Parallel()

	th := testh.New(t)

	dbPath := tu.TempFile(t, "gh720.db")

	// First, create a tiny db with no conn params.
	bootSrc := &source.Source{
		Handle:   "@bootstrap",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + filepath.ToSlash(dbPath),
	}
	bootGrip := th.Open(bootSrc)
	bootDB, err := bootGrip.DB(th.Context)
	require.NoError(t, err)
	_, err = bootDB.ExecContext(th.Context, `CREATE TABLE t (id INTEGER); INSERT INTO t VALUES (1);`)
	require.NoError(t, err)
	require.NoError(t, bootGrip.Close())

	// Now open with ?mode=ro and read source metadata — this used to fail.
	roLoc, err := sqlite3.MungeLocation("sqlite3://" + filepath.ToSlash(dbPath) + "?mode=ro")
	require.NoError(t, err)

	roSrc := &source.Source{
		Handle:   "@ro",
		Type:     drivertype.SQLite,
		Location: roLoc,
	}
	roGrip := th.Open(roSrc)

	md, err := roGrip.SourceMetadata(th.Context, true)
	require.NoError(t, err, "SourceMetadata must succeed even when Location carries ?mode=ro")
	require.Equal(t, "@ro", md.Handle)
	require.NotNil(t, md.Size, "file size should be non-nil")
	require.NotZero(t, *md.Size, "file size should be non-zero")
	require.Equal(t, filepath.Base(dbPath), md.Name)
}

// TestNewScratchSource_SecretsResolved verifies that the scratch
// source is marked SecretsResolved: its Location is a literal file
// path constructed internally (never a placeholder template), so the
// connect path's '$$' unescape must not reinterpret it. Without the
// marker, a scratch path containing a literal '$$' (e.g. a cache dir
// under a directory named with '$$') would be silently rewritten.
func TestNewScratchSource_SecretsResolved(t *testing.T) {
	ctx := testh.New(t).Context
	fpath := filepath.Join(t.TempDir(), "scratch$$db.sqlite")

	src, clnup, err := sqlite3.NewScratchSource(ctx, fpath)
	require.NoError(t, err)
	// The scratch DB file is only created on first open, which this
	// test never does, so clnup's file removal may error: ignore it.
	t.Cleanup(func() { _ = clnup() })

	require.True(t, src.SecretsResolved,
		"internally constructed literal locations must be marked resolved")

	resolved, err := driver.ResolveSourceSecrets(ctx, src)
	require.NoError(t, err)
	require.Equal(t, src.Location, resolved.Location,
		"resolution must not alter the literal scratch path")
}

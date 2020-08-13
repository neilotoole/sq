package sqlite3_test

import (
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/sqlmodel"
	"github.com/neilotoole/sq/libsq/sqlz"
	"github.com/neilotoole/sq/libsq/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestSmoke(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.SL3)

	sink, err := th.QuerySQL(src, "SELECT * FROM "+sakila.TblFilm)
	require.NoError(t, err)
	require.Equal(t, sakila.TblFilmCount, len(sink.Recs))
}

func TestQueryEmptyTable(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.SL3)

	// Get an empty table by copying an existing one
	tblName := th.CopyTable(true, src, sakila.TblFilm, "", false)
	require.Equal(t, int64(0), th.RowCount(src, tblName))

	sink, err := th.QuerySQL(src, "SELECT * FROM "+tblName)
	require.NoError(t, err)
	require.Equal(t, 0, len(sink.Recs))
}

// TestExhibitDriverColumnTypesBehavior shows the unusual
// behavior of SQLite wrt column types. The following is observed:
//
// 1. If rows.ColumnTypes is invoked prior to rows.Next being
//    invoked, the column ScanType will be nil.
// 2. The values returned by rows.ColumnTypes can change after
//    each call to rows.Next. This is because of SQLite's dynamic
//    typing: any value can be stored in any column.
//
// The second fact is potentially problematic for sq, as sq expects
// that the values of a column are all of the same type. Thus, sq
// will likely encounter problems dealing with SQLite tables
// that have mixed data types in columns.
func TestExhibitDriverColumnTypesBehavior(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.SL3)
	db := th.Open(src).DB()
	t.Log("using source: " + src.Location)

	tblName := stringz.UniqTableName("scan_test")
	createStmt := "CREATE TABLE " + tblName + " (col1 REAL)"
	insertStmt := "INSERT INTO " + tblName + " VALUES(?)"
	query := "SELECT * FROM " + tblName

	// Create the table
	th.ExecSQL(src, createStmt)
	t.Cleanup(func() { th.DropTable(src, tblName) })

	// 1. Demonstrate that ColumnType.ScanType returns nil
	//    before rows.Next is invoked
	rows1, err := db.Query(query)
	require.NoError(t, err)
	defer rows1.Close()

	colTypes, err := rows1.ColumnTypes()
	require.NoError(t, err)
	require.Equal(t, colTypes[0].Name(), "col1")
	require.Nil(t, colTypes[0].ScanType(), "scan type is nil because rows.Next was not invoked")

	require.False(t, rows1.Next()) // no rows yet since table is empty
	colTypes, err = rows1.ColumnTypes()
	require.Error(t, err, "ColumnTypes returns an error because the Next call closed rows")
	require.Nil(t, colTypes)

	// 2. Demonstrate that a column's scan type can be different for
	//    each row (due to sqlite's dynamic typing)

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
	require.Nil(t, colTypes[0].ScanType(), "scan type should be nil because rows.Next was not invoked")

	// 1st data row
	require.True(t, rows2.Next())
	colTypes, err = rows2.ColumnTypes()
	require.NoError(t, err)
	scanType := colTypes[0].ScanType()
	require.Nil(t, scanType, "scan type be nil because the value is nil")

	// 2nd data row
	require.True(t, rows2.Next())
	colTypes, err = rows2.ColumnTypes()
	require.NoError(t, err)
	scanType = colTypes[0].ScanType()
	require.NotNil(t, scanType, "scan type should be non-nil because the value is not nil")
	require.Equal(t, sqlz.RTypeFloat64.String(), scanType.String())

	// 3nd data row
	require.True(t, rows2.Next())
	colTypes, err = rows2.ColumnTypes()
	require.NoError(t, err)
	scanType = colTypes[0].ScanType()
	require.NotNil(t, scanType, "scan type should be non-nil because the value is not nil")
	require.Equal(t, sqlz.RTypeString.String(), scanType.String())

	require.False(t, rows2.Next(), "should be end of rows")
	require.Nil(t, rows2.Err())
}

func TestDriver_CreateTable_NotNullDefault(t *testing.T) {
	t.Parallel()

	th, src, dbase, drvr := testh.NewWith(t, sakila.SL3)

	tblName := stringz.UniqTableName(t.Name())
	colNames, colKinds := fixt.ColNamePerKind(drvr.Dialect().IntBool, false, false)

	tblDef := sqlmodel.NewTableDef(tblName, colNames, colKinds)
	for _, colDef := range tblDef.Cols {
		colDef.NotNull = true
		colDef.HasDefault = true
	}

	err := drvr.CreateTable(th.Context, dbase.DB(), tblDef)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tblName) })

	th.InsertDefaultRow(src, tblName)

	sink, err := th.QuerySQL(src, "SELECT * FROM "+tblName)
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
		{loc: `sqlite3://C:\dir\sakila.db`, want: `C:\dir\sakila.db`},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.loc, func(t *testing.T) {
			src := &source.Source{
				Handle:   "@h1",
				Type:     sqlite3.Type,
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

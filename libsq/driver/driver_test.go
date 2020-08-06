package driver_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/drivers/xlsx"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/sqlmodel"
	"github.com/neilotoole/sq/libsq/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestDriver_DropTable(t *testing.T) {
	for _, handle := range sakila.SQLAll {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(handle)
			targetTable := stringz.UniqSuffix(sakila.TblActor)

			drvr, err := th.Registry().DriverFor(src.Type)
			require.NoError(t, err)
			sqlDrvr := drvr.(driver.SQLDriver)
			db := th.Open(src).DB()

			// Copy a table that we can play with
			th.CopyTable(false, src, sakila.TblActor, targetTable, false)
			require.NoError(t, sqlDrvr.DropTable(th.Context, db, targetTable, true))

			// Copy the table again so we can drop it again
			th.CopyTable(false, src, sakila.TblActor, targetTable, false)

			// test with ifExists = false
			require.NoError(t, sqlDrvr.DropTable(th.Context, db, targetTable, false))

			// Check that we get the expected behavior when the table doesn't exist
			require.NoError(t, sqlDrvr.DropTable(th.Context, db, stringz.UniqSuffix("not_a_table"), true), "should be no error when ifExists is true")
			require.Error(t, sqlDrvr.DropTable(th.Context, db, stringz.UniqSuffix("not_a_table"), false), "error expected when ifExists is false")
		})
	}
}

func TestDriver_CopyTable(t *testing.T) {
	for _, handle := range sakila.SQLAll {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			// t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)

			drvr, err := th.Registry().DriverFor(src.Type)
			require.NoError(t, err)
			sqlDrvr := drvr.(driver.SQLDriver)
			db := th.Open(src).DB()
			require.EqualValues(t, sakila.TblActorCount, th.RowCount(src, sakila.TblActor), "fromTable should have ActorCount rows beforehand")

			toTable := stringz.UniqSuffix(sakila.TblActor)
			// First, test with copyData = true
			copied, err := sqlDrvr.CopyTable(th.Context, db, sakila.TblActor, toTable, true)
			require.NoError(t, err)
			require.Equal(t, int64(sakila.TblActorCount), copied)
			require.EqualValues(t, sakila.TblActorCount, th.RowCount(src, toTable))
			th.DropTable(src, toTable)

			// Then, with copyData = false
			copied, err = sqlDrvr.CopyTable(th.Context, db, sakila.TblActor, toTable, false)
			require.NoError(t, err)
			require.Equal(t, int64(0), copied)
			require.EqualValues(t, 0, th.RowCount(src, toTable))
			th.DropTable(src, toTable)
		})
	}
}

// TestDriver_CreateTable_Minimal tests the minimal functionality
// of SQLDriver.CreateTable impls.
func TestDriver_CreateTable_Minimal(t *testing.T) {
	t.Parallel()

	testCases := sakila.SQLAll
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, dbase, drvr := testh.NewWith(t, handle)

			tblName := stringz.UniqTableName(t.Name())
			colNames, colKinds := fixt.ColNamePerKind(drvr.Dialect().IntBool, false, false)
			tblDef := sqlmodel.NewTableDef(tblName, colNames, colKinds)

			err := drvr.CreateTable(th.Context, dbase.DB(), tblDef)
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tblName) })

			colTypes, err := drvr.TableColumnTypes(th.Context, dbase.DB(), tblName, colNames)
			require.NoError(t, err)
			require.Equal(t, len(colNames), len(colTypes))

			recMeta, _, err := drvr.RecordMeta(colTypes)
			require.NoError(t, err)

			require.Equal(t, colNames, recMeta.Names())
			require.Equal(t, colKinds, recMeta.Kinds())
		})
	}
}

func TestDriver_TableColumnTypes(t *testing.T) {
	t.Parallel()

	testCases := sakila.SQLAll
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			testh.SkipShort(t, handle == sakila.XLSX)
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			dbase := th.Open(src)
			drvr, db := dbase.SQLDriver(), dbase.DB()

			// Run the test both with and without data in the target table.
			// Some driver implementations of rows.ColumnTypes behave
			// differently depending upon whether the query returns rows
			// or not.
			for _, copyData := range []bool{false, true} {
				actualTblName := th.CopyTable(false, src, sakila.TblActor, "", copyData)
				t.Cleanup(func() { th.DropTable(src, actualTblName) })

				// Note nil colNames, should still get all columns
				// as if the query was (SELECT * FROM actualTblName)
				colTypes, err := drvr.TableColumnTypes(th.Context, db, actualTblName, nil)
				require.NoError(t, err)
				require.Equal(t, len(sakila.TblActorCols), len(colTypes))
				for i := range colTypes {
					require.Equal(t, sakila.TblActorCols[i], colTypes[i].Name())
				}

				// Try again, but requesting specific col names
				wantColNames := []string{sakila.TblActorCols[0], sakila.TblActorCols[2]}
				colTypes, err = drvr.TableColumnTypes(th.Context, db, actualTblName, wantColNames)
				require.NoError(t, err)
				require.Equal(t, len(wantColNames), len(colTypes))
				for i := range colTypes {
					require.Equal(t, wantColNames[i], colTypes[i].Name())
				}
			}
		})
	}
}

func TestSQLDriver_PrepareUpdateStmt(t *testing.T) {
	t.Parallel()
	testCases := sakila.SQLAll

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			testh.SkipShort(t, handle == sakila.XLSX)
			t.Parallel()

			th, src, dbase, drvr := testh.NewWith(t, handle)

			actualTblName := th.CopyTable(false, src, sakila.TblActor, "", true)
			t.Cleanup(func() { th.DropTable(src, actualTblName) })

			const (
				actorID     int64  = 1
				whereClause string = "actor_id = ?"
			)
			var (
				destCols = []string{"first_name", "last_name"}
				wantVals = []interface{}{"Kubla", "Khan"}
				args     = append(wantVals, actorID)
			)

			stmtExecer, err := drvr.PrepareUpdateStmt(th.Context, dbase.DB(), actualTblName, destCols, whereClause)
			require.NoError(t, err)
			require.Equal(t, destCols, stmtExecer.DestMeta().Names())
			require.NoError(t, stmtExecer.Munge(wantVals))

			affected, err := stmtExecer.Exec(th.Context, args...)
			require.NoError(t, err)
			assert.Equal(t, int64(1), affected)

			sink, err := th.QuerySQL(src, "SELECT * FROM "+actualTblName+" WHERE actor_id = 1")

			require.NoError(t, err)
			require.Equal(t, 1, len(sink.Recs))
			require.Equal(t, actorID, testh.Val(sink.Recs[0][0]))
			require.Equal(t, wantVals[0], testh.Val(sink.Recs[0][1]))
			require.Equal(t, wantVals[1], testh.Val(sink.Recs[0][2]))
		})
	}
}

func TestDriver_Ping(t *testing.T) {
	t.Parallel()
	testCases := append(sakila.All, sakila.CSVActor, sakila.CSVActorHTTP)

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			testh.SkipShort(t, handle == sakila.XLSX)
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			drvr := th.DriverFor(src)

			err := drvr.Ping(th.Context, src)
			require.NoError(t, err)
		})
	}
}

func TestDriver_Open(t *testing.T) {
	t.Parallel()
	testCases := append(sakila.All, sakila.CSVActor, sakila.CSVActorHTTP)

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			testh.SkipShort(t, handle == sakila.XLSX)
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			drvr := th.DriverFor(src)

			dbase, err := drvr.Open(th.Context, src)
			require.NoError(t, err)
			require.NoError(t, dbase.DB().PingContext(th.Context))
			require.NoError(t, dbase.Close())
		})
	}
}

// coreDrivers is a slice of the core driver types.
var coreDrivers = []source.Type{
	postgres.Type,
	sqlserver.Type,
	sqlite3.Type,
	mysql.Type,
	csv.TypeCSV,
	csv.TypeTSV,
	xlsx.Type,
}

// sqlDrivers is a slice of the SQL driver types.
var sqlDrivers = []source.Type{
	postgres.Type,
	sqlserver.Type,
	sqlite3.Type,
	mysql.Type,
}

// docDrivers is a slice of the doc driver types.
var docDrivers = []source.Type{
	csv.TypeCSV,
	csv.TypeTSV,
	xlsx.Type,
}

func TestRegistry_DriversMetadata_All(t *testing.T) {
	t.Parallel()

	reg := testh.New(t).Registry()
	metas := reg.DriversMetadata()
	require.Equal(t, len(metas), len(reg.Drivers()))

	m := map[source.Type]driver.Metadata{}
	for i := range metas {
		m[metas[i].Type] = metas[i]
	}

	for _, typ := range coreDrivers {
		md, ok := m[typ]
		require.True(t, ok)
		require.Equal(t, typ, md.Type)
	}
}

func TestRegistry_DriversMetadata_SQL(t *testing.T) {
	t.Parallel()

	for _, typ := range sqlDrivers {
		typ := typ

		t.Run(typ.String(), func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			reg := th.Registry()

			drvr, err := reg.DriverFor(typ)
			require.NoError(t, err)

			meta := drvr.DriverMetadata()
			require.Equal(t, typ, meta.Type)
			require.True(t, meta.IsSQL)
			require.False(t, meta.Monotable, "can't be monotable and SQL")

			sqlDrvr, ok := drvr.(driver.SQLDriver)
			require.True(t, ok)

			dialect := sqlDrvr.Dialect()
			require.Equal(t, typ, dialect.Type)
			require.NotEmpty(t, dialect.Quote)
			require.NotNil(t, dialect.Placeholders)
		})
	}
}

func TestRegistry_DriversMetadata_Doc(t *testing.T) {
	t.Parallel()

	for _, typ := range docDrivers {
		typ := typ

		t.Run(typ.String(), func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			reg := th.Registry()

			drvr, err := reg.DriverFor(typ)
			require.NoError(t, err)

			_, ok := drvr.(driver.SQLDriver)
			require.False(t, ok)

			meta := drvr.DriverMetadata()
			require.Equal(t, typ, meta.Type)
			require.False(t, meta.IsSQL)
		})
	}
}

func TestDatabase_TableMetadata(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.SQLAll {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, dbase, _ := testh.NewWith(t, handle)

			tblMeta, err := dbase.TableMetadata(th.Context, sakila.TblActor)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActor, tblMeta.Name)
		})
	}
}

func TestDatabase_SourceMetadata(t *testing.T) {
	t.Parallel()

	for _, handle := range []string{sakila.Pg9} {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, dbase, _ := testh.NewWith(t, handle)

			md, err := dbase.SourceMetadata(th.Context)
			require.NoError(t, err)
			require.Equal(t, "sakila", md.Name)
		})
	}
}

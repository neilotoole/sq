package driver_test

import (
	"github.com/neilotoole/sq/testh/tutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq/core/kind"

	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestDriver_DropTable(t *testing.T) {
	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, dbase, drvr := testh.NewWith(t, handle)
			db := dbase.DB()

			tblName := stringz.UniqTableName(sakila.TblActor)

			// Copy a table that we can play with
			tblName = th.CopyTable(false, src, sakila.TblActor, tblName, false)
			require.NoError(t, drvr.DropTable(th.Context, db, tblName, true))

			// Copy the table again so we can drop it again
			tblName = th.CopyTable(false, src, sakila.TblActor, tblName, false)

			// test with ifExists = false
			require.NoError(t, drvr.DropTable(th.Context, db, tblName, false))

			// Check that we get the expected behavior when the table doesn't exist
			require.NoError(t, drvr.DropTable(th.Context, db, stringz.UniqSuffix("not_a_table"), true), "should be no error when ifExists is true")
			require.Error(t, drvr.DropTable(th.Context, db, stringz.UniqSuffix("not_a_table"), false), "error expected when ifExists is false")
		})
	}
}
func TestDriver_TableExists(t *testing.T) {
	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			th, _, dbase, drvr := testh.NewWith(t, handle)
			db := dbase.DB()

			tblName := sakila.TblActor
			exists, err := drvr.TableExists(th.Context, db, tblName)
			require.NoError(t, err)
			require.True(t, exists)

			tblName = stringz.UniqTableName("no_exist")
			exists, err = drvr.TableExists(th.Context, db, tblName)
			require.NoError(t, err)
			require.False(t, exists)
		})
	}
}

func TestDriver_CopyTable(t *testing.T) {
	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, dbase, drvr := testh.NewWith(t, handle)
			db := dbase.DB()
			require.Equal(t, int64(sakila.TblActorCount), th.RowCount(src, sakila.TblActor), "fromTable should have ActorCount rows beforehand")

			toTable := stringz.UniqTableName(sakila.TblActor)
			// First, test with copyData = true
			copied, err := drvr.CopyTable(th.Context, db, sakila.TblActor, toTable, true)
			require.NoError(t, err)
			require.Equal(t, int64(sakila.TblActorCount), copied)
			require.Equal(t, int64(sakila.TblActorCount), th.RowCount(src, toTable))
			defer th.DropTable(src, toTable)

			toTable = stringz.UniqTableName(sakila.TblActor)
			// Then, with copyData = false
			copied, err = drvr.CopyTable(th.Context, db, sakila.TblActor, toTable, false)
			require.NoError(t, err)
			require.Equal(t, int64(0), copied)
			require.Equal(t, int64(0), th.RowCount(src, toTable))
			defer th.DropTable(src, toTable)
		})
	}
}

// TestDriver_CreateTable_Minimal tests the minimal functionality
// of SQLDriver.CreateTable impls.
func TestDriver_CreateTable_Minimal(t *testing.T) {
	t.Parallel()

	testCases := sakila.SQLAll()
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

	testCases := sakila.SQLAll()
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			tutil.SkipShort(t, handle == sakila.XLSX)
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
				tblName := th.CopyTable(true, src, sakila.TblActor, "", copyData)

				// Note nil colNames, should still get all columns
				// as if the query was (SELECT * FROM actualTblName)
				colTypes, err := drvr.TableColumnTypes(th.Context, db, tblName, nil)
				require.NoError(t, err)
				require.Equal(t, len(sakila.TblActorCols()), len(colTypes))
				for i := range colTypes {
					require.Equal(t, sakila.TblActorCols()[i], colTypes[i].Name())
				}

				// Try again, but requesting specific col names
				wantColNames := []string{sakila.TblActorCols()[0], sakila.TblActorCols()[2]}
				colTypes, err = drvr.TableColumnTypes(th.Context, db, tblName, wantColNames)
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
	testCases := sakila.SQLAll()

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			tutil.SkipShort(t, handle == sakila.XLSX)
			t.Parallel()

			th, src, dbase, drvr := testh.NewWith(t, handle)

			tblName := th.CopyTable(true, src, sakila.TblActor, "", true)

			const (
				actorID     int64  = 1
				whereClause string = "actor_id = ?"
			)
			var (
				destCols = []string{"first_name", "last_name"}
				wantVals = []any{"Kubla", "Khan"}
				args     = append(wantVals, actorID)
			)

			stmtExecer, err := drvr.PrepareUpdateStmt(th.Context, dbase.DB(), tblName, destCols, whereClause)
			require.NoError(t, err)
			require.Equal(t, destCols, stmtExecer.DestMeta().Names())
			require.NoError(t, stmtExecer.Munge(wantVals))

			affected, err := stmtExecer.Exec(th.Context, args...)
			require.NoError(t, err)
			assert.Equal(t, int64(1), affected)

			sink, err := th.QuerySQL(src, "SELECT * FROM "+tblName+" WHERE actor_id = 1")

			require.NoError(t, err)
			require.Equal(t, 1, len(sink.Recs))
			require.Equal(t, actorID, tutil.Val(sink.Recs[0][0]))
			require.Equal(t, wantVals[0], tutil.Val(sink.Recs[0][1]))
			require.Equal(t, wantVals[1], tutil.Val(sink.Recs[0][2]))
		})
	}
}

func TestDriver_Ping(t *testing.T) {
	testCases := sakila.AllHandles()
	testCases = append(testCases, sakila.CSVActor, sakila.CSVActorHTTP)

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			tutil.SkipShort(t, handle == sakila.XLSX)

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
	testCases := sakila.AllHandles()
	testCases = append(testCases, sakila.CSVActor, sakila.CSVActorHTTP)

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			tutil.SkipShort(t, handle == sakila.XLSX)
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

func TestNewBatchInsert(t *testing.T) {
	// This value is chosen as it's not a neat divisor of 200 (sakila.TblActorSize).
	const batchSize = 70

	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			th, src, _, drvr := testh.NewWith(t, handle)
			conn, err := th.Open(src).DB().Conn(th.Context)
			require.NoError(t, err)
			defer func() { assert.NoError(t, conn.Close()) }()

			tblName := th.CopyTable(true, src, sakila.TblActor, "", false)

			// Get records from TblActor that we'll write to the new tbl
			recMeta, recs := testh.RecordsFromTbl(t, handle, sakila.TblActor)
			bi, err := driver.NewBatchInsert(th.Context, th.Log, drvr, conn, tblName, recMeta.Names(), batchSize)
			require.NoError(t, err)

			for _, rec := range recs {
				err = bi.Munge(rec)
				require.NoError(t, err)

				select {
				case <-th.Context.Done():
					close(bi.RecordCh)
					// Should never happen
					t.Fatal(th.Context.Err())
				case err = <-bi.ErrCh:
					close(bi.RecordCh)
					// Should not happen
					t.Fatal(err)
				case bi.RecordCh <- rec:
				}
			}
			close(bi.RecordCh) // Indicates end of records

			err = <-bi.ErrCh
			require.Nil(t, err)

			sink, err := th.QuerySQL(src, "SELECT * FROM "+tblName)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
			th.TruncateTable(src, tblName) // cleanup
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

	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, dbase, _ := testh.NewWith(t, handle)

			tblMeta, err := dbase.TableMetadata(th.Context, sakila.TblActor)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActor, tblMeta.Name)
			require.Equal(t, int64(sakila.TblActorCount), tblMeta.RowCount)
		})
	}
}

func TestDatabase_SourceMetadata(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, dbase, _ := testh.NewWith(t, handle)

			md, err := dbase.SourceMetadata(th.Context)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActor, md.Tables[0].Name)
			require.Equal(t, int64(sakila.TblActorCount), md.Tables[0].RowCount)
		})
	}
}

func TestSQLDriver_AlterTableAddColumn(t *testing.T) {
	testCases := []string{sakila.SL3, sakila.Pg, sakila.MS}

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			th, src, dbase, drvr := testh.NewWith(t, handle)

			// Make a copy of the table to play with
			tbl := th.CopyTable(true, src, sakila.TblActor, "", true)

			const wantCol, wantKind = "col_int", kind.Int
			wantCols := append(sakila.TblActorCols(), wantCol)
			wantKinds := append(sakila.TblActorColKinds(), wantKind)

			err := drvr.AlterTableAddColumn(th.Context, dbase.DB(), tbl, wantCol, wantKind)
			require.NoError(t, err)

			sink, err := th.QuerySQL(src, "SELECT * FROM "+tbl)
			require.NoError(t, err)

			gotCols := sink.RecMeta.Names()
			require.Equal(t, wantCols, gotCols)

			gotKinds := sink.RecMeta.Kinds()
			require.Equal(t, wantKinds, gotKinds)
		})
	}
}

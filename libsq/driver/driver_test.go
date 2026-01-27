package driver_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

func TestDriver_DropTable(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, handle)

			tblName := stringz.UniqTableName(sakila.TblActor)

			// Copy a table that we can play with
			tblName = th.CopyTable(false, src, tablefq.From(sakila.TblActor), tablefq.From(tblName), false)
			require.NoError(t, drvr.DropTable(th.Context, db, tablefq.From(tblName), true))

			// Copy the table again so we can drop it again
			tblName = th.CopyTable(false, src, tablefq.From(sakila.TblActor), tablefq.From(tblName), false)

			// test with ifExists = false
			require.NoError(t, drvr.DropTable(th.Context, db, tablefq.From(tblName), false))

			// Check that we get the expected behavior when the table doesn't exist
			notTable := tablefq.New(stringz.UniqSuffix("not_a_table"))
			require.NoError(t, drvr.DropTable(th.Context, db, notTable, true),
				"should be no error when ifExists is true")
			require.Error(t, drvr.DropTable(th.Context, db, notTable, false),
				"error expected when ifExists is false")
		})
	}
}

func TestDriver_TableExists(t *testing.T) {
	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			th, _, drvr, _, db := testh.NewWith(t, handle)

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
	t.Parallel()

	for _, handle := range sakila.SQLAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, handle)
			require.Equal(t, int64(sakila.TblActorCount), th.RowCount(src, sakila.TblActor),
				"fromTable should have ActorCount rows beforehand")

			toTable := stringz.UniqTableName(sakila.TblActor)

			// Test 1: CopyTable with copyData = true
			// This should copy the table structure AND all data from the source table.
			copied, err := drvr.CopyTable(th.Context, db, tablefq.From(sakila.TblActor), tablefq.From(toTable), true)
			require.NoError(t, err)

			// Handle dialect.RowsAffectedUnsupported: Some drivers (e.g., ClickHouse)
			// cannot report row counts for INSERT ... SELECT operations due to
			// database protocol limitations. In that case, CopyTable returns -1
			// (dialect.RowsAffectedUnsupported) instead of the actual count.
			//
			// When this happens, we skip the assertion on the return value but still
			// verify the data was actually copied by checking the destination table's
			// row count directly. This ensures the test validates correctness even
			// when the driver can't report the count.
			if copied != dialect.RowsAffectedUnsupported {
				require.Equal(t, int64(sakila.TblActorCount), copied)
			} else {
				t.Logf("Driver does not support reporting rows affected; verifying via row count")
			}
			require.Equal(t, int64(sakila.TblActorCount), th.RowCount(src, toTable))
			defer th.DropTable(src, tablefq.From(toTable))

			toTable = stringz.UniqTableName(sakila.TblActor)

			// Test 2: CopyTable with copyData = false
			// This should copy only the table structure (schema), not the data.
			// The returned count should always be 0 since no data is copied.
			// Note: dialect.RowsAffectedUnsupported should NOT be returned here
			// because when copyData=false, the driver knows exactly 0 rows were copied.
			copied, err = drvr.CopyTable(th.Context, db, tablefq.From(sakila.TblActor), tablefq.From(toTable), false)
			require.NoError(t, err)
			require.Equal(t, int64(0), copied)
			require.Equal(t, int64(0), th.RowCount(src, toTable))
			defer th.DropTable(src, tablefq.From(toTable))
		})
	}
}

// TestDriver_CreateTable_Minimal tests the minimal functionality
// of SQLDriver.CreateTable impls.
func TestDriver_CreateTable_Minimal(t *testing.T) {
	t.Parallel()

	testCases := sakila.SQLAll()
	for _, handle := range testCases {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, handle)

			// Skip ClickHouse: this test verifies that kind.Kind values roundtrip
			// exactly through CreateTable -> TableColumnTypes -> RecordMeta.
			// Based on our current understanding, ClickHouse has type limitations
			// that prevent exact roundtrips:
			//   - kind.Time -> DateTime -> kind.Datetime (no time-only type)
			//   - kind.Bytes -> String -> kind.Text (binary stored as String)
			// See drivers/clickhouse/README.md "Known Limitations" for details.
			// This understanding may be incomplete or incorrect.
			tu.SkipIf(t, drvr.DriverMetadata().Type == drivertype.ClickHouse,
				"ClickHouse: kind.Time and kind.Bytes don't roundtrip exactly (see README Known Limitations)")

			tblName := stringz.UniqTableName(t.Name())
			colNames, colKinds := fixt.ColNamePerKind(drvr.Dialect().IntBool, false, false)
			tblDef := schema.NewTable(tblName, colNames, colKinds)

			err := drvr.CreateTable(th.Context, db, tblDef)
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

			colTypes, err := drvr.TableColumnTypes(th.Context, db, tblName, colNames)
			require.NoError(t, err)
			require.Equal(t, len(colNames), len(colTypes))

			recMeta, _, err := drvr.RecordMeta(th.Context, colTypes)
			require.NoError(t, err)

			require.Equal(t, colNames, recMeta.Names())
			require.Equal(t, colKinds, recMeta.Kinds())
		})
	}
}

func TestDriver_TableColumnTypes(t *testing.T) { //nolint:tparallel
	testCases := sakila.SQLAll()
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			tu.SkipShort(t, handle == sakila.XLSX)
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, handle)

			// Run the test both with and without data in the target table.
			// Some driver implementations of rows.ColumnTypes behave
			// differently depending upon whether the query returns rows
			// or not.
			for _, copyData := range []bool{false, true} {
				tblName := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, copyData)

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

func TestSQLDriver_PrepareUpdateStmt(t *testing.T) { //nolint:tparallel
	testCases := sakila.SQLAll()
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			tu.SkipShort(t, handle == sakila.XLSX)
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, handle)

			// ClickHouse Skip: The clickhouse-go driver's PrepareContext() only
			// supports INSERT and SELECT statements. ClickHouse uses
			// "ALTER TABLE ... UPDATE" syntax instead of standard UPDATE, but
			// PrepareContext() rejects this with "invalid INSERT query" because
			// the driver misclassifies non-SELECT statements as INSERTs.
			// Direct execution via ExecContext() works, but PrepareUpdateStmt
			// relies on PrepareContext(). See README.md "Development Log".
			tu.SkipIf(t, drvr.DriverMetadata().Type == drivertype.ClickHouse,
				"ClickHouse: PrepareContext doesn't support ALTER TABLE UPDATE")

			tblName := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)

			const (
				actorID     int64  = 1
				whereClause string = "actor_id = ?"
			)
			var (
				destCols = []string{"first_name", "last_name"}
				wantVals = []any{"Kubla", "Khan"}
				args     = append(wantVals, actorID)
			)

			stmtExecer, err := drvr.PrepareUpdateStmt(th.Context, db, tblName, destCols, whereClause)
			require.NoError(t, err)
			require.Equal(t, destCols, stmtExecer.DestMeta().Names())
			require.NoError(t, stmtExecer.Munge(wantVals))

			affected, err := stmtExecer.Exec(th.Context, args...)
			require.NoError(t, err)
			assert.Equal(t, int64(1), affected)

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+tblName+" WHERE actor_id = 1")

			require.NoError(t, err)
			require.Equal(t, 1, len(sink.Recs))
			require.Equal(t, actorID, stringz.Val(sink.Recs[0][0]))
			require.Equal(t, wantVals[0], stringz.Val(sink.Recs[0][1]))
			require.Equal(t, wantVals[1], stringz.Val(sink.Recs[0][2]))
		})
	}
}

func TestDriver_Ping(t *testing.T) {
	testCases := sakila.AllHandles()
	testCases = append(testCases, sakila.CSVActor, sakila.CSVActorHTTP)

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			tu.SkipShort(t, handle == sakila.XLSX)

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
			tu.SkipShort(t, handle == sakila.XLSX)
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			drvr := th.DriverFor(src)
			grip, err := drvr.Open(th.Context, src)
			require.NoError(t, err)
			db, err := grip.DB(th.Context)
			require.NoError(t, err)
			require.NoError(t, db.PingContext(th.Context))
			require.NoError(t, grip.Close())
		})
	}
}

func TestNewBatchInsert(t *testing.T) {
	// This value is chosen as it's not a neat divisor of 200 (sakila.TblActorSize).
	const batchSize = 70

	for _, handle := range sakila.SQLAll() {
		t.Run(handle, func(t *testing.T) {
			th, src, drvr, _, db := testh.NewWith(t, handle)

			// ClickHouse Skip: The clickhouse-go driver does not support multi-row
			// parameter binding. When sq generates "INSERT INTO t VALUES (?,?), (?,?)"
			// with flattened args, clickhouse-go expects args for a single row only.
			// Forcing single-row inserts (numRows=1) fixes that error, but causes
			// connection state corruption after many Exec calls on the same prepared
			// statement, resulting in "Unexpected packet Query received from client"
			// on subsequent queries. A proper fix requires using clickhouse-go's
			// native Batch API. See drivers/clickhouse/README.md "Development Log".
			tu.SkipIf(t, drvr.DriverMetadata().Type == drivertype.ClickHouse,
				"ClickHouse: clickhouse-go doesn't support multi-row parameter binding")

			tblName := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, false)
			conn, err := db.Conn(th.Context)
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = conn.Close()
			})

			// Get records from TblActor that we'll write to the new tbl
			recMeta, recs := testh.RecordsFromTbl(t, handle, sakila.TblActor)
			bi, err := driver.NewBatchInsert(
				th.Context,
				"Insert records",
				drvr,
				conn,
				tblName,
				recMeta.Names(),
				batchSize,
			)
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

			require.NoError(t, conn.Close())

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+tblName)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
			th.TruncateTable(src, tblName) // cleanup
		})
	}
}

// coreDrivers is a slice of the core driver types.
var coreDrivers = []drivertype.Type{
	drivertype.Pg,
	drivertype.MSSQL,
	drivertype.SQLite,
	drivertype.MySQL,
	drivertype.ClickHouse,
	drivertype.CSV,
	drivertype.TSV,
	drivertype.XLSX,
}

// sqlDrivers is a slice of the SQL driver types.
var sqlDrivers = []drivertype.Type{
	drivertype.Pg,
	drivertype.MSSQL,
	drivertype.SQLite,
	drivertype.MySQL,
	drivertype.ClickHouse,
}

// docDrivers is a slice of the doc driver types.
var docDrivers = []drivertype.Type{
	drivertype.CSV,
	drivertype.TSV,
	drivertype.XLSX,
}

func TestRegistry_DriversMetadata_All(t *testing.T) {
	t.Parallel()

	reg := testh.New(t).Registry()
	metas := reg.DriversMetadata()
	require.Equal(t, len(metas), len(reg.Drivers()))

	m := map[drivertype.Type]driver.Metadata{}
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

func TestGrip_TableMetadata(t *testing.T) { //nolint:tparallel
	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, _, grip, _ := testh.NewWith(t, handle)

			tblMeta, err := grip.TableMetadata(th.Context, sakila.TblActor)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActor, tblMeta.Name)
			require.Equal(t, int64(sakila.TblActorCount), tblMeta.RowCount)
		})
	}
}

func TestGrip_SourceMetadata(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, _, grip, _ := testh.NewWith(t, handle)

			md, err := grip.SourceMetadata(th.Context, false)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActor, md.Tables[0].Name)
			require.Equal(t, int64(sakila.TblActorCount), md.Tables[0].RowCount)
		})
	}
}

// TestSQLDriver_ListTableNames_ArgSchemaEmpty tests [driver.SQLDriver.ListTableNames]
// with an empty schema arg.
func TestSQLDriver_ListTableNames_ArgSchemaEmpty(t *testing.T) { //nolint:tparallel
	for _, handle := range sakila.SQLLatest() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, drvr, _, db := testh.NewWith(t, handle)

			got, err := drvr.ListTableNames(th.Context, db, "", false, false)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.True(t, len(got) == 0)

			got, err = drvr.ListTableNames(th.Context, db, "", true, false)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Contains(t, got, sakila.TblActor)
			require.NotContains(t, got, sakila.ViewFilmList)

			got, err = drvr.ListTableNames(th.Context, db, "", false, true)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.NotContains(t, got, sakila.TblActor)
			require.Contains(t, got, sakila.ViewFilmList)

			got, err = drvr.ListTableNames(th.Context, db, "", true, true)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Contains(t, got, sakila.TblActor)
			require.Contains(t, got, sakila.ViewFilmList)

			gotCopy := append([]string(nil), got...)
			slices.Sort(gotCopy)
			require.Equal(t, got, gotCopy, "expected results to be sorted")
		})
	}
}

// TestSQLDriver_ListTableNames_ArgSchemaNotEmpty tests
// [driver.SQLDriver.ListTableNames] with a non-empty schema arg.
func TestSQLDriver_ListTableNames_ArgSchemaNotEmpty(t *testing.T) { //nolint:tparallel
	testCases := []struct {
		handle     string
		schema     string
		wantTables int
		wantViews  int
	}{
		{handle: sakila.Pg12, schema: "public", wantTables: 21, wantViews: 7},
		{handle: sakila.MS19, schema: "dbo", wantTables: 16, wantViews: 5},
		{handle: sakila.SL3, schema: "main", wantTables: 16, wantViews: 5},
		{handle: sakila.My8, schema: "sakila", wantTables: 16, wantViews: 7},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.handle, func(t *testing.T) {
			t.Parallel()

			th, _, drvr, _, db := testh.NewWith(t, tc.handle)

			got, err := drvr.ListTableNames(th.Context, db, tc.schema, false, false)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.True(t, len(got) == 0)

			got, err = drvr.ListTableNames(th.Context, db, tc.schema, true, false)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got, tc.wantTables)

			got, err = drvr.ListTableNames(th.Context, db, tc.schema, false, true)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got, tc.wantViews)

			got, err = drvr.ListTableNames(th.Context, db, tc.schema, true, true)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got, tc.wantTables+tc.wantViews)

			gotCopy := append([]string(nil), got...)
			slices.Sort(gotCopy)
			require.Equal(t, got, gotCopy, "expected results to be sorted")
		})
	}
}

// TestGrip_SourceMetadata_concurrent tests the behavior of the
// drivers when SourceMetadata is invoked concurrently.
func TestGrip_SourceMetadata_concurrent(t *testing.T) { //nolint:tparallel
	const concurrency = 5

	handles := sakila.SQLLatest()
	for _, handle := range handles {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, _, grip, _ := testh.NewWith(t, handle)
			g, gCtx := errgroup.WithContext(th.Context)
			for i := 0; i < concurrency; i++ {
				g.Go(func() error {
					md, err := grip.SourceMetadata(gCtx, false)
					require.NoError(t, err)
					require.NotNil(t, md)
					gotTbl := md.Table(sakila.TblActor)
					require.NotNil(t, gotTbl)
					require.Equal(t, int64(sakila.TblActorCount), gotTbl.RowCount)
					return nil
				})
			}

			require.NoError(t, g.Wait())
		})
	}
}

func TestSQLDriver_AlterTableAddColumn(t *testing.T) {
	testCases := sakila.SQLLatest()

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			th, src, drvr, _, db := testh.NewWith(t, handle)

			// Make a copy of the table to play with
			tbl := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)

			const wantCol, wantKind = "col_int", kind.Int
			wantCols := append(sakila.TblActorCols(), wantCol)
			wantKinds := append(sakila.TblActorColKinds(), wantKind)

			err := drvr.AlterTableAddColumn(th.Context, db, tbl, wantCol, wantKind)
			require.NoError(t, err)

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+tbl)
			require.NoError(t, err)

			gotCols := sink.RecMeta.Names()
			require.Equal(t, wantCols, gotCols)

			gotKinds := sink.RecMeta.Kinds()
			require.Equal(t, wantKinds, gotKinds)
		})
	}
}

func TestSQLDriver_AlterTableRename(t *testing.T) {
	testCases := sakila.SQLLatest()

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			th, src, drvr, grip, db := testh.NewWith(t, handle)

			// Make a copy of the table to play with
			tbl := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)
			defer th.DropTable(src, tablefq.From(tbl))

			newName := stringz.UniqSuffix("actor_copy_")
			err := drvr.AlterTableRename(th.Context, db, tbl, newName)
			require.NoError(t, err)
			defer th.DropTable(src, tablefq.From(newName))

			md, err := grip.TableMetadata(th.Context, newName)
			require.NoError(t, err)
			require.Equal(t, newName, md.Name)
			sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+newName)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

func TestSQLDriver_AlterTableRenameColumn(t *testing.T) {
	testCases := sakila.SQLLatest()

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			th, src, drvr, grip, db := testh.NewWith(t, handle)

			// Make a copy of the table to play with
			tbl := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)

			newName := "given_name"
			err := drvr.AlterTableRenameColumn(th.Context, db, tbl, "first_name", newName)
			require.NoError(t, err)

			md, err := grip.TableMetadata(th.Context, tbl)
			require.NoError(t, err)
			require.NotNil(t, md.Column(newName))
			sink, err := th.QuerySQL(src, nil, fmt.Sprintf("SELECT %s FROM %s", newName, tbl))
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

func TestSQLDriver_CurrentCatalog(t *testing.T) {
	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			th, _, drvr, _, db := testh.NewWith(t, handle)
			if !drvr.Dialect().Catalog {
				t.Skipf("driver {%s} does not support catalogs", drvr.DriverMetadata().Type)
				return
			}

			currentCatalog, err := drvr.CurrentCatalog(th.Context, db)
			require.NoError(t, err)
			require.NotEmpty(t, currentCatalog)

			gotCatalogs, err := drvr.ListCatalogs(th.Context, db)
			require.NoError(t, err)
			require.Equal(t, currentCatalog, gotCatalogs[0])
		})
	}
}

func TestSQLDriver_CurrentSchemaCatalog(t *testing.T) {
	testCases := []struct {
		handle      string
		wantSchema  string
		wantCatalog string
	}{
		{sakila.SL3, "main", "default"},
		{sakila.Pg, "public", "sakila"},
		{sakila.My, "sakila", "def"},
		{sakila.MS, "dbo", "sakila"},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.handle, func(t *testing.T) {
			th, _, drvr, grip, db := testh.NewWith(t, tc.handle)

			gotSchema, err := drvr.CurrentSchema(th.Context, db)
			require.NoError(t, err)
			require.Equal(t, tc.wantSchema, gotSchema)

			md, err := grip.SourceMetadata(th.Context, false)
			require.NoError(t, err)
			require.NotNil(t, md)
			require.Equal(t, md.Schema, tc.wantSchema)
			require.Equal(t, md.Catalog, tc.wantCatalog)

			gotSchemas, err := drvr.ListSchemas(th.Context, db)
			require.NoError(t, err)
			require.Contains(t, gotSchemas, gotSchema)

			if drvr.Dialect().Catalog {
				gotCatalog, err := drvr.CurrentCatalog(th.Context, db)
				require.NoError(t, err)
				require.Equal(t, tc.wantCatalog, gotCatalog)
				gotCatalogs, err := drvr.ListCatalogs(th.Context, db)
				require.NoError(t, err)
				require.Contains(t, gotCatalogs, gotCatalog)
			}
		})
	}
}

func TestSQLDriver_SchemaExists(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		handle string
		schema string
		wantOK bool
	}{
		{handle: sakila.SL3, schema: "main", wantOK: true},
		{handle: sakila.SL3, schema: "", wantOK: false},
		{handle: sakila.SL3, schema: "not_exist", wantOK: false},
		{handle: sakila.Pg, schema: "public", wantOK: true},
		{handle: sakila.Pg, schema: "information_schema", wantOK: true},
		{handle: sakila.Pg, schema: "not_exist", wantOK: false},
		{handle: sakila.Pg, schema: "", wantOK: false},
		{handle: sakila.My, schema: "sakila", wantOK: true},
		{handle: sakila.My, schema: "", wantOK: false},
		{handle: sakila.My, schema: "not_exist", wantOK: false},
		{handle: sakila.MS, schema: "dbo", wantOK: true},
		{handle: sakila.MS, schema: "sys", wantOK: true},
		{handle: sakila.MS, schema: "INFORMATION_SCHEMA", wantOK: true},
		{handle: sakila.MS, schema: "", wantOK: false},
		{handle: sakila.MS, schema: "not_exist", wantOK: false},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tu.Name(tc.handle, tc.schema, tc.wantOK), func(t *testing.T) {
			t.Parallel()

			th, _, drvr, _, db := testh.NewWith(t, tc.handle)
			ok, err := drvr.SchemaExists(th.Context, db, tc.schema)
			require.NoError(t, err)
			require.Equal(t, tc.wantOK, ok)
		})
	}
}

func TestSQLDriver_CatalogExists(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		handle  string
		catalog string
		wantOK  bool
		wantErr bool
	}{
		{handle: sakila.SL3, catalog: "default", wantErr: true},
		{handle: sakila.SL3, catalog: "not_exist", wantErr: true},
		{handle: sakila.SL3, catalog: "", wantErr: true},
		{handle: sakila.Pg, catalog: "sakila", wantOK: true},
		{handle: sakila.Pg, catalog: "postgres", wantOK: true},
		{handle: sakila.Pg, catalog: "not_exist", wantOK: false},
		{handle: sakila.Pg, catalog: "", wantOK: false},
		{handle: sakila.My, catalog: "def", wantOK: true},
		{handle: sakila.My, catalog: "not_exist", wantOK: false},
		{handle: sakila.My, catalog: "", wantOK: false},
		{handle: sakila.MS, catalog: "sakila", wantOK: true},
		{handle: sakila.MS, catalog: "model", wantOK: true},
		{handle: sakila.MS, catalog: "not_exist", wantOK: false},
		{handle: sakila.MS, catalog: "", wantOK: false},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tu.Name(tc.handle, tc.catalog, tc.wantOK), func(t *testing.T) {
			t.Parallel()

			th, _, drvr, _, db := testh.NewWith(t, tc.handle)

			ok, err := drvr.CatalogExists(th.Context, db, tc.catalog)
			require.Equal(t, tc.wantOK, ok)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDriverCreateDropSchema(t *testing.T) {
	testCases := []struct {
		handle        string
		defaultSchema string
	}{
		{sakila.SL3, "main"},
		{sakila.Pg, "public"},
		{sakila.My, "sakila"},
		{sakila.MS, "dbo"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.handle, func(t *testing.T) {
			th, src, drvr, _, db := testh.NewWith(t, tc.handle)
			ctx := th.Context

			conn, err := db.Conn(th.Context)
			require.NoError(t, err)
			t.Cleanup(func() {
				assert.NoError(t, conn.Close())
			})

			gotSchema1, err := drvr.CurrentSchema(ctx, conn)
			require.NoError(t, err)
			require.Equal(t, tc.defaultSchema, gotSchema1)

			newSchema := "test_schema_" + stringz.Uniq8()

			err = drvr.CreateSchema(ctx, conn, newSchema)
			require.NoError(t, err)

			t.Cleanup(func() {
				err = drvr.DropSchema(ctx, conn, newSchema)
				assert.NoError(t, err)
			})

			schemaNames, err := drvr.ListSchemas(ctx, conn)
			require.NoError(t, err)
			require.Contains(t, schemaNames, tc.defaultSchema)
			require.Contains(t, schemaNames, newSchema)

			destTblFQ := tablefq.T{Schema: newSchema, Table: stringz.UniqTableName("actor2")}
			srcTblFQ := tablefq.From(sakila.TblActor)
			copied, err := drvr.CopyTable(ctx, conn, srcTblFQ, destTblFQ, true)
			require.NoError(t, err)
			require.Equal(t, int64(sakila.TblActorCount), copied)

			q := fmt.Sprintf("SELECT * FROM %s.%s", destTblFQ.Schema, destTblFQ.Table)
			sink, err := th.QuerySQL(src, conn, q)
			require.NoError(t, err)
			require.Equal(t, int64(sakila.TblActorCount), int64(len(sink.Recs)))

			// Do a second copy for good measure. We want to verify that CopyTable works
			// even on the non-default schema (this could probably be its own test).
			destTblFQ2 := tablefq.T{Schema: newSchema, Table: stringz.UniqSuffix("actor3_")}
			copied, err = drvr.CopyTable(ctx, conn, destTblFQ, destTblFQ2, true)
			require.NoError(t, err)
			require.Equal(t, int64(sakila.TblActorCount), copied)

			q = fmt.Sprintf("SELECT * FROM %s.%s", destTblFQ2.Schema, destTblFQ2.Table)
			sink, err = th.QuerySQL(src, conn, q)
			require.NoError(t, err)
			require.Equal(t, int64(sakila.TblActorCount), int64(len(sink.Recs)))
		})
	}
}

func TestSQLDriver_ErrWrap_IsErrNotExist(t *testing.T) {
	for _, h := range sakila.SQLLatest() {
		h := h
		t.Run(h, func(t *testing.T) {
			th, _, _, _, _ := testh.NewWith(t, h)
			_, err := th.QuerySLQ(h+".does_not_exist", nil)
			require.Error(t, err)
			require.True(t, errz.Has[*driver.NotExistError](err))
		})
	}
}

func TestMungeColNames(t *testing.T) {
	testCases := []struct {
		in   []string
		want []string
	}{
		{[]string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{[]string{"a", "b", "a", "d"}, []string{"a", "b", "a_1", "d"}},
		{[]string{"a", "b", "a", "b", "d", "a"}, []string{"a", "b", "a_1", "b_1", "d", "a_2"}},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			ctx := options.NewContext(context.Background(), options.Options{})
			got, err := driver.MungeResultColNames(ctx, tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestEmptyDataError(t *testing.T) {
	var err error
	require.False(t, errz.Has[driver.EmptyDataError](err))
	require.False(t, errz.Has[driver.EmptyDataError](errz.New("huzzah")))

	var ede1 driver.EmptyDataError
	require.True(t, errz.Has[driver.EmptyDataError](ede1))

	var ede2 driver.EmptyDataError
	require.True(t, errors.As(ede1, &ede2))

	err = driver.NewEmptyDataError("huzzah")
	require.True(t, errz.Has[driver.EmptyDataError](err))
	err = fmt.Errorf("wrap me: %w", err)
	require.True(t, errz.Has[driver.EmptyDataError](err))

	err = driver.NewEmptyDataError("%s doesn't exist", "me")
	require.True(t, errz.Has[driver.EmptyDataError](err))
	require.Equal(t, "me doesn't exist", err.Error())
}

func TestNotExistError(t *testing.T) {
	var err error
	require.False(t, errz.Has[*driver.NotExistError](err))

	var nee1 *driver.NotExistError
	require.True(t, errz.Has[*driver.NotExistError](nee1))

	var nee2 *driver.NotExistError
	require.True(t, errors.As(nee1, &nee2))

	err = driver.NewNotExistError(errz.New("huzzah"))
	require.True(t, errz.Has[*driver.NotExistError](err))
	err = fmt.Errorf("wrap me: %w", err)
	require.True(t, errz.Has[*driver.NotExistError](err))
}

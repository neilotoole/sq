package driver_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

func TestDriver_DropTable(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.SQLAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, handle)

			tblName := stringz.UniqTableName(sakila.TblActor)

			// Copy a table that we can play with. dropAfter=true registers
			// a safety-net ifExists drop at creation time, so the table is
			// reaped even if a drop assertion below fails.
			tblName = th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.From(tblName), false)
			require.NoError(t, drvr.DropTable(th.Context, db, tablefq.From(tblName), true))

			// Copy the table again so we can drop it again
			tblName = th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.From(tblName), false)

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

// TestDriver_TableExists_MultipleSchemas is a regression test for
// https://github.com/neilotoole/sq/issues/484. TableExists must scope its
// lookup to the connection's current schema. Previously the MySQL and Postgres
// drivers queried information_schema.tables filtered only by table name (no
// schema predicate) and compared COUNT(*) == 1, so a table whose name also
// existed in another schema produced the wrong result:
//
//   - Same-named table in two schemas: COUNT(*) == 2, so the "== 1" check
//     reported the table as missing, and --insert tried (and failed) to CREATE
//     an already-existing table ("Table 'x' already exists").
//   - Same-named table only in another schema: COUNT(*) == 1 reported the table
//     as present, so --insert skipped CREATE and the subsequent INSERT failed
//     because the table was absent from the current schema.
func TestDriver_TableExists_MultipleSchemas(t *testing.T) {
	// Only MySQL and Postgres were affected by #484. The other drivers were
	// already schema-scoped (or single-schema, for SQLite, whose unqualified
	// sqlite_master never sees attached schemas) and serve here as contract
	// guards against future regressions; ClickHouse, included since its
	// CopyTable was fixed to honor the target schema (#652), also guards that.
	// Oracle is omitted because its CreateSchema is unsupported.
	testCases := []struct {
		handle        string
		defaultSchema string
	}{
		{sakila.SL3, "main"},
		{sakila.Duck, "main"},
		{sakila.Pg, "public"},
		{sakila.My, "sakila"},
		{sakila.MS, "dbo"},
		{sakila.CH, "sakila"},
	}

	for _, tc := range testCases {
		t.Run(tc.handle, func(t *testing.T) {
			th, _, drvr, _, db := testh.NewWith(t, tc.handle)
			ctx := th.Context

			conn, err := db.Conn(ctx)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, conn.Close()) })

			gotSchema, err := drvr.CurrentSchema(ctx, conn)
			require.NoError(t, err)
			require.Equal(t, tc.defaultSchema, gotSchema)

			// otherSchema holds a same-named table that must NOT be mistaken
			// for one in the current schema.
			otherSchema := "test_schema_" + stringz.Uniq8()
			require.NoError(t, drvr.CreateSchema(ctx, conn, otherSchema))
			t.Cleanup(func() { assert.NoError(t, drvr.DropSchema(ctx, conn, otherSchema)) })

			tblName := stringz.UniqTableName("actor_484")
			srcTblFQ := tablefq.From(sakila.TblActor)

			// Create the table in otherSchema only.
			_, err = drvr.CopyTable(ctx, conn, srcTblFQ, tablefq.T{Schema: otherSchema, Table: tblName}, false)
			require.NoError(t, err)

			// TableExists checks the current schema, where the table does not
			// yet exist, so it must report false even though the name exists in
			// otherSchema.
			exists, err := drvr.TableExists(ctx, conn, tblName)
			require.NoError(t, err)
			require.False(t, exists,
				"table %q exists only in schema %q, not in current schema %q",
				tblName, otherSchema, tc.defaultSchema)

			// Now create the table in the current schema too, so the name
			// exists in two schemas at once.
			currentTblFQ := tablefq.From(tblName)
			t.Cleanup(func() { assert.NoError(t, drvr.DropTable(ctx, conn, currentTblFQ, true)) })
			_, err = drvr.CopyTable(ctx, conn, srcTblFQ, currentTblFQ, false)
			require.NoError(t, err)

			// TableExists must now report true: the table is present in the
			// current schema, even though the same name also exists in
			// otherSchema.
			exists, err = drvr.TableExists(ctx, conn, tblName)
			require.NoError(t, err)
			require.True(t, exists,
				"table %q exists in current schema %q (and also in %q)",
				tblName, tc.defaultSchema, otherSchema)
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

			// Each block scopes its own toTable so that t.Cleanup's closure
			// captures the correct name even if a later block reassigns. The
			// cleanup is registered immediately after the name is generated
			// so a failed assertion below still triggers DropTable — earlier
			// versions registered cleanup via `defer` after the assertions,
			// which left orphan tables (e.g. ACTOR__XXXXXX) in Oracle when
			// any intermediate assertion failed.
			{
				toTable := stringz.UniqTableName(sakila.TblActor)
				t.Cleanup(func() { th.DropTable(src, tablefq.From(toTable)) })

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
			}

			{
				toTable := stringz.UniqTableName(sakila.TblActor)
				t.Cleanup(func() { th.DropTable(src, tablefq.From(toTable)) })

				// Test 2: CopyTable with copyData = false
				// This should copy only the table structure (schema), not the data.
				// The returned count should always be 0 since no data is copied.
				// Note: dialect.RowsAffectedUnsupported should NOT be returned here
				// because when copyData=false, the driver knows exactly 0 rows were copied.
				copied, err := drvr.CopyTable(th.Context, db, tablefq.From(sakila.TblActor), tablefq.From(toTable), false)
				require.NoError(t, err)
				require.Equal(t, int64(0), copied)
				require.Equal(t, int64(0), th.RowCount(src, toTable))
			}
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

			// Skip Oracle: kind.Date and kind.Time don't roundtrip exactly.
			// Oracle's DATE type stores down to seconds (it's effectively a small
			// datetime), so kind.Date round-trips back as kind.Datetime. Oracle
			// has no time-only type, so kind.Time is stored as TIMESTAMP and
			// also reads back as kind.Datetime. See drivers/oracle/render.go and
			// drivers/oracle/README.md "Known limitations" for details.
			tu.SkipIf(t, drvr.DriverMetadata().Type == drivertype.Oracle,
				"Oracle: kind.Date and kind.Time don't roundtrip exactly (see README Known limitations)")

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

			gotNames := recMeta.Names()
			// Oracle returns identifiers in their stored case (upper for
			// unquoted), so column-name comparisons are case-insensitive.
			if drvr.DriverMetadata().Type == drivertype.Oracle {
				require.Len(t, gotNames, len(colNames))
				for i := range gotNames {
					require.True(t, strings.EqualFold(colNames[i], gotNames[i]),
						"col name got %q want ~%q", gotNames[i], colNames[i])
				}
			} else {
				require.Equal(t, colNames, gotNames)
			}
			require.Equal(t, colKinds, recMeta.Kinds())
		})
	}
}

func TestDriver_TableColumnTypes(t *testing.T) { //nolint:tparallel
	testCases := sakila.SQLAll()
	for _, handle := range testCases {
		t.Run(handle, func(t *testing.T) {
			tu.SkipShort(t, handle == sakila.XLSX)
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, handle)

			// Oracle returns identifier names in their stored case (upper for
			// unquoted), so column-name comparisons are case-insensitive.
			isOracle := drvr.DriverMetadata().Type == drivertype.Oracle
			eqName := func(want, got string) {
				t.Helper()
				if isOracle {
					require.True(t, strings.EqualFold(want, got),
						"col name got %q want ~%q", got, want)
				} else {
					require.Equal(t, want, got)
				}
			}

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
					eqName(sakila.TblActorCols()[i], colTypes[i].Name())
				}

				// Try again, but requesting specific col names
				wantColNames := []string{sakila.TblActorCols()[0], sakila.TblActorCols()[2]}
				colTypes, err = drvr.TableColumnTypes(th.Context, db, tblName, wantColNames)
				require.NoError(t, err)
				require.Equal(t, len(wantColNames), len(colTypes))
				for i := range colTypes {
					eqName(wantColNames[i], colTypes[i].Name())
				}
			}
		})
	}
}

func TestSQLDriver_PrepareUpdateStmt(t *testing.T) { //nolint:tparallel
	testCases := sakila.SQLAll()
	for _, handle := range testCases {
		t.Run(handle, func(t *testing.T) {
			tu.SkipShort(t, handle == sakila.XLSX)
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, handle)

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
			gotNames := stmtExecer.DestMeta().Names()
			// Oracle returns identifier names in their stored case (upper for
			// unquoted), so column-name comparisons are case-insensitive.
			if drvr.DriverMetadata().Type == drivertype.Oracle {
				require.Len(t, gotNames, len(destCols))
				for i := range gotNames {
					require.True(t, strings.EqualFold(destCols[i], gotNames[i]),
						"col name got %q want ~%q", gotNames[i], destCols[i])
				}
			} else {
				require.Equal(t, destCols, gotNames)
			}
			require.NoError(t, stmtExecer.Munge(wantVals))

			affected, err := stmtExecer.Exec(th.Context, args...)
			require.NoError(t, err)
			if drvr.DriverMetadata().Type == drivertype.ClickHouse {
				// ClickHouse mutations (ALTER TABLE UPDATE) are asynchronous
				// by default and do not report rows affected: RowsAffected()
				// always returns 0 regardless of how many rows were actually
				// modified. The driver's PrepareUpdateStmt appends
				// "SETTINGS mutations_sync = 1" to force synchronous execution
				// (so the data is updated before the SELECT below), but
				// RowsAffected() still returns 0. The driver returns
				// dialect.RowsAffectedUnsupported (-1) to signal "unknown".
				// See https://github.com/ClickHouse/clickhouse-go/issues/1203
				assert.Equal(t, dialect.RowsAffectedUnsupported, affected)
			} else {
				assert.Equal(t, int64(1), affected)
			}

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

// coreDrivers is a slice of the core driver types.
var coreDrivers = []drivertype.Type{
	drivertype.Pg,
	drivertype.MSSQL,
	drivertype.SQLite,
	drivertype.MySQL,
	drivertype.ClickHouse,
	drivertype.Oracle,
	drivertype.DuckDB,
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
	drivertype.Oracle,
	drivertype.DuckDB,
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
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, _, grip, _ := testh.NewWith(t, handle)

			tblMeta, err := grip.TableMetadata(th.Context, sakila.TblActor)
			require.NoError(t, err)
			// Oracle preserves the database's stored case (upper for unquoted
			// identifiers); other engines fold to lower.
			require.True(t, strings.EqualFold(sakila.TblActor, tblMeta.Name),
				"table name got %q want ~%q", tblMeta.Name, sakila.TblActor)
			require.Equal(t, int64(sakila.TblActorCount), tblMeta.RowCount)
		})
	}
}

func TestGrip_SourceMetadata(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.SQLAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, _, grip, _ := testh.NewWith(t, handle)

			md, err := grip.SourceMetadata(th.Context, false)
			require.NoError(t, err)
			require.True(t, strings.EqualFold(sakila.TblActor, md.Tables[0].Name),
				"first table name got %q want ~%q", md.Tables[0].Name, sakila.TblActor)
			require.Equal(t, int64(sakila.TblActorCount), md.Tables[0].RowCount)
		})
	}
}

func TestGrip_SourceMetadata_OracleViewsAndCounts(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	if !th.SourceConfigured(sakila.Ora) {
		t.Skip("Oracle Sakila source not configured")
	}

	th, _, _, grip, _ := testh.NewWith(t, sakila.Ora)

	md, err := grip.SourceMetadata(th.Context, false)
	require.NoError(t, err)
	require.NotNil(t, md)

	// The SAKILA Oracle image omits actor_info and nicer_but_slower_film_list
	// (they rely on MySQL GROUP_CONCAT); see sakiladb/oracle schema notes.
	require.Equal(t, int64(5), md.ViewCount)

	// Oracle stores unquoted identifiers as upper, so look up by uppercase.
	view := md.Table(strings.ToUpper(sakila.ViewFilmList))
	require.NotNil(t, view, "film_list view should appear in SourceMetadata.Tables")
	require.Equal(t, sqlz.TableTypeView, view.TableType)
	require.Equal(t, "VIEW", view.DBTableType)

	require.Equal(t, md.TableCount+md.ViewCount, int64(len(md.Tables)))
}

func TestSQLDriver_Oracle_TableExists_Objects(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	if !th.SourceConfigured(sakila.Ora) {
		t.Skip("Oracle Sakila source not configured")
	}

	th, _, drvr, _, db := testh.NewWith(t, sakila.Ora)

	ok, err := drvr.TableExists(th.Context, db, sakila.ViewFilmList)
	require.NoError(t, err)
	require.True(t, ok, "TableExists should be true for a view name")

	ok, err = drvr.TableExists(th.Context, db, "not_a_real_table_name_xyz999")
	require.NoError(t, err)
	require.False(t, ok)
}

// TestSQLDriver_ListTableNames_ArgSchemaEmpty tests [driver.SQLDriver.ListTableNames]
// with an empty schema arg.
func TestSQLDriver_ListTableNames_ArgSchemaEmpty(t *testing.T) { //nolint:tparallel
	for _, handle := range sakila.SQLLatest() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, drvr, _, db := testh.NewWith(t, handle)

			// Oracle returns identifiers in their stored case (upper for
			// unquoted), so use case-folded name comparisons.
			contains := func(haystack []string, needle string) bool {
				for _, s := range haystack {
					if strings.EqualFold(s, needle) {
						return true
					}
				}
				return false
			}

			got, err := drvr.ListTableNames(th.Context, db, "", false, false)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.True(t, len(got) == 0)

			got, err = drvr.ListTableNames(th.Context, db, "", true, false)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.True(t, contains(got, sakila.TblActor), "%v should contain ~%q", got, sakila.TblActor)
			require.False(t, contains(got, sakila.ViewFilmList), "%v should not contain ~%q", got, sakila.ViewFilmList)

			got, err = drvr.ListTableNames(th.Context, db, "", false, true)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.False(t, contains(got, sakila.TblActor), "%v should not contain ~%q", got, sakila.TblActor)
			require.True(t, contains(got, sakila.ViewFilmList), "%v should contain ~%q", got, sakila.ViewFilmList)

			got, err = drvr.ListTableNames(th.Context, db, "", true, true)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.True(t, contains(got, sakila.TblActor), "%v should contain ~%q", got, sakila.TblActor)
			require.True(t, contains(got, sakila.ViewFilmList), "%v should contain ~%q", got, sakila.ViewFilmList)

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
		// Oracle schemas are users; schema lookup is owner-scoped and case-insensitive.
		// The SAKILA Oracle image omits the film_text table and the actor_info /
		// nicer_but_slower_film_list views; see sakiladb/oracle schema notes.
		{handle: sakila.Ora, schema: "SAKILA", wantTables: 15, wantViews: 5},
	}

	for _, tc := range testCases {
		t.Run(tc.handle, func(t *testing.T) {
			t.Parallel()

			th, _, drvr, _, db := testh.NewWith(t, tc.handle)

			got, err := drvr.ListTableNames(th.Context, db, tc.schema, false, false)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.True(t, len(got) == 0)

			wantTables := tc.wantTables
			if tc.handle == sakila.Ora {
				// Oracle's "tables" list includes materialized views (ALL_MVIEWS).
				var mviewCount int
				err := db.QueryRowContext(th.Context,
					`SELECT COUNT(*) FROM all_mviews WHERE owner = :1`,
					strings.ToUpper(tc.schema)).Scan(&mviewCount)
				require.NoError(t, err)
				wantTables += mviewCount
			}

			// The table-inclusive assertions use a lower bound, not an exact
			// count. go test runs package binaries in parallel, and tests in
			// other packages (e.g. cli's TestCmdSQL_ExecMode) transiently
			// create scratch tables in these shared sakila schemas, which would
			// make an exact table count flaky. A missing expected table still
			// fails (len < wantTables). The views-only count stays exact: those
			// tests create tables, not views.
			got, err = drvr.ListTableNames(th.Context, db, tc.schema, true, false)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.GreaterOrEqual(t, len(got), wantTables)

			got, err = drvr.ListTableNames(th.Context, db, tc.schema, false, true)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got, tc.wantViews)

			got, err = drvr.ListTableNames(th.Context, db, tc.schema, true, true)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.GreaterOrEqual(t, len(got), wantTables+tc.wantViews)

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
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, _, _, grip, _ := testh.NewWith(t, handle)
			g, gCtx := errgroup.WithContext(th.Context)
			for range concurrency {
				g.Go(func() error {
					md, err := grip.SourceMetadata(gCtx, false)
					require.NoError(t, err)
					require.NotNil(t, md)
					// Oracle returns identifiers in their stored case (upper for
					// unquoted), so look up the table case-insensitively.
					var found bool
					for _, tbl := range md.Tables {
						if strings.EqualFold(tbl.Name, sakila.TblActor) {
							found = true
							require.Equal(t, int64(sakila.TblActorCount), tbl.RowCount)
							break
						}
					}
					require.True(t, found, "table %q not in metadata", sakila.TblActor)
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
			// Oracle returns identifier names in their stored case (upper for
			// unquoted), so column-name comparisons are case-insensitive.
			if drvr.DriverMetadata().Type == drivertype.Oracle {
				require.Len(t, gotCols, len(wantCols))
				for i := range gotCols {
					require.True(t, strings.EqualFold(wantCols[i], gotCols[i]),
						"col name got %q want ~%q", gotCols[i], wantCols[i])
				}
			} else {
				require.Equal(t, wantCols, gotCols)
			}

			gotKinds := sink.RecMeta.Kinds()
			require.Equal(t, wantKinds, gotKinds)
		})
	}
}

func TestSQLDriver_AlterTableRename(t *testing.T) {
	testCases := sakila.SQLLatest()

	for _, handle := range testCases {
		t.Run(handle, func(t *testing.T) {
			th, src, drvr, grip, db := testh.NewWith(t, handle)

			// Make a copy of the table to play with. dropAfter=true registers
			// an ifExists drop for the original name, which is a no-op after
			// the rename succeeds.
			tbl := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)

			// Register the drop of the new name before the rename: if an
			// assertion below fails, the renamed table must still be reaped
			// from the shared container.
			newName := stringz.UniqSuffix("actor_copy_")
			t.Cleanup(func() { th.DropTable(src, tablefq.From(newName)) })
			err := drvr.AlterTableRename(th.Context, db, tbl, newName)
			require.NoError(t, err)

			md, err := grip.TableMetadata(th.Context, newName)
			require.NoError(t, err)
			// Oracle returns the database's stored case (upper for unquoted).
			require.True(t, strings.EqualFold(newName, md.Name),
				"renamed-table name got %q want ~%q", md.Name, newName)
			sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+newName)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

func TestSQLDriver_AlterTableRenameColumn(t *testing.T) {
	testCases := sakila.SQLLatest()

	for _, handle := range testCases {
		t.Run(handle, func(t *testing.T) {
			th, src, drvr, grip, db := testh.NewWith(t, handle)

			// Make a copy of the table to play with
			tbl := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)

			newName := "given_name"
			err := drvr.AlterTableRenameColumn(th.Context, db, tbl, "first_name", newName)
			require.NoError(t, err)

			md, err := grip.TableMetadata(th.Context, tbl)
			require.NoError(t, err)
			// Oracle stores unquoted identifiers as upper; look up under that.
			lookupName := newName
			if drvr.DriverMetadata().Type == drivertype.Oracle {
				lookupName = strings.ToUpper(newName)
			}
			require.NotNil(t, md.Column(lookupName), "%s column not found in %v", lookupName, md.Columns)
			sink, err := th.QuerySQL(src, nil, fmt.Sprintf("SELECT %s FROM %s", newName, tbl))
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

func TestSQLDriver_CurrentCatalog(t *testing.T) {
	for _, handle := range sakila.SQLAll() {
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
		{sakila.CH, "sakila", "sakila"},
		// Oracle: schema is the connected user; the catalog is DB_NAME
		// (the PDB name in multitenant deployments).
		{sakila.Ora, "SAKILA", "SAKILA"},
	}

	for _, tc := range testCases {
		t.Run(tc.handle, func(t *testing.T) {
			th, _, drvr, grip, db := testh.NewWith(t, tc.handle)

			gotSchema, err := drvr.CurrentSchema(th.Context, db)
			require.NoError(t, err)
			if drvr.DriverMetadata().Type == drivertype.Oracle {
				require.True(t, strings.EqualFold(tc.wantSchema, gotSchema),
					"schema got %q want ~%q", gotSchema, tc.wantSchema)
			} else {
				require.Equal(t, tc.wantSchema, gotSchema)
			}

			md, err := grip.SourceMetadata(th.Context, false)
			require.NoError(t, err)
			require.NotNil(t, md)
			require.Equal(t, gotSchema, md.Schema)
			require.Equal(t, tc.wantCatalog, md.Catalog)

			gotSchemas, err := drvr.ListSchemas(th.Context, db)
			require.NoError(t, err)
			if drvr.DriverMetadata().Type == drivertype.Oracle {
				require.True(t, slices.ContainsFunc(gotSchemas, func(s string) bool {
					return strings.EqualFold(s, gotSchema)
				}), "schemas should contain %q, got %v", gotSchema, gotSchemas)
			} else {
				require.Contains(t, gotSchemas, gotSchema)
			}

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

// TestSQLDriver_SourceMetadata_FieldCoverage asserts that each SQL driver
// populates the source-level [metadata.Source] fields its real users see
// in `sq inspect`. The expectation matrix per handle pins the current
// per-driver behavior so any future driver change that drops a field
// (or starts populating one) will be caught here.
//
//nolint:tparallel
func TestSQLDriver_SourceMetadata_FieldCoverage(t *testing.T) {
	// wantPKIndex pins whether the driver surfaces the PK-backing
	// index as a [metadata.Index] entry. DuckDB's duckdb_indexes()
	// doesn't expose the implicit PK index, so it leaves the PK
	// information on Column.PrimaryKey only.
	testCases := []struct {
		handle      string
		wantUser    bool // SQLite has no auth; SQL Server doesn't populate it.
		wantSize    bool // every SAKILA DB has data, so size > 0 is expected.
		wantProps   bool // DBProperties map should be non-empty.
		wantFKs     bool // FK metadata. ClickHouse has no FK support.
		wantPKIndex bool // PK-backing index surfaced as a [metadata.Index] entry.
	}{
		{handle: sakila.SL3, wantUser: false, wantSize: true, wantProps: true, wantFKs: true, wantPKIndex: true},
		{handle: sakila.Pg, wantUser: true, wantSize: true, wantProps: true, wantFKs: true, wantPKIndex: true},
		{handle: sakila.My, wantUser: true, wantSize: true, wantProps: true, wantFKs: true, wantPKIndex: true},
		{handle: sakila.MS, wantUser: false, wantSize: true, wantProps: true, wantFKs: true, wantPKIndex: true},
		{handle: sakila.CH, wantUser: true, wantSize: true, wantProps: true, wantFKs: false, wantPKIndex: false},
		{handle: sakila.Ora, wantUser: true, wantSize: true, wantProps: true, wantFKs: true, wantPKIndex: true},
		// DuckDB's getSourceMetadata doesn't populate Source.DBProperties
		// today (the driver exposes DBProperties via its SQLDriver method
		// but doesn't wire it into source-level inspect). DuckDB also
		// doesn't surface PK-backing indexes via duckdb_indexes().
		{handle: sakila.Duck, wantUser: false, wantSize: true, wantProps: false, wantFKs: true, wantPKIndex: false},
	}

	for _, tc := range testCases {
		t.Run(tc.handle, func(t *testing.T) {
			t.Parallel()

			th, _, _, grip, _ := testh.NewWith(t, tc.handle)
			md, err := grip.SourceMetadata(th.Context, false)
			require.NoError(t, err)
			require.NotNil(t, md)

			// Always-populated fields.
			require.NotEmpty(t, md.Handle, "Handle")
			require.NotEmpty(t, md.Location, "Location")
			require.NotEmpty(t, md.Driver, "Driver")
			require.NotEmpty(t, md.DBDriver, "DBDriver")
			require.NotEmpty(t, md.Name, "Name")
			require.NotEmpty(t, md.FQName, "FQName")
			require.NotEmpty(t, md.Schema, "Schema")
			require.NotEmpty(t, md.Catalog, "Catalog")
			require.NotEmpty(t, md.DBProduct, "DBProduct")
			require.NotEmpty(t, md.DBVersion, "DBVersion")
			require.NotEmpty(t, md.Tables, "Tables")
			require.Positive(t, md.TableCount, "TableCount")
			// SAKILA includes views on every supported driver except where the
			// upstream image omits them; assert non-zero where applicable.
			require.NotZero(t, md.ViewCount, "ViewCount")

			if tc.wantUser {
				require.NotEmpty(t, md.User, "User")
			}
			if tc.wantSize {
				require.NotNil(t, md.Size, "Size")
				require.Positive(t, *md.Size, "Size")
			}
			if tc.wantProps {
				require.NotEmpty(t, md.DBProperties, "DBProperties")
			}

			// Per-table invariants. Every table/view returned from
			// SourceMetadata should carry the always-populated fields below
			// regardless of driver — these are the columns users see in
			// `sq inspect` and rely on for cross-source operations.
			for _, tbl := range md.Tables {
				require.NotEmpty(t, tbl.Name, "Table.Name on %s", tc.handle)
				require.NotEmpty(t, tbl.FQName, "Table.FQName on %s.%s", tc.handle, tbl.Name)
				require.NotEmpty(t, tbl.TableType, "Table.TableType on %s.%s", tc.handle, tbl.Name)
				require.NotEmpty(t, tbl.DBTableType, "Table.DBTableType on %s.%s", tc.handle, tbl.Name)
				require.NotNil(t, tbl.Columns, "Table.Columns on %s.%s", tc.handle, tbl.Name)
			}

			if tc.wantPKIndex {
				// Most SAKILA drivers create a PK-backing index that
				// surfaces in duckdb-style catalog views as a regular
				// Index entry. Verify Table.Indexes includes it.
				// DuckDB intentionally doesn't expose PK-backing indexes
				// in duckdb_indexes(), so this assertion is gated.
				language := findTable(md.Tables, "language")
				require.NotNil(t, language, "language table missing from %s", tc.handle)
				require.NotEmpty(t, language.Indexes,
					"language.Indexes should be populated on %s", tc.handle)
				var langPK *metadata.Index
				for _, idx := range language.Indexes {
					if idx.Primary {
						langPK = idx
						break
					}
				}
				require.NotNil(t, langPK,
					"language should have a Primary index on %s", tc.handle)
				require.True(t, langPK.Unique, "PK index must be unique on %s", tc.handle)
				require.Equal(t, []string{"language_id"}, lowerAll(langPK.Columns),
					"language PK index covers language_id on %s", tc.handle)
			}

			if tc.wantFKs {
				// sakila.film.language_id references sakila.language.language_id
				// across every supported SQL source, making it the canonical
				// FK-coverage smoke check.
				film := findTable(md.Tables, sakila.TblFilm)
				require.NotNil(t, film, "film table missing from %s metadata", tc.handle)
				langFK := findOutgoingFK(film, "language_id")
				require.NotNil(t, langFK, "film.language_id outgoing FK missing on %s", tc.handle)
				require.Equal(t, "language", strings.ToLower(langFK.RefTable),
					"film.language_id should reference language on %s", tc.handle)
				require.Equal(t, []string{"language_id"},
					lowerAll(langFK.RefColumns),
					"film.language_id ref columns on %s", tc.handle)

				// Incoming back-reference on language.
				language := findTable(md.Tables, "language")
				require.NotNil(t, language, "language table missing from %s metadata", tc.handle)
				require.NotEmpty(t, language.FK.Incoming,
					"language.FK.Incoming should include the film FK on %s", tc.handle)
			}
		})
	}
}

// findTable returns the table whose Name matches name (case-insensitive,
// since Oracle reports identifiers in upper case while other drivers
// preserve case). Returns nil if no match.
func findTable(tables []*metadata.Table, name string) *metadata.Table {
	want := strings.ToLower(name)
	for _, tbl := range tables {
		if strings.ToLower(tbl.Name) == want {
			return tbl
		}
	}
	return nil
}

// findOutgoingFK returns the first single-column outgoing FK on tbl
// whose referencing column matches colName (case-insensitive), or nil.
func findOutgoingFK(tbl *metadata.Table, colName string) *metadata.ForeignKey {
	if tbl.FK == nil {
		return nil
	}
	want := strings.ToLower(colName)
	for _, fk := range tbl.FK.Outgoing {
		if len(fk.Columns) == 1 && strings.ToLower(fk.Columns[0]) == want {
			return fk
		}
	}
	return nil
}

// lowerAll returns a copy of ss with every element lower-cased. Used to
// normalize identifier casing across drivers (Oracle returns upper case).
func lowerAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToLower(s)
	}
	return out
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
		{handle: sakila.CH, schema: "sakila", wantOK: true},
		{handle: sakila.CH, schema: "", wantOK: false},
		{handle: sakila.CH, schema: "not_exist", wantOK: false},
		{handle: sakila.Ora, schema: "sakila", wantOK: true},
		{handle: sakila.Ora, schema: "", wantOK: false},
		{handle: sakila.Ora, schema: "not_exist", wantOK: false},
	}

	for _, tc := range testCases {
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
		{handle: sakila.CH, catalog: "sakila", wantOK: true},
		{handle: sakila.CH, catalog: "system", wantOK: true},
		{handle: sakila.CH, catalog: "not_exist", wantOK: false},
		{handle: sakila.CH, catalog: "", wantOK: false},
		{handle: sakila.Ora, catalog: "sakila", wantOK: false, wantErr: true},
		{handle: sakila.Ora, catalog: "X", wantOK: false, wantErr: true},
		{handle: sakila.Ora, catalog: "", wantOK: false, wantErr: true},
	}

	for _, tc := range testCases {
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
		// Note: ClickHouse is tested separately in
		// drivers/clickhouse because CopyTable returns
		// dialect.RowsAffectedUnsupported (-1) instead of
		// the actual row count.
	}

	for _, tc := range testCases {
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

// TestDriverCreateDropSchema_Numeric tests creating and dropping schemas
// with numeric and numeric-prefixed names. This validates that the grammar
// changes in PR #515 (issue #470) work correctly across all SQL drivers.
// See: https://github.com/neilotoole/sq/issues/470
func TestDriverCreateDropSchema_Numeric(t *testing.T) {
	// Test cases for numeric and numeric-prefixed schema names.
	// These names require proper quoting in SQL generation.
	numericSchemas := []struct {
		name   string // test name
		schema string // schema name to create
	}{
		{"pure_zero", "0"},
		{"pure_numeric", "12345"},
		{"numeric_prefixed", "123abc"},
		{"numeric_with_underscore", "456_test"},
		{"numeric_long", "9876543210"},
		{"numeric_mixed", "0a1b2c"},
		// Test leading zeros (issue #470)
		{"leading_zeros_short", "007"},
		{"leading_zeros_long", "00123"},
		{"leading_zeros_many", "000000"},
	}

	// Drivers that support schema creation.
	driverCases := []struct {
		handle        string
		defaultSchema string
	}{
		{sakila.Pg, "public"},
		{sakila.My, "sakila"},
		{sakila.MS, "dbo"},
		// Note: SQLite doesn't support CREATE SCHEMA in the traditional sense.
	}

	for _, dc := range driverCases {
		t.Run(dc.handle, func(t *testing.T) {
			th, src, drvr, _, db := testh.NewWith(t, dc.handle)
			ctx := th.Context

			conn, err := db.Conn(ctx)
			require.NoError(t, err)
			t.Cleanup(func() {
				assert.NoError(t, conn.Close())
			})

			for _, nc := range numericSchemas {
				t.Run(nc.name, func(t *testing.T) {
					// Make schema name unique to avoid conflicts.
					schemaName := nc.schema + "_" + stringz.Uniq8()

					// Create the numeric schema.
					err := drvr.CreateSchema(ctx, conn, schemaName)
					require.NoError(t, err, "CreateSchema(%q) should succeed", schemaName)

					t.Cleanup(func() {
						dropErr := drvr.DropSchema(ctx, conn, schemaName)
						assert.NoError(t, dropErr, "DropSchema(%q) cleanup should succeed", schemaName)
					})

					// Verify schema exists.
					exists, err := drvr.SchemaExists(ctx, conn, schemaName)
					require.NoError(t, err, "SchemaExists(%q) should not error", schemaName)
					require.True(t, exists, "SchemaExists(%q) should return true", schemaName)

					// Verify schema appears in list.
					schemaNames, err := drvr.ListSchemas(ctx, conn)
					require.NoError(t, err, "ListSchemas should not error")
					require.Contains(t, schemaNames, schemaName,
						"ListSchemas should contain %q", schemaName)

					// Copy a table into the numeric schema.
					destTblFQ := tablefq.T{Schema: schemaName, Table: stringz.UniqTableName("actor")}
					srcTblFQ := tablefq.From(sakila.TblActor)
					copied, err := drvr.CopyTable(ctx, conn, srcTblFQ, destTblFQ, true)
					require.NoError(t, err, "CopyTable to numeric schema should succeed")
					require.Equal(t, int64(sakila.TblActorCount), copied,
						"CopyTable should copy all rows")

					// Query the table in the numeric schema.
					// The schema name must be properly quoted in the SQL.
					q := fmt.Sprintf("SELECT * FROM %s.%s",
						drvr.Dialect().Enquote(destTblFQ.Schema),
						drvr.Dialect().Enquote(destTblFQ.Table))
					sink, err := th.QuerySQL(src, conn, q)
					require.NoError(t, err, "Query in numeric schema should succeed")
					require.Equal(t, int64(sakila.TblActorCount), int64(len(sink.Recs)),
						"Query should return all rows")
				})
			}
		})
	}
}

func TestSQLDriver_ErrWrap_IsErrNotExist(t *testing.T) {
	for _, h := range sakila.SQLLatest() {
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
		// Case-insensitive duplicate detection: cross-source joins with
		// Oracle (UPPERCASE) on one side and another driver (lowercase)
		// on the other produce mixed-case column names. The munged form
		// preserves the original case but the disambiguation suffix is
		// applied as if names were case-folded.
		{[]string{"A", "a"}, []string{"A", "a_1"}},
		{[]string{"ID", "id", "Id"}, []string{"ID", "id_1", "Id_2"}},
		{
			[]string{"STORE_ID", "ADDRESS_ID", "address_id", "last_update", "LAST_UPDATE"},
			[]string{"STORE_ID", "ADDRESS_ID", "address_id_1", "last_update", "LAST_UPDATE_1"},
		},
	}

	for i, tc := range testCases {
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

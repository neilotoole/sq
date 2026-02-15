package sqlserver_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestSmoke(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.MSAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, _, _, _ := testh.NewWith(t, handle)
			sink, err := th.QuerySQL(src, nil, "SELECT * FROM actor")
			require.NoError(t, err)
			require.Equal(t, len(sakila.TblActorCols()), len(sink.RecMeta))
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

func TestDriverBehavior(t *testing.T) {
	t.Parallel()

	// This test exists to help understand the behavior of the driver impl.
	// It can be deleted eventually.

	th := testh.New(t)
	src := th.Source(sakila.MS)
	db := th.OpenDB(src)

	const query = "SELECT * FROM payment ORDER BY (SELECT 0) OFFSET 0 ROWS FETCH NEXT 1 ROWS ONLY"

	rows, err := db.QueryContext(th.Context, query)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, rows.Close()) })

	colTypes, err := rows.ColumnTypes()
	require.NoError(t, err)

	for i, colType := range colTypes {
		nullable, ok := colType.Nullable()
		t.Logf("%d:	%s	%s	%s	nullable,ok={%v,%v}", i, colType.Name(), colType.DatabaseTypeName(),
			colType.ScanType().Name(), nullable, ok)

		if !nullable {
			scanType := colType.ScanType()
			z := reflect.Zero(scanType)
			t.Logf("zero: %T %v", z, z)
		}
	}
}

func TestDriver_CreateTable_NotNullDefault(t *testing.T) {
	t.Parallel()

	testCases := []string{sakila.MS}
	for _, handle := range testCases {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, handle)

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
			for i := range colNames {
				require.NotNil(t, sink.Recs[0][i])
				nullable, ok := sink.RecMeta[i].Nullable()
				require.True(t, ok)
				require.False(t, nullable)
			}

			// Check kind.Bytes is handled correctly
			const iBytes = 8 // the index of col_bytes
			require.Equal(t, kind.Bytes, colKinds[iBytes])
			b, ok := sink.Recs[0][iBytes].([]byte)
			require.True(t, ok)
			require.NotNil(t, b)
			require.Equal(t, 0, len(b), "b should be non-nil but zero length")
		})
	}
}

// TestNumericSchema tests that numeric and numeric-prefixed schema names
// work correctly in SQL Server.
// This tests the fix for issue #470.
// See: https://github.com/neilotoole/sq/issues/470
func TestNumericSchema(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		schema string
	}{
		{"pure_numeric", "12345"},
		{"numeric_prefixed", "123testschema"},
		{"numeric_with_underscore", "456_schema"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, sakila.MS)
			ctx := th.Context

			// Make the schema name unique.
			schemaName := tc.schema + "_" + stringz.Uniq8()

			// Create the schema with a numeric name.
			// In SQL Server, the schema name must be properly quoted.
			err := drvr.CreateSchema(ctx, db, schemaName)
			require.NoError(t, err, "CreateSchema(%q) should succeed for SQL Server", schemaName)

			t.Cleanup(func() {
				_ = drvr.DropSchema(ctx, db, schemaName)
			})

			// Verify the schema exists.
			exists, err := drvr.SchemaExists(ctx, db, schemaName)
			require.NoError(t, err)
			require.True(t, exists, "SchemaExists(%q) should return true", schemaName)

			// List schemas and verify our numeric schema appears.
			schemas, err := drvr.ListSchemas(ctx, db)
			require.NoError(t, err)
			require.Contains(t, schemas, schemaName,
				"ListSchemas should contain %q", schemaName)

			// Copy a table to the new schema.
			destTblFQ := tablefq.T{Schema: schemaName, Table: stringz.UniqTableName("actor")}
			srcTblFQ := tablefq.From(sakila.TblActor)
			copied, err := drvr.CopyTable(ctx, db, srcTblFQ, destTblFQ, true)
			require.NoError(t, err, "CopyTable to numeric schema should succeed")
			require.Equal(t, int64(sakila.TblActorCount), copied)

			// Query the table in the numeric schema.
			// The schema name must be properly double-quoted in the SQL.
			q := `SELECT * FROM "` + destTblFQ.Schema + `"."` + destTblFQ.Table + `"`
			sink, err := th.QuerySQL(src, nil, q)
			require.NoError(t, err, "Query in numeric schema should succeed")
			require.Equal(t, sakila.TblActorCount, len(sink.Recs),
				"Query should return all rows")
		})
	}
}

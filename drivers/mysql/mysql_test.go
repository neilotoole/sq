package mysql_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestSmoke(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.MyAll() {
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

func TestDriver_CreateTable_NotNullDefault(t *testing.T) {
	t.Parallel()

	testCases := sakila.MyAll()
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

			// MySQL doesn't support default values for TEXT or BLOB
			// See: https://bugs.mysql.com/bug.php?id=21532
			// So, instead of "INSERT INTO tblName () VALUES ()" we
			// need to provide explicit values for col_text and col_bytes.
			insertDefaultStmt := "INSERT INTO " + tblName + " (col_text, col_bytes) VALUES (?, ?)"
			affected := th.ExecSQL(src, insertDefaultStmt, "", []byte{})
			require.Equal(t, int64(1), affected)

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
		})
	}
}

// TestBug252_ShowCollation_uint64 tests the
// bug https://github.com/neilotoole/sq/issues/252.
func TestBug252_ShowCollation_uint64(t *testing.T) {
	testCases := sakila.MyAll()
	for _, handle := range testCases {
		t.Run(handle, func(t *testing.T) {
			th, src, _, _, _ := testh.NewWith(t, handle)

			sink, err := th.QuerySQL(src, nil, "SHOW COLLATION")
			require.NoError(t, err)
			require.NotNil(t, sink)
		})
	}
}

// TestNumericSchema tests that numeric and numeric-prefixed database names
// (which are synonymous with schemas in MySQL) work correctly.
// This tests the fix for issue #470.
// See: https://github.com/neilotoole/sq/issues/470
//
// Note: In MySQL, "schema" and "database" are interchangeable.
// Creating a schema with a numeric name requires backtick quoting.
func TestNumericSchema(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		schema string
	}{
		// Note: Pure numeric database names in MySQL are tricky and may
		// require special handling depending on MySQL version.
		// We focus on numeric-prefixed names which are more commonly used.
		{"numeric_prefixed", "123testdb"},
		{"numeric_with_underscore", "456_testdb"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, sakila.My)
			ctx := th.Context

			// Make the schema/database name unique.
			schemaName := tc.schema + "_" + stringz.Uniq8()

			// Create the database with a numeric-prefixed name.
			err := drvr.CreateSchema(ctx, db, schemaName)
			require.NoError(t, err, "CreateSchema(%q) should succeed for MySQL", schemaName)

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
			// The schema name must be properly backtick-quoted in the SQL.
			q := "SELECT * FROM `" + destTblFQ.Schema + "`.`" + destTblFQ.Table + "`"
			sink, err := th.QuerySQL(src, nil, q)
			require.NoError(t, err, "Query in numeric schema should succeed")
			require.Equal(t, sakila.TblActorCount, len(sink.Recs),
				"Query should return all rows")
		})
	}
}

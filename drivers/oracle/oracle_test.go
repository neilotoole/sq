package oracle_test

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/drivers/oracle"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestSmoke is a basic smoke test to verify the driver works.
func TestSmoke(t *testing.T) {
	tu.SkipShort(t, true)
	th, src, drvr, _, db := testh.NewWith(t, sakila.Ora)
	ctx := th.Context

	require.NoError(t, drvr.Ping(ctx, src, driver.ModeReadWrite), "Ping should succeed")

	var result int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT 1 FROM DUAL").Scan(&result))
	assert.Equal(t, 1, result)
}

// TestOracle_DBProperties_BestEffort verifies DBProperties returns core fields
// even when v$instance is not readable (version may come from v$version).
func TestOracle_DBProperties_BestEffort(t *testing.T) {
	tu.SkipShort(t, true)
	th, _, drvr, _, db := testh.NewWith(t, sakila.Ora)

	props, err := drvr.DBProperties(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, props["db_name"])
	require.NotEmpty(t, props["current_schema"])
	if v, ok := props["version"].(string); ok {
		require.NotEmpty(t, v)
	}
}

// TestCurrentSchema tests the CurrentSchema method.
func TestCurrentSchema(t *testing.T) {
	tu.SkipShort(t, true)
	th, _, drvr, _, db := testh.NewWith(t, sakila.Ora)

	schemaName, err := drvr.CurrentSchema(th.Context, db)
	require.NoError(t, err)
	assert.NotEmpty(t, schemaName)
}

// TestCreateAndDropTable tests table creation and deletion.
func TestCreateAndDropTable(t *testing.T) {
	tu.SkipShort(t, true)
	th, _, drvr, _, db := testh.NewWith(t, sakila.Ora)
	ctx := th.Context

	// Create a test table
	tblName := stringz.UniqSuffix("TEST")
	tblDef := &schema.Table{
		Name: tblName,
		Cols: []*schema.Column{
			{Name: "ID", Kind: kind.Int, NotNull: true},
			{Name: "NAME", Kind: kind.Text, NotNull: false},
			{Name: "CREATED_AT", Kind: kind.Datetime, NotNull: false},
		},
	}

	err := drvr.CreateTable(ctx, db, tblDef)
	require.NoError(t, err, "CreateTable should succeed")

	// Verify table exists
	exists, err := drvr.TableExists(ctx, db, tblName)
	require.NoError(t, err)
	assert.True(t, exists, "Table should exist after creation")

	// Insert a test row
	_, err = db.ExecContext(ctx,
		`INSERT INTO "`+strings.ToUpper(tblName)+`" (ID, NAME, CREATED_AT) VALUES (:1, :2, :3)`,
		1, "Test", nil)
	require.NoError(t, err, "Insert should succeed")

	// Query the row back
	var id int
	var name sql.NullString
	err = db.QueryRowContext(ctx, `SELECT ID, NAME FROM "`+strings.ToUpper(tblName)+`" WHERE ID = :1`, 1).Scan(&id, &name)
	require.NoError(t, err)
	assert.Equal(t, 1, id)
	assert.True(t, name.Valid)
	assert.Equal(t, "Test", name.String)

	// Drop the table
	err = drvr.DropTable(ctx, db, tablefq.From(tblName), false)
	require.NoError(t, err, "DropTable should succeed")

	// Verify table no longer exists
	exists, err = drvr.TableExists(ctx, db, tblName)
	require.NoError(t, err)
	assert.False(t, exists, "Table should not exist after drop")
}

// TestTypeMappings tests various Oracle data types.
func TestTypeMappings(t *testing.T) {
	tu.SkipShort(t, true)
	th, _, drvr, _, db := testh.NewWith(t, sakila.Ora)
	ctx := th.Context

	// Create a test table with various types
	tblName := stringz.UniqSuffix("TEST_TYPES")
	tblDef := &schema.Table{
		Name: tblName,
		Cols: []*schema.Column{
			{Name: "COL_INT", Kind: kind.Int},
			{Name: "COL_TEXT", Kind: kind.Text},
			{Name: "COL_FLOAT", Kind: kind.Float},
			{Name: "COL_DECIMAL", Kind: kind.Decimal},
			{Name: "COL_BOOL", Kind: kind.Bool},
			{Name: "COL_DATETIME", Kind: kind.Datetime},
			{Name: "COL_DATE", Kind: kind.Date},
			{Name: "COL_BYTES", Kind: kind.Bytes},
		},
	}

	err := drvr.CreateTable(ctx, db, tblDef)
	require.NoError(t, err)
	defer func() { _ = drvr.DropTable(ctx, db, tablefq.From(tblName), true) }()

	// Insert test data
	testTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	testDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx,
		`INSERT INTO "`+strings.ToUpper(tblName)+`" VALUES (:1, :2, :3, :4, :5, :6, :7, :8)`,
		42, "test text", 3.14, 123.45, 1, testTime, testDate, []byte{1, 2, 3})
	require.NoError(t, err)

	// Query back and verify types
	rows, err := db.QueryContext(ctx, `SELECT * FROM "`+strings.ToUpper(tblName)+`"`)
	require.NoError(t, err)
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	require.NoError(t, err)
	assert.Equal(t, 8, len(colTypes))

	// Verify we can scan the row
	require.True(t, rows.Next())

	values := make([]any, 8)
	for i := range values {
		values[i] = new(any)
	}

	err = rows.Scan(values...)
	require.NoError(t, err)
	require.NoError(t, rows.Err())
}

// TestListTables tests listing tables.
func TestListTables(t *testing.T) {
	tu.SkipShort(t, true)
	th, _, drvr, _, db := testh.NewWith(t, sakila.Ora)

	tables, err := drvr.ListTableNames(th.Context, db, "", true, false)
	require.NoError(t, err)
	assert.NotNil(t, tables)
}

// TestSakilaCrossDatabase tests reading data from Postgres and writing to Oracle.
// This is a real-world integration test demonstrating cross-database data migration.
func TestSakilaCrossDatabase(t *testing.T) {
	tu.SkipShort(t, true)
	th := testh.New(t)
	pgDB := th.OpenDB(th.Source(sakila.Pg))
	oraGrip := th.Open(th.Source(sakila.Ora))
	oraDrvr := oraGrip.SQLDriver()
	ctx := th.Context
	oraDB, err := oraGrip.DB(ctx)
	require.NoError(t, err)

	// Test 1: Copy actor table
	t.Run("CopyActorTable", func(t *testing.T) {
		testTableName := stringz.UniqSuffix("ACTOR")

		// Read actor data from Postgres
		rows, err := pgDB.QueryContext(ctx, "SELECT actor_id, first_name, last_name FROM actor ORDER BY actor_id")
		require.NoError(t, err, "Failed to query Postgres actor table")
		defer rows.Close()

		// Collect rows
		type actorRow struct {
			ActorID   int
			FirstName string
			LastName  string
		}

		var actors []actorRow
		for rows.Next() {
			var a actorRow
			err := rows.Scan(&a.ActorID, &a.FirstName, &a.LastName)
			require.NoError(t, err)
			actors = append(actors, a)
		}
		require.NoError(t, rows.Err())
		require.NotEmpty(t, actors, "Expected actor data in Postgres")

		pgRowCount := len(actors)

		// Create table in Oracle
		tblDef := &schema.Table{
			Name: testTableName,
			Cols: []*schema.Column{
				{Name: "ACTOR_ID", Kind: kind.Int, NotNull: true},
				{Name: "FIRST_NAME", Kind: kind.Text, NotNull: true},
				{Name: "LAST_NAME", Kind: kind.Text, NotNull: true},
			},
		}

		err = oraDrvr.CreateTable(ctx, oraDB, tblDef)
		require.NoError(t, err, "Failed to create actor table in Oracle")

		// Insert data into Oracle (use uppercase table name)
		insertSQL := fmt.Sprintf(
			`INSERT INTO "%s" (ACTOR_ID, FIRST_NAME, LAST_NAME) VALUES (:1, :2, :3)`,
			strings.ToUpper(testTableName),
		)
		stmt, err := oraDB.PrepareContext(ctx, insertSQL)
		require.NoError(t, err, "Failed to prepare insert statement")
		defer stmt.Close()

		for _, actor := range actors {
			_, err := stmt.ExecContext(ctx, actor.ActorID, actor.FirstName, actor.LastName)
			require.NoError(t, err, "Failed to insert actor %d", actor.ActorID)
		}

		// Verify row count in Oracle (use uppercase table name)
		var oracleRowCount int
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, strings.ToUpper(testTableName))
		err = oraDB.QueryRowContext(ctx, countQuery).Scan(&oracleRowCount)
		require.NoError(t, err, "Failed to count Oracle rows")

		assert.Equal(t, pgRowCount, oracleRowCount, "Row count mismatch between Postgres and Oracle")

		// Verify data integrity - spot check first and last rows
		if len(actors) > 0 {
			var oracleActorID int
			var oracleFirstName, oracleLastName string

			// Check first actor (use uppercase table name)
			firstActor := actors[0]
			err = oraDB.QueryRowContext(ctx,
				fmt.Sprintf(`SELECT ACTOR_ID, FIRST_NAME, LAST_NAME FROM "%s" WHERE ACTOR_ID = :1`, strings.ToUpper(testTableName)),
				firstActor.ActorID).Scan(&oracleActorID, &oracleFirstName, &oracleLastName)
			require.NoError(t, err, "Failed to query first actor from Oracle")

			assert.Equal(t, firstActor.ActorID, oracleActorID, "Actor ID mismatch")
			assert.Equal(t, firstActor.FirstName, oracleFirstName, "First name mismatch")
			assert.Equal(t, firstActor.LastName, oracleLastName, "Last name mismatch")

			// Check last actor (use uppercase table name)
			lastActor := actors[len(actors)-1]
			err = oraDB.QueryRowContext(ctx,
				fmt.Sprintf(`SELECT ACTOR_ID, FIRST_NAME, LAST_NAME FROM "%s" WHERE ACTOR_ID = :1`, strings.ToUpper(testTableName)),
				lastActor.ActorID).Scan(&oracleActorID, &oracleFirstName, &oracleLastName)
			require.NoError(t, err, "Failed to query last actor from Oracle")

			assert.Equal(t, lastActor.ActorID, oracleActorID, "Actor ID mismatch")
			assert.Equal(t, lastActor.FirstName, oracleFirstName, "First name mismatch")
			assert.Equal(t, lastActor.LastName, oracleLastName, "Last name mismatch")
		}

		// Cleanup
		err = oraDrvr.DropTable(ctx, oraDB, tablefq.From(testTableName), true)
		require.NoError(t, err, "Failed to drop Oracle test table")
	})
}

// TestTableMetadata_DispatchByObjectType is a regression test covering both:
//
//   - The OWNER-column bug in getTableMetadata (USER_TABLES does not have an
//     OWNER column, so the prior query raised ORA-00904 on every base table).
//   - The dispatcher bug in grip.TableMetadata (it always called
//     getTableMetadata, so inspecting a view failed with
//     "sql: no rows in result set").
//
// It also serves as a high-level regression for the kindFromDBTypeName fix:
// every column in a SAKILA view has a parameterized type (VARCHAR2(N) etc.)
// and must resolve to a known kind rather than kind.Unknown.
//
// The test assumes the standard SAKILA fixture (table "actor", view
// "customer_list").
func TestTableMetadata_DispatchByObjectType(t *testing.T) {
	tu.SkipShort(t, true)
	th, _, _, grip, _ := testh.NewWith(t, sakila.Ora)
	ctx := th.Context

	// Oracle stores unquoted identifiers as upper case and the driver
	// returns them verbatim — no case folding on output.
	t.Run("table", func(t *testing.T) {
		md, err := grip.TableMetadata(ctx, "actor")
		require.NoError(t, err, "inspecting a base table must succeed")
		require.NotNil(t, md)
		assert.Equal(t, "ACTOR", md.Name)
		assert.Equal(t, "table", md.TableType)
		assert.Equal(t, "TABLE", md.DBTableType)
		assert.NotEmpty(t, md.Columns, "actor should have columns")

		// Regression for kindFromDBTypeName: no parameterized type should
		// classify as kind.Unknown.
		for _, col := range md.Columns {
			assert.NotEqual(t, kind.Unknown, col.Kind,
				"column %s (%s) classified as Unknown", col.Name, col.ColumnType)
		}
	})

	t.Run("view", func(t *testing.T) {
		md, err := grip.TableMetadata(ctx, "customer_list")
		require.NoError(t, err, "inspecting a view must succeed")
		require.NotNil(t, md)
		assert.Equal(t, "CUSTOMER_LIST", md.Name)
		assert.Equal(t, "view", md.TableType)
		assert.Equal(t, "VIEW", md.DBTableType)
		assert.NotEmpty(t, md.Columns)

		// customer_list is composed entirely of VARCHAR2(N) and NUMBER
		// columns; pre-fix, every VARCHAR2(N) classified as kind.Unknown.
		for _, col := range md.Columns {
			assert.NotEqual(t, kind.Unknown, col.Kind,
				"column %s (%s) classified as Unknown", col.Name, col.ColumnType)
		}
	})

	t.Run("missing object yields clean error", func(t *testing.T) {
		_, err := grip.TableMetadata(ctx, "this_object_does_not_exist")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist",
			"missing-object error should be descriptive, got: %v", err)
	})
}

// TestGetSourceMetadata_NoSchema covers the noSchema=true early-return branch
// of getSourceMetadata, which grip.SourceMetadata(noSchema=false) doesn't hit.
func TestGetSourceMetadata_NoSchema(t *testing.T) {
	tu.SkipShort(t, true)
	th, src, _, _, db := testh.NewWith(t, sakila.Ora)

	md, err := oracle.GetSourceMetadata(th.Context, src, db, true)
	require.NoError(t, err)
	require.NotNil(t, md)
	require.NotEmpty(t, md.Schema)
	require.Empty(t, md.Tables, "noSchema=true must skip table enumeration")
}

func TestDBSemver(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, src, _, grip, _ := testh.NewWith(t, sakila.Ora)
	v, err := grip.DBSemver(th.Context)
	require.NoError(t, err)
	require.True(t, semver.IsValid(v), "want canonical semver, got %q", v)
	require.NotEqual(t, "v0.0.0", v, "want a real engine version, got degenerate %q", v)

	md, err := th.SourceMetadata(src)
	require.NoError(t, err)
	require.Equal(t, v, md.DBSemver, "metadata.Source.DBSemver must match Grip.DBSemver")
}

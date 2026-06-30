package oracle_test

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // postgres driver
	_ "github.com/sijms/go-ora/v2"     // Registers database/sql driver name "oracle".
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/drivers/oracle"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

const (
	// Default test DSN for the sakiladb/oracle image.
	testDSN = "oracle://sakila:p_ssW0rd@localhost:1521/SAKILA"
)

// skipIfNoOracle skips the test if Oracle is not available.
// It provides helpful error messages for common setup issues.
func skipIfNoOracle(t *testing.T) {
	t.Helper()

	dsn := os.Getenv("SQ_TEST_ORACLE_DSN")
	if dsn == "" {
		dsn = testDSN
	}
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		t.Skipf("Oracle driver open failed: %v", err)
		return
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = db.PingContext(pingCtx); err != nil {
		errMsg := err.Error()

		// Check for connection refused (Oracle database not running).
		if strings.Contains(errMsg, "connection refused") ||
			strings.Contains(errMsg, "ORA-12541") ||
			strings.Contains(errMsg, "no such host") ||
			strings.Contains(errMsg, "i/o timeout") {
			t.Skip(`Oracle database not reachable.

Start a local instance:
  docker run -d -p 1521:1521 sakiladb/oracle:latest

Then set SQ_TEST_ORACLE_DSN if not using the default DSN. See
drivers/oracle/README.md.`)
			return
		}

		t.Skipf("Oracle not available: %v", err)
		return
	}
}

// getTestSource returns a test source for Oracle.
func getTestSource(t *testing.T) *source.Source {
	t.Helper()

	dsn := os.Getenv("SQ_TEST_ORACLE_DSN")
	if dsn == "" {
		dsn = testDSN
	}

	return &source.Source{
		Handle:   "@test_oracle",
		Type:     drivertype.Oracle,
		Location: dsn,
		Options:  options.Options{},
	}
}

// TestSmoke is a basic smoke test to verify the driver works.
func TestSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoOracle(t)

	ctx := context.Background()
	log := slog.Default()
	ctx = lg.NewContext(ctx, log)

	provider := &oracle.Provider{Log: log}
	drvr, err := provider.DriverFor(drivertype.Oracle)
	require.NoError(t, err)
	require.NotNil(t, drvr)

	src := getTestSource(t)

	// Test Ping
	err = drvr.Ping(ctx, src, driver.ModeReadWrite)
	require.NoError(t, err, "Ping should succeed")

	// Test Open
	grip, err := drvr.Open(ctx, src, driver.ModeReadWrite)
	require.NoError(t, err, "Open should succeed")
	require.NotNil(t, grip)
	defer grip.Close()

	db, err := grip.DB(ctx)
	require.NoError(t, err)
	require.NotNil(t, db)

	// Test a simple query
	var result int
	err = db.QueryRowContext(ctx, "SELECT 1 FROM DUAL").Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, 1, result)

	log.Info("Smoke test passed", lga.Src, src)
}

// TestOracle_DBProperties_BestEffort verifies DBProperties returns core fields
// even when v$instance is not readable (version may come from v$version).
func TestOracle_DBProperties_BestEffort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoOracle(t)

	ctx := context.Background()
	log := slog.Default()
	ctx = lg.NewContext(ctx, log)

	provider := &oracle.Provider{Log: log}
	drvr, err := provider.DriverFor(drivertype.Oracle)
	require.NoError(t, err)

	sqlDrvr, ok := drvr.(driver.SQLDriver)
	require.True(t, ok, "driver should implement driver.SQLDriver")

	src := getTestSource(t)
	grip, err := drvr.Open(ctx, src, driver.ModeReadWrite)
	require.NoError(t, err)
	defer grip.Close()

	db, err := grip.DB(ctx)
	require.NoError(t, err)

	props, err := sqlDrvr.DBProperties(ctx, db)
	require.NoError(t, err)
	require.NotEmpty(t, props["db_name"])
	require.NotEmpty(t, props["current_schema"])
	if v, ok := props["version"].(string); ok {
		require.NotEmpty(t, v)
	}
}

// TestCurrentSchema tests the CurrentSchema method.
func TestCurrentSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoOracle(t)

	ctx := context.Background()
	log := slog.Default()
	ctx = lg.NewContext(ctx, log)

	provider := &oracle.Provider{Log: log}
	drvr, err := provider.DriverFor(drivertype.Oracle)
	require.NoError(t, err)

	sqlDrvr, ok := drvr.(driver.SQLDriver)
	require.True(t, ok, "driver should implement driver.SQLDriver")
	src := getTestSource(t)

	grip, err := drvr.Open(ctx, src, driver.ModeReadWrite)
	require.NoError(t, err)
	defer grip.Close()

	db, err := grip.DB(ctx)
	require.NoError(t, err)

	schemaName, err := sqlDrvr.CurrentSchema(ctx, db)
	require.NoError(t, err)
	assert.NotEmpty(t, schemaName)

	log.Info("Current schema", lga.Schema, schemaName)
}

// TestCreateAndDropTable tests table creation and deletion.
func TestCreateAndDropTable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoOracle(t)

	ctx := context.Background()
	log := slog.Default()
	ctx = lg.NewContext(ctx, log)

	provider := &oracle.Provider{Log: log}
	drvr, err := provider.DriverFor(drivertype.Oracle)
	require.NoError(t, err)

	sqlDrvr, ok := drvr.(driver.SQLDriver)
	require.True(t, ok, "driver should implement driver.SQLDriver")
	src := getTestSource(t)

	grip, err := drvr.Open(ctx, src, driver.ModeReadWrite)
	require.NoError(t, err)
	defer grip.Close()

	db, err := grip.DB(ctx)
	require.NoError(t, err)

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

	err = sqlDrvr.CreateTable(ctx, db, tblDef)
	require.NoError(t, err, "CreateTable should succeed")

	// Verify table exists
	exists, err := sqlDrvr.TableExists(ctx, db, tblName)
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
	err = sqlDrvr.DropTable(ctx, db, tablefq.From(tblName), false)
	require.NoError(t, err, "DropTable should succeed")

	// Verify table no longer exists
	exists, err = sqlDrvr.TableExists(ctx, db, tblName)
	require.NoError(t, err)
	assert.False(t, exists, "Table should not exist after drop")

	log.Info("CreateAndDropTable test passed")
}

// TestTypeMappings tests various Oracle data types.
func TestTypeMappings(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoOracle(t)

	ctx := context.Background()
	log := slog.Default()
	ctx = lg.NewContext(ctx, log)

	provider := &oracle.Provider{Log: log}
	drvr, err := provider.DriverFor(drivertype.Oracle)
	require.NoError(t, err)

	sqlDrvr, ok := drvr.(driver.SQLDriver)
	require.True(t, ok, "driver should implement driver.SQLDriver")
	src := getTestSource(t)

	grip, err := drvr.Open(ctx, src, driver.ModeReadWrite)
	require.NoError(t, err)
	defer grip.Close()

	db, err := grip.DB(ctx)
	require.NoError(t, err)

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

	err = sqlDrvr.CreateTable(ctx, db, tblDef)
	require.NoError(t, err)
	defer func() { _ = sqlDrvr.DropTable(ctx, db, tablefq.From(tblName), true) }()

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

	values := make([]interface{}, 8)
	for i := range values {
		values[i] = new(interface{})
	}

	err = rows.Scan(values...)
	require.NoError(t, err)

	log.Info("TypeMappings test passed")
}

// TestListTables tests listing tables.
func TestListTables(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoOracle(t)

	ctx := context.Background()
	log := slog.Default()
	ctx = lg.NewContext(ctx, log)

	provider := &oracle.Provider{Log: log}
	drvr, err := provider.DriverFor(drivertype.Oracle)
	require.NoError(t, err)

	sqlDrvr, ok := drvr.(driver.SQLDriver)
	require.True(t, ok, "driver should implement driver.SQLDriver")
	src := getTestSource(t)

	grip, err := drvr.Open(ctx, src, driver.ModeReadWrite)
	require.NoError(t, err)
	defer grip.Close()

	db, err := grip.DB(ctx)
	require.NoError(t, err)

	// List tables
	tables, err := sqlDrvr.ListTableNames(ctx, db, "", true, false)
	require.NoError(t, err)
	assert.NotNil(t, tables)

	log.Info("Listed tables", lga.Count, len(tables))
}

// skipIfNoPostgres skips the test if Postgres is not available.
func skipIfNoPostgres(t *testing.T) {
	t.Helper()

	dsn := os.Getenv("SQ_TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://testuser:testpass@localhost:5432/sakila?sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skipf("Postgres not available: %v", err)
		return
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		t.Skipf("Postgres not available: %v", err)
		return
	}
}

// TestSakilaCrossDatabase tests reading data from Postgres and writing to Oracle.
// This is a real-world integration test demonstrating cross-database data migration.
func TestSakilaCrossDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoOracle(t)
	skipIfNoPostgres(t)

	ctx := context.Background()
	log := slog.Default()
	ctx = lg.NewContext(ctx, log)

	// Setup Postgres source
	pgDSN := os.Getenv("SQ_TEST_POSTGRES_DSN")
	if pgDSN == "" {
		pgDSN = "postgres://testuser:testpass@localhost:5432/sakila?sslmode=disable"
	}

	pgSrc := &source.Source{
		Handle:   "@test_postgres",
		Type:     drivertype.Pg,
		Location: pgDSN,
		Options:  options.Options{},
	}

	// Setup Oracle source
	oracleSrc := getTestSource(t)

	// Connect to Postgres
	pgDB, err := sql.Open("pgx", pgDSN)
	require.NoError(t, err, "Failed to open Postgres connection")
	defer pgDB.Close()

	err = pgDB.PingContext(ctx)
	require.NoError(t, err, "Failed to ping Postgres")

	// Get Oracle driver and connect
	oracleProvider := &oracle.Provider{Log: log}
	oracleDrvr, err := oracleProvider.DriverFor(drivertype.Oracle)
	require.NoError(t, err)

	oracleSQLDrvr, ok := oracleDrvr.(driver.SQLDriver)
	require.True(t, ok, "Oracle driver should implement driver.SQLDriver")

	oracleGrip, err := oracleDrvr.Open(ctx, oracleSrc, driver.ModeReadWrite)
	require.NoError(t, err)
	defer oracleGrip.Close()

	oracleDB, err := oracleGrip.DB(ctx)
	require.NoError(t, err)

	log.Info("Connected to both databases", "postgres", pgSrc.Handle, "oracle", oracleSrc.Handle)

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
		log.Info("Read actors from Postgres", lga.Count, pgRowCount)

		// Create table in Oracle
		tblDef := &schema.Table{
			Name: testTableName,
			Cols: []*schema.Column{
				{Name: "ACTOR_ID", Kind: kind.Int, NotNull: true},
				{Name: "FIRST_NAME", Kind: kind.Text, NotNull: true},
				{Name: "LAST_NAME", Kind: kind.Text, NotNull: true},
			},
		}

		err = oracleSQLDrvr.CreateTable(ctx, oracleDB, tblDef)
		require.NoError(t, err, "Failed to create actor table in Oracle")
		log.Info("Created table in Oracle", "table", testTableName)

		// Insert data into Oracle (use uppercase table name)
		insertSQL := fmt.Sprintf(
			`INSERT INTO "%s" (ACTOR_ID, FIRST_NAME, LAST_NAME) VALUES (:1, :2, :3)`,
			strings.ToUpper(testTableName),
		)
		stmt, err := oracleDB.PrepareContext(ctx, insertSQL)
		require.NoError(t, err, "Failed to prepare insert statement")
		defer stmt.Close()

		for _, actor := range actors {
			_, err := stmt.ExecContext(ctx, actor.ActorID, actor.FirstName, actor.LastName)
			require.NoError(t, err, "Failed to insert actor %d", actor.ActorID)
		}
		log.Info("Inserted rows into Oracle", lga.Count, len(actors))

		// Verify row count in Oracle (use uppercase table name)
		var oracleRowCount int
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, strings.ToUpper(testTableName))
		err = oracleDB.QueryRowContext(ctx, countQuery).Scan(&oracleRowCount)
		require.NoError(t, err, "Failed to count Oracle rows")

		assert.Equal(t, pgRowCount, oracleRowCount, "Row count mismatch between Postgres and Oracle")
		log.Info("Row count verification passed", "postgres", pgRowCount, "oracle", oracleRowCount)

		// Verify data integrity - spot check first and last rows
		if len(actors) > 0 {
			var oracleActorID int
			var oracleFirstName, oracleLastName string

			// Check first actor (use uppercase table name)
			firstActor := actors[0]
			err = oracleDB.QueryRowContext(ctx,
				fmt.Sprintf(`SELECT ACTOR_ID, FIRST_NAME, LAST_NAME FROM "%s" WHERE ACTOR_ID = :1`, strings.ToUpper(testTableName)),
				firstActor.ActorID).Scan(&oracleActorID, &oracleFirstName, &oracleLastName)
			require.NoError(t, err, "Failed to query first actor from Oracle")

			assert.Equal(t, firstActor.ActorID, oracleActorID, "Actor ID mismatch")
			assert.Equal(t, firstActor.FirstName, oracleFirstName, "First name mismatch")
			assert.Equal(t, firstActor.LastName, oracleLastName, "Last name mismatch")

			// Check last actor (use uppercase table name)
			lastActor := actors[len(actors)-1]
			err = oracleDB.QueryRowContext(ctx,
				fmt.Sprintf(`SELECT ACTOR_ID, FIRST_NAME, LAST_NAME FROM "%s" WHERE ACTOR_ID = :1`, strings.ToUpper(testTableName)),
				lastActor.ActorID).Scan(&oracleActorID, &oracleFirstName, &oracleLastName)
			require.NoError(t, err, "Failed to query last actor from Oracle")

			assert.Equal(t, lastActor.ActorID, oracleActorID, "Actor ID mismatch")
			assert.Equal(t, lastActor.FirstName, oracleFirstName, "First name mismatch")
			assert.Equal(t, lastActor.LastName, oracleLastName, "Last name mismatch")

			log.Info("Data integrity verification passed")
		}

		// Cleanup
		err = oracleSQLDrvr.DropTable(ctx, oracleDB, tablefq.From(testTableName), true)
		require.NoError(t, err, "Failed to drop Oracle test table")
		log.Info("Cleaned up Oracle test table", "table", testTableName)
	})

	log.Info("Cross-database test completed successfully")
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
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfNoOracle(t)

	ctx := context.Background()
	log := slog.Default()
	ctx = lg.NewContext(ctx, log)

	provider := &oracle.Provider{Log: log}
	drvr, err := provider.DriverFor(drivertype.Oracle)
	require.NoError(t, err)

	src := getTestSource(t)
	grip, err := drvr.Open(ctx, src, driver.ModeReadWrite)
	require.NoError(t, err)
	defer grip.Close()

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

func TestDBSemver(t *testing.T) {
	t.Parallel()
	th, _, _, grip, _ := testh.NewWith(t, sakila.Ora)
	v, err := grip.DBSemver(th.Context)
	require.NoError(t, err)
	require.True(t, semver.IsValid(v), "want canonical semver, got %q", v)
}

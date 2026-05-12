package duckdb_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
)

// newImplTestDB opens a fresh in-file DuckDB, wires it as an sq source, and
// returns the raw *sql.DB plus an sq Helper for higher-level operations.
// The returned db and source are cleaned up automatically via t.Cleanup.
func newImplTestDB(t *testing.T) (*sql.DB, *testh.Helper, *source.Source) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "impl_test.duckdb")

	db, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	th := testh.New(t)
	src := &source.Source{
		Handle:   "@impl_test",
		Type:     drivertype.DuckDB,
		Location: "duckdb://" + dbPath,
	}
	th.Add(src)
	return db, th, src
}

// TestPing verifies that Ping succeeds against a freshly opened DuckDB source.
func TestPing(t *testing.T) {
	_, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	require.NoError(t, drvr.Ping(th.Context, src))
}

// TestValidateSource verifies that ValidateSource returns the same source
// for a DuckDB source and returns an error for a non-DuckDB type.
func TestValidateSource(t *testing.T) {
	_, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	got, err := drvr.ValidateSource(src)
	require.NoError(t, err)
	require.Equal(t, src, got)

	bad := &source.Source{Handle: "@bad", Type: drivertype.SQLite, Location: "sqlite3:///x.db"}
	_, err = drvr.ValidateSource(bad)
	require.Error(t, err)
}

// TestCreateTable_BasicTypes creates a table with one column per kind and
// verifies that TableMetadata reflects the expected schema.
func TestCreateTable_BasicTypes(t *testing.T) {
	db, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	tblDef := &schema.Table{
		Name: "tbl_kinds",
		Cols: []*schema.Column{
			{Name: "c_text", Kind: kind.Text},
			{Name: "c_int", Kind: kind.Int},
			{Name: "c_float", Kind: kind.Float},
			{Name: "c_decimal", Kind: kind.Decimal},
			{Name: "c_bool", Kind: kind.Bool},
			{Name: "c_datetime", Kind: kind.Datetime},
			{Name: "c_date", Kind: kind.Date},
			{Name: "c_time", Kind: kind.Time},
			{Name: "c_bytes", Kind: kind.Bytes},
		},
	}

	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	// Table must be queryable.
	rows, err := db.QueryContext(th.Context, `SELECT * FROM "tbl_kinds" WHERE 0=1`)
	require.NoError(t, err)
	cols, err := rows.Columns()
	require.NoError(t, err)
	require.NoError(t, rows.Close())

	require.Equal(t, len(tblDef.Cols), len(cols))
	for i, col := range tblDef.Cols {
		require.Equal(t, col.Name, cols[i])
	}
}

// TestTableColumnTypes verifies that TableColumnTypes returns column metadata
// for a known table.
func TestTableColumnTypes(t *testing.T) {
	db, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	ctx := th.Context
	_, err := db.ExecContext(ctx, `CREATE TABLE "ct_tbl" (id BIGINT, name VARCHAR, score DOUBLE)`)
	require.NoError(t, err)

	colTypes, err := drvr.TableColumnTypes(ctx, db, "ct_tbl", nil)
	require.NoError(t, err)
	require.Len(t, colTypes, 3)

	// With explicit column subset.
	colTypes2, err := drvr.TableColumnTypes(ctx, db, "ct_tbl", []string{"name", "score"})
	require.NoError(t, err)
	require.Len(t, colTypes2, 2)
	require.Equal(t, "name", colTypes2[0].Name())
}

// TestDropTable creates a table and then drops it, verifying it is gone.
func TestDropTable(t *testing.T) {
	db, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	ctx := th.Context
	_, err := db.ExecContext(ctx, `CREATE TABLE "drop_me" (id BIGINT)`)
	require.NoError(t, err)

	tbl := tablefq.New("drop_me")
	require.NoError(t, drvr.DropTable(ctx, db, tbl, false))

	exists, err := drvr.TableExists(ctx, db, "drop_me")
	require.NoError(t, err)
	require.False(t, exists)

	// DROP TABLE IF EXISTS on a non-existent table must not error.
	require.NoError(t, drvr.DropTable(ctx, db, tbl, true))

	// DROP TABLE (without IF EXISTS) on a non-existent table must error.
	require.Error(t, drvr.DropTable(ctx, db, tbl, false))
}

// TestCopyTable_DataAndSchema copies a table both with and without data and
// verifies row counts.
func TestCopyTable_DataAndSchema(t *testing.T) {
	db, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	ctx := th.Context
	_, err := db.ExecContext(ctx, `CREATE TABLE "orig" (id BIGINT, name VARCHAR)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO "orig" VALUES (1,'a'), (2,'b'), (3,'c')`)
	require.NoError(t, err)

	from := tablefq.New("orig")

	// Schema-only copy.
	to1 := tablefq.New("copy_no_data")
	affected, err := drvr.CopyTable(ctx, db, from, to1, false)
	require.NoError(t, err)
	require.Equal(t, int64(0), affected)

	var cnt int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM "copy_no_data"`).Scan(&cnt))
	require.Equal(t, int64(0), cnt)

	// Data copy.
	to2 := tablefq.New("copy_with_data")
	affected, err = drvr.CopyTable(ctx, db, from, to2, true)
	require.NoError(t, err)
	_ = affected // DuckDB CREATE TABLE AS may return -1 for affected rows

	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM "copy_with_data"`).Scan(&cnt))
	require.Equal(t, int64(3), cnt)
}

// TestTruncate inserts rows, truncates the table, and verifies the count is 0.
func TestTruncate(t *testing.T) {
	_, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	ctx := th.Context
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `CREATE TABLE "trunc_tbl" (id BIGINT)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO "trunc_tbl" VALUES (1),(2),(3),(4),(5)`)
	require.NoError(t, err)

	affected, err := drvr.Truncate(ctx, src, "trunc_tbl", false)
	require.NoError(t, err)
	require.Equal(t, int64(5), affected)

	var cnt int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM "trunc_tbl"`).Scan(&cnt))
	require.Equal(t, int64(0), cnt)
}

// TestCreateSchema_DropSchema creates a schema and then drops it.
func TestCreateSchema_DropSchema(t *testing.T) {
	db, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	ctx := th.Context

	const schemaName = "test_schema"
	require.NoError(t, drvr.CreateSchema(ctx, db, schemaName))

	exists, err := drvr.SchemaExists(ctx, db, schemaName)
	require.NoError(t, err)
	require.True(t, exists)

	require.NoError(t, drvr.DropSchema(ctx, db, schemaName))

	exists, err = drvr.SchemaExists(ctx, db, schemaName)
	require.NoError(t, err)
	require.False(t, exists)
}

// TestCatalogExists verifies that CatalogExists returns true for the current
// database and false for an unknown catalog.
func TestCatalogExists(t *testing.T) {
	db, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	ctx := th.Context

	catalog, err := drvr.CurrentCatalog(ctx, db)
	require.NoError(t, err)
	require.NotEmpty(t, catalog)

	exists, err := drvr.CatalogExists(ctx, db, catalog)
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = drvr.CatalogExists(ctx, db, "definitely_does_not_exist_xyz")
	require.NoError(t, err)
	require.False(t, exists)

	// Empty string must return false without error.
	exists, err = drvr.CatalogExists(ctx, db, "")
	require.NoError(t, err)
	require.False(t, exists)
}

// TestDBProperties verifies that DBProperties returns a non-empty map
// and that a known DuckDB setting is present.
func TestDBProperties(t *testing.T) {
	db, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	ctx := th.Context

	props, err := drvr.DBProperties(ctx, db)
	require.NoError(t, err)
	require.NotEmpty(t, props)

	// "threads" is always present in DuckDB's settings.
	_, ok := props["threads"]
	require.True(t, ok, "expected 'threads' in DBProperties result")
}

// TestNewBatchInsert_RoundTrip bulk-inserts N rows into a table and verifies
// that exactly N rows are queryable afterwards.
func TestNewBatchInsert_RoundTrip(t *testing.T) {
	db, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	ctx := th.Context

	const tblName = "batch_tbl"
	_, err := db.ExecContext(ctx, `CREATE TABLE "batch_tbl" (id BIGINT, name VARCHAR)`)
	require.NoError(t, err)

	// Batch insert requires a single connection.
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()

	const numRows = 250
	colNames := []string{"id", "name"}

	bi, err := drvr.NewBatchInsert(ctx, "test batch insert", conn, src, tblName, colNames)
	require.NoError(t, err)

	for i := range numRows {
		rec := []any{int64(i + 1), "row"}
		require.NoError(t, bi.Munge(rec))
		bi.RecordCh <- rec
	}
	close(bi.RecordCh)

	// Wait for the goroutine to finish.
	for err2 := range bi.ErrCh {
		require.NoError(t, err2)
	}

	require.Equal(t, int64(numRows), bi.Written())

	var cnt int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM "batch_tbl"`).Scan(&cnt))
	require.Equal(t, int64(numRows), cnt)
}

// TestPrepareInsertStmt verifies that PrepareInsertStmt builds a valid
// prepared statement that can execute without error.
func TestPrepareInsertStmt(t *testing.T) {
	db, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	ctx := th.Context

	_, err := db.ExecContext(ctx, `CREATE TABLE "ins_tbl" (id BIGINT, name VARCHAR)`)
	require.NoError(t, err)

	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()

	execer, err := drvr.PrepareInsertStmt(ctx, conn, "ins_tbl", []string{"id", "name"}, 1)
	require.NoError(t, err)
	require.NotNil(t, execer)
	defer execer.Close()

	rec := []any{int64(1), "alice"}
	require.NoError(t, execer.Munge(rec))
	affected, err := execer.Exec(ctx, rec...)
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	var name string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT name FROM "ins_tbl" WHERE id = 1`).Scan(&name))
	require.Equal(t, "alice", name)
}

// TestPrepareUpdateStmt verifies that PrepareUpdateStmt produces a working
// UPDATE prepared statement.
func TestPrepareUpdateStmt(t *testing.T) {
	db, th, src := newImplTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	ctx := th.Context

	_, err := db.ExecContext(ctx, `CREATE TABLE "upd_tbl" (id BIGINT, name VARCHAR)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO "upd_tbl" VALUES (1, 'alice')`)
	require.NoError(t, err)

	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()

	execer, err := drvr.PrepareUpdateStmt(ctx, conn, "upd_tbl", []string{"name"}, `id = 1`)
	require.NoError(t, err)
	require.NotNil(t, execer)
	defer execer.Close()

	rec := []any{"bob"}
	require.NoError(t, execer.Munge(rec))
	_, err = execer.Exec(ctx, rec...)
	require.NoError(t, err)

	var name string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT name FROM "upd_tbl" WHERE id = 1`).Scan(&name))
	require.Equal(t, "bob", name)
}

// Compile-time check: sqlz.DB is satisfied by *sql.Conn.
var _ sqlz.DB = (*sql.Conn)(nil)

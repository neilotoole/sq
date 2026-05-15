package duckdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestSourceMetadata_Sakila verifies that SourceMetadata returns valid metadata
// for the sakila DuckDB fixture.
func TestSourceMetadata_Sakila(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)

	grip := th.Open(src)

	md, err := grip.SourceMetadata(context.Background(), false)
	require.NoError(t, err)
	require.Contains(t, md.DBProduct, "DuckDB")
	require.NotEmpty(t, md.DBVersion)
	require.NotEmpty(t, md.Tables)

	tableNames := make([]string, len(md.Tables))
	for i, tbl := range md.Tables {
		tableNames[i] = tbl.Name
	}
	require.Contains(t, tableNames, "actor")
	require.Contains(t, tableNames, "film")
}

// TestTableMetadata_Actor verifies that TableMetadata returns correct column
// metadata for the sakila "actor" table.
func TestTableMetadata_Actor(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)

	grip := th.Open(src)

	tblMeta, err := grip.TableMetadata(context.Background(), "actor")
	require.NoError(t, err)
	require.Equal(t, "actor", tblMeta.Name)
	require.NotEmpty(t, tblMeta.Columns)

	colNames := make([]string, len(tblMeta.Columns))
	for i, c := range tblMeta.Columns {
		colNames[i] = c.Name
	}
	require.Contains(t, colNames, "actor_id")
	require.Contains(t, colNames, "first_name")
	require.Contains(t, colNames, "last_name")
	require.Contains(t, colNames, "last_update")
}

// TestSakilaFixture_ForeignKeyCount pins the bundled sakila.duckdb
// fixture to 21 FK constraints (22 in the source minus the
// fk_store_staff cycle-breaker). Detects regressions in the portsakila
// tool that produce a fixture with the wrong FK count — those wouldn't
// surface in unit tests since the fixture is regenerated via a separate
// `go run ./.../portsakila` invocation.
func TestSakilaFixture_ForeignKeyCount(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	grip := th.Open(src)
	db, err := grip.DB(context.Background())
	require.NoError(t, err)

	var got int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM duckdb_constraints() WHERE constraint_type = 'FOREIGN KEY'`,
	).Scan(&got))
	require.Equal(t, 21, got,
		"sakila.duckdb must have 21 of 22 FKs preserved (only fk_store_staff is stripped)")
}

// TestTableMetadata_PrimaryKey asserts the contract of stmtPrimaryKeys (in
// metadata.go): it uses UNNEST so each PK column name is returned as its
// own row. A string-split implementation would pass simple identifiers
// but mis-split column names containing a comma or space.
//
// The single_pk + composite_pk subtests cover the basic shape (simple
// identifiers, single + multi-column). The whitespace_identifier subtest
// builds an in-memory DB with a PK column name containing a space and a
// comma, which is what UNNEST is actually load-bearing for.
func TestTableMetadata_PrimaryKey(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	grip := th.Open(src)
	ctx := context.Background()

	t.Run("single_pk", func(t *testing.T) {
		md, err := grip.TableMetadata(ctx, "actor")
		require.NoError(t, err)
		pkCols := pkColumnNames(md.Columns)
		require.Equal(t, []string{"actor_id"}, pkCols)
	})

	t.Run("composite_pk", func(t *testing.T) {
		md, err := grip.TableMetadata(ctx, "film_actor")
		require.NoError(t, err)
		pkCols := pkColumnNames(md.Columns)
		require.ElementsMatch(t, []string{"actor_id", "film_id"}, pkCols)
	})

	t.Run("whitespace_identifier", func(t *testing.T) {
		// Open a separate :memory: source so we don't pollute the shared
		// sakila fixture with an ad-hoc table.
		memSrc := &source.Source{
			Handle:   "@pk_whitespace",
			Type:     drivertype.DuckDB,
			Location: "duckdb://:memory:",
		}
		th.Add(memSrc)
		memGrip := th.Open(memSrc)
		memDB, err := memGrip.DB(ctx)
		require.NoError(t, err)

		// "first, last" contains both a comma and a space — exactly what a
		// string-split implementation would mishandle.
		_, err = memDB.ExecContext(ctx,
			`CREATE TABLE t ("first, last" VARCHAR, "age" INTEGER, PRIMARY KEY ("first, last"))`)
		require.NoError(t, err)

		md, err := memGrip.TableMetadata(ctx, "t")
		require.NoError(t, err)
		pkCols := pkColumnNames(md.Columns)
		require.Equal(t, []string{"first, last"}, pkCols)
	})
}

// TestSourceMetadata_Misc verifies multi-schema enumeration against the
// testdata/misc.duckdb fixture (foo, bar schemas).
func TestSourceMetadata_Misc(t *testing.T) {
	th := testh.New(t)
	src := th.Source("@miscdb_duck")
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	ctx := context.Background()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	schemas, err := drvr.ListSchemas(ctx, db)
	require.NoError(t, err)
	require.Subset(t, schemas, []string{"foo", "bar"})

	fooTables, err := drvr.ListTableNames(ctx, db, "foo", true, true)
	require.NoError(t, err)
	require.Equal(t, []string{"t1"}, fooTables)

	barTables, err := drvr.ListTableNames(ctx, db, "bar", true, true)
	require.NoError(t, err)
	require.Equal(t, []string{"t2"}, barTables)
}

// TestSourceMetadata_Empty verifies that a DuckDB source with no user
// tables surfaces sensible metadata: catalog/schema set, table/view
// counts zero.
func TestSourceMetadata_Empty(t *testing.T) {
	th := testh.New(t)
	src := th.Source("@emptydb_duck")
	grip := th.Open(src)

	md, err := grip.SourceMetadata(context.Background(), false)
	require.NoError(t, err)
	require.NotEmpty(t, md.Name)
	require.NotEmpty(t, md.Schema)
	require.Zero(t, md.TableCount)
	require.Zero(t, md.ViewCount)
	require.Empty(t, md.Tables)
}

// TestRecordMeta_BlobScan verifies BLOB and NULL-BLOB rows scan cleanly
// through the type munge against the testdata/blob.duckdb fixture.
func TestRecordMeta_BlobScan(t *testing.T) {
	th := testh.New(t)
	src := th.Source("@blobdb_duck")
	grip := th.Open(src)

	ctx := context.Background()
	db, err := grip.DB(ctx)
	require.NoError(t, err)

	rows, err := db.QueryContext(ctx, `SELECT id, data FROM blobs ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	require.NoError(t, err)
	recMeta, newRecFn, err := grip.SQLDriver().RecordMeta(ctx, colTypes)
	require.NoError(t, err)
	require.Equal(t, kind.Bytes, recMeta[1].Kind())

	var recs []record.Record
	for rows.Next() {
		scanRow := recMeta.NewScanRow()
		require.NoError(t, rows.Scan(scanRow...))
		rec, err := newRecFn(scanRow)
		require.NoError(t, err)
		recs = append(recs, rec)
	}
	require.NoError(t, rows.Err())
	require.Len(t, recs, 2)

	// Row 1: BLOB \x00\x01\x02\x03
	_, ok := recs[0][1].([]byte)
	require.True(t, ok, "row 1 BLOB should scan as []byte, got %T", recs[0][1])
	require.Equal(t, []byte{0x00, 0x01, 0x02, 0x03}, recs[0][1])

	// Row 2: NULL BLOB.
	require.Nil(t, recs[1][1])
}

// pkColumnNames returns the names of columns where PrimaryKey is true.
func pkColumnNames(cols []*metadata.Column) []string {
	var names []string
	for _, c := range cols {
		if c.PrimaryKey {
			names = append(names, c.Name)
		}
	}
	return names
}

// TestRecordMeta_BasicQuery verifies that RecordMeta correctly maps a
// simple query's column types to record.Meta with the right kinds.
func TestRecordMeta_BasicQuery(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	grip := th.Open(src)

	db, err := grip.DB(context.Background())
	require.NoError(t, err)

	rows, err := db.QueryContext(context.Background(),
		`SELECT actor_id, first_name, last_name, last_update FROM actor LIMIT 1`)
	require.NoError(t, err)
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	require.NoError(t, err)

	recMeta, newRecFn, err := grip.SQLDriver().RecordMeta(context.Background(), colTypes)
	require.NoError(t, err)
	require.NotNil(t, newRecFn)
	require.Len(t, recMeta, 4)

	require.Equal(t, "actor_id", recMeta[0].Name())
	require.Equal(t, kind.Int, recMeta[0].Kind())
	require.Equal(t, kind.Text, recMeta[1].Kind())     // first_name VARCHAR
	require.Equal(t, kind.Text, recMeta[2].Kind())     // last_name VARCHAR
	require.Equal(t, kind.Datetime, recMeta[3].Kind()) // last_update TIMESTAMP

	// Verify the munge function produces a valid record.
	require.True(t, rows.Next())
	scanRow := recMeta.NewScanRow()
	require.NoError(t, rows.Scan(scanRow...))
	rec, err := newRecFn(scanRow)
	require.NoError(t, err)
	require.Len(t, rec, 4)
	// actor_id should be a non-nil int64.
	require.NotNil(t, rec[0])
	_, ok := rec[0].(int64)
	require.True(t, ok, "actor_id should be int64, got %T", rec[0])
}

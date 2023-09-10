package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/neilotoole/sq/libsq/core/lg"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestSmoke(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.PgAll() {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, _, _, _ := testh.NewWith(t, handle)
			sink, err := th.QuerySQL(src, "SELECT * FROM actor")
			require.NoError(t, err)
			require.Equal(t, len(sakila.TblActorCols()), len(sink.RecMeta))
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

func TestDriverBehavior(t *testing.T) {
	// This test was created to help understand the behavior of the driver impl.
	// It can be deleted eventually.
	testCases := sakila.PgAll()

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(handle)
			db := th.OpenDB(src)

			query := `SELECT
       (SELECT actor_id FROM actor limit 1) AS actor_id,
       (SELECT first_name FROM actor LIMIT 1) AS first_name,
       (SELECT last_name FROM actor LIMIT 1) AS last_name
LIMIT 1`

			rows, err := db.QueryContext(th.Context, query)
			require.NoError(t, err)
			require.NoError(t, rows.Err())
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
		})
	}
}

func Test_VerifyDriverDoesNotReportNullability(t *testing.T) {
	t.Parallel()
	// This test demonstrates that the backing pgx driver
	// does not report column nullability (as one might hope).
	//
	// When/if the driver is modified to behave as hoped (if
	// at all possible) then we can simplify the
	// postgres driver wrapper.
	testCases := sakila.PgAll()
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			db := th.OpenDB(src)

			_, actualTblName := createTypeTestTable(th, src, true)
			t.Cleanup(func() { th.DropTable(src, actualTblName) })

			rows, err := db.Query("SELECT * FROM " + actualTblName)
			require.NoError(t, err)
			require.NoError(t, rows.Err())
			t.Cleanup(func() { assert.NoError(t, rows.Close()) })

			colTypes, err := rows.ColumnTypes()
			require.NoError(t, err)

			for _, colType := range colTypes {
				colName := colType.Name()

				// The _n suffix indicates a nullable col
				if !strings.HasSuffix(colName, "_n") {
					continue
				}

				// The col is indicated as nullable via its name/suffix
				nullable, hasNullable := colType.Nullable()
				require.False(t, hasNullable, "ColumnType.hasNullable is unfortunately expected to be false for {%s}",
					colName)
				require.False(t, nullable, "ColumnType.nullable is unfortunately expected to be false for {%s}", colName)
			}

			for rows.Next() {
				require.NoError(t, rows.Err())
			}
		})
	}
}

func TestGetTableColumnNames(t *testing.T) {
	testCases := sakila.PgAll()

	for _, handle := range testCases {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			th, _, _, _, db := testh.NewWith(t, handle)
			colNames, err := postgres.GetTableColumnNames(th.Context, db, sakila.TblActor)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCols(), colNames)
		})
	}
}

func TestDriver_CreateTable_NotNullDefault(t *testing.T) {
	t.Parallel()

	testCases := sakila.PgAll()
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, handle)

			tblName := stringz.UniqTableName(t.Name())
			colNames, colKinds := fixt.ColNamePerKind(drvr.Dialect().IntBool, false, false)

			tblDef := sqlmodel.NewTableDef(tblName, colNames, colKinds)
			for _, colDef := range tblDef.Cols {
				colDef.NotNull = true
				colDef.HasDefault = true
			}

			err := drvr.CreateTable(th.Context, db, tblDef)
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tblName) })

			th.InsertDefaultRow(src, tblName)

			sink, err := th.QuerySQL(src, "SELECT * FROM "+tblName)
			require.NoError(t, err)
			require.Equal(t, 1, len(sink.Recs))
			require.Equal(t, len(colNames), len(sink.RecMeta))
			for i := range colNames {
				require.NotNil(t, sink.Recs[0][i])
				_, ok := sink.RecMeta[i].Nullable()
				require.False(t, ok, "postgres driver doesn't report nullability")
			}
		})
	}
}

// TestAlternateSchema verifies that we can access a schema
// other than the default ("public").
func TestAlternateSchema(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	ctx := th.Context

	src := th.Source(sakila.Pg)
	t.Logf("Using src: {%s}", src)

	db := th.OpenDB(src)
	require.NoError(t, db.Ping())

	schemaName := stringz.UniqSuffix("test_schema")
	err := createSchema(ctx, db, schemaName)
	t.Logf("Created schema {%s} in {%s}", schemaName, src)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, dropSchema(ctx, db, schemaName))
	})

	tblName := stringz.UniqSuffix("test_table")
	const wantRowCount = 5
	require.NoError(t, createSimpleTable(ctx, db, schemaName, tblName, wantRowCount))

	// We create a new src to point to the new schema.
	// We change the schema by setting the search_path param.
	src2 := src.Clone()
	src2.Handle += "2"
	src2.Location += "?search_path=" + schemaName
	dbase2 := th.Open(src2)
	md2, err := dbase2.SourceMetadata(ctx, false)
	require.NoError(t, err)
	require.Equal(t, schemaName, md2.Schema)

	tblMeta2, err := dbase2.TableMetadata(ctx, tblName)
	require.NoError(t, err)
	require.Equal(t, int64(wantRowCount), tblMeta2.RowCount)
}

func createSchema(ctx context.Context, db *sql.DB, name string) error {
	const tpl = `DROP SCHEMA IF EXISTS %q CASCADE;
CREATE SCHEMA %q;`
	stmt := fmt.Sprintf(tpl, name, name)
	_, err := db.ExecContext(ctx, stmt)
	return err
}

func dropSchema(ctx context.Context, db *sql.DB, name string) error {
	const tpl = `DROP SCHEMA IF EXISTS %q CASCADE;`
	stmt := fmt.Sprintf(tpl, name)
	_, err := db.ExecContext(ctx, stmt)
	return err
}

func createSimpleTable(ctx context.Context, db *sql.DB, schemaName, tblName string, insertRowCount int) error {
	const tpl = `CREATE TABLE %q.%q
	(
	id   serial PRIMARY KEY,
	NAME VARCHAR(255)
	);`

	stmt := fmt.Sprintf(tpl, schemaName, tblName)

	_, err := db.ExecContext(ctx, stmt)
	if err != nil {
		return errz.Err(err)
	}

	stmt = fmt.Sprintf("INSERT INTO %q.%q (NAME) VALUES ($1)", schemaName, tblName)

	for i := 0; i < insertRowCount; i++ {
		_, err = db.ExecContext(ctx, stmt, fmt.Sprintf("name-%d", i))
		if err != nil {
			return errz.Err(err)
		}
	}

	return nil
}

func BenchmarkDatabase_SourceMetadata(b *testing.B) {
	for _, handle := range sakila.PgAll() {
		handle := handle
		b.Run(handle, func(b *testing.B) {
			th := testh.New(b)
			th.Log = lg.Discard()
			dbase := th.Open(th.Source(handle))
			b.ResetTimer()

			md, err := dbase.SourceMetadata(th.Context, false)
			require.NoError(b, err)
			require.Equal(b, "sakila", md.Name)
		})
	}
}

func TestIsErrRelationDoesNotExist(t *testing.T) {
	th, src, _, _, _ := testh.NewWith(t, sakila.Pg)
	_, err := th.QuerySQL(src, "SELECT * FROM tbl_does_not_exist")
	require.Error(t, err)
	require.True(t, postgres.IsErrRelationNotExist(err))
}

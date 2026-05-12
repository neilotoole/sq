package duckdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
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

package duckdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

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

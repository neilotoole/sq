package duckdb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
)

// newAlterTestDB opens a fresh DuckDB file, creates a test table with three
// columns (id INTEGER, name VARCHAR, age INTEGER), inserts two rows, and
// returns the raw *sql.DB plus an sq Helper/Grip for schema introspection.
func newAlterTestDB(t *testing.T) (*sql.DB, *testh.Helper, *source.Source) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "alter_test.duckdb")

	db, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	_, err = db.ExecContext(ctx, `CREATE TABLE tbl (id INTEGER, name VARCHAR, age INTEGER)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO tbl VALUES (1, 'alice', 30), (2, 'bob', 25)`)
	require.NoError(t, err)

	// Wrap as an sq Source for metadata introspection.
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@alter_test",
		Type:     drivertype.DuckDB,
		Location: "duckdb://" + dbPath,
	}
	th.Add(src)

	return db, th, src
}

// TestAlterTableRename verifies that AlterTableRename renames a table
// and that the new name is visible via TableMetadata.
func TestAlterTableRename(t *testing.T) {
	db, th, src := newAlterTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	require.NoError(t, drvr.AlterTableRename(th.Context, db, "tbl", "tbl_renamed"))

	md, err := grip.TableMetadata(th.Context, "tbl_renamed")
	require.NoError(t, err)
	require.Equal(t, "tbl_renamed", md.Name)

	// Original name must no longer exist.
	origMD, err := grip.TableMetadata(th.Context, "tbl")
	require.Error(t, err, "old table name should not exist after rename")
	require.Nil(t, origMD)
}

// TestAlterTableAddColumn verifies that AlterTableAddColumn adds a new column
// with the expected kind to an existing table.
func TestAlterTableAddColumn(t *testing.T) {
	db, th, src := newAlterTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	require.NoError(t, drvr.AlterTableAddColumn(th.Context, db, "tbl", "score", kind.Float))

	md, err := grip.TableMetadata(th.Context, "tbl")
	require.NoError(t, err)

	col := md.Column("score")
	require.NotNil(t, col, "column 'score' not found in table metadata")
	require.Equal(t, kind.Float, col.Kind)
}

// TestAlterTableRenameColumn verifies that AlterTableRenameColumn renames a
// column and that the new name is visible via TableMetadata.
func TestAlterTableRenameColumn(t *testing.T) {
	db, th, src := newAlterTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	require.NoError(t, drvr.AlterTableRenameColumn(th.Context, db, "tbl", "name", "full_name"))

	md, err := grip.TableMetadata(th.Context, "tbl")
	require.NoError(t, err)

	require.NotNil(t, md.Column("full_name"), "renamed column 'full_name' not found")
	require.Nil(t, md.Column("name"), "old column 'name' should not exist after rename")
}

// TestAlterTableColumnKinds verifies that AlterTableColumnKinds changes
// column types and that the new kinds are reflected in TableMetadata.
func TestAlterTableColumnKinds(t *testing.T) {
	db, th, src := newAlterTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	// Change name (VARCHAR→kind.Text) to kind.Bytes and age (INTEGER) to kind.Float.
	err := drvr.AlterTableColumnKinds(
		th.Context, db, "tbl",
		[]string{"name", "age"},
		[]kind.Kind{kind.Bytes, kind.Float},
	)
	require.NoError(t, err)

	md, err := grip.TableMetadata(th.Context, "tbl")
	require.NoError(t, err)

	require.Equal(t, kind.Bytes, md.Column("name").Kind)
	require.Equal(t, kind.Float, md.Column("age").Kind)
}

// TestAlterTableColumnKinds_MismatchedLengths verifies that
// AlterTableColumnKinds returns an error when cols and kinds have
// different lengths.
func TestAlterTableColumnKinds_MismatchedLengths(t *testing.T) {
	db, th, src := newAlterTestDB(t)
	grip := th.Open(src)
	drvr := grip.SQLDriver()

	err := drvr.AlterTableColumnKinds(
		th.Context, db, "tbl",
		[]string{"name"},
		[]kind.Kind{kind.Text, kind.Int}, // one too many kinds
	)
	require.Error(t, err)
}

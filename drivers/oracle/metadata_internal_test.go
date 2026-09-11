package oracle

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// TestMetadataHelpers_ErrorPaths drives the first-query error branch of every
// unexported metadata helper by handing each a closed *sql.DB. This covers the
// errw(err) propagation paths deterministically. sql.Open does not dial the
// server, so no live Oracle is required and the test always runs.
func TestMetadataHelpers_ErrorPaths(t *testing.T) {
	const dsn = "oracle://u:p@localhost:1521/x"
	db, err := sql.Open("oracle", dsn)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	ctx := context.Background()
	const tbl = "ACTOR"

	_, err = queryOracleObjectNames(ctx, db, `SELECT table_name FROM user_tables`)
	require.Error(t, err)

	_, err = liveRowCount(ctx, db, tbl)
	require.Error(t, err)

	_, err = getColumnsMetadata(ctx, db, tbl)
	require.Error(t, err)

	_, err = getOraclePKColumnNames(ctx, db, tbl)
	require.Error(t, err)

	_, err = getOracleForeignKeys(ctx, db, tbl)
	require.Error(t, err)

	_, err = getOracleForeignKeys(ctx, db, "")
	require.Error(t, err)

	_, err = getOracleIncomingFKs(ctx, db, tbl)
	require.Error(t, err)

	_, err = getOracleUniqueConstraints(ctx, db, tbl)
	require.Error(t, err)

	_, err = getOracleUniqueConstraints(ctx, db, "")
	require.Error(t, err)

	_, err = getOracleIndexes(ctx, db, tbl)
	require.Error(t, err)

	_, err = getViewMetadata(ctx, db, tbl, true)
	require.Error(t, err)

	_, err = getMaterializedViewMetadata(ctx, db, tbl)
	require.Error(t, err)

	_, err = getTableMetadata(ctx, db, tbl, true)
	require.Error(t, err)

	_, err = getObjectMetadata(ctx, db, tbl)
	require.Error(t, err)

	_, err = loadUserSchemaObjectsMetadata(ctx, slog.New(slog.DiscardHandler), "@h", db)
	require.Error(t, err)

	src := &source.Source{Handle: "@h", Type: drivertype.Oracle, Location: dsn}
	_, err = getSourceMetadata(ctx, src, db, false)
	require.Error(t, err)
}

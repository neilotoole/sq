package oracle

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// oracleTestDSN matches the docker-compose default (sakiladb/oracle); override
// with SQ_TEST_ORACLE_DSN.
const oracleTestDSN = "oracle://sakila:p_ssW0rd@localhost:1521/SAKILA"

// openOracleForInternalTest opens a connection to the test Oracle instance,
// skipping the test if it's unreachable. The caller owns Close.
func openOracleForInternalTest(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dsn := os.Getenv("SQ_TEST_ORACLE_DSN")
	if dsn == "" {
		dsn = oracleTestDSN
	}
	// sql.Open only fails if the "oracle" driver isn't registered or the DSN
	// is malformed; both are real regressions, so require success here and
	// skip only when the instance itself is unreachable (PingContext).
	db, err := sql.Open("oracle", dsn)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Skipf("Oracle not reachable: %v", err)
	}
	return db
}

// TestGetSourceMetadata_NoSchema covers the noSchema=true early-return branch
// of getSourceMetadata, which grip.SourceMetadata(noSchema=false) doesn't hit.
func TestGetSourceMetadata_NoSchema(t *testing.T) {
	db := openOracleForInternalTest(t)
	defer db.Close()

	src := &source.Source{
		Handle:   "@ora_internal",
		Type:     drivertype.Oracle,
		Location: oracleTestDSN,
	}
	md, err := getSourceMetadata(context.Background(), src, db, true)
	require.NoError(t, err)
	require.NotNil(t, md)
	require.NotEmpty(t, md.Schema)
	require.Empty(t, md.Tables, "noSchema=true must skip table enumeration")
}

// TestMetadataHelpers_ErrorPaths drives the first-query error branch of every
// unexported metadata helper by handing each a closed *sql.DB. This covers the
// errw(err) propagation paths deterministically.
func TestMetadataHelpers_ErrorPaths(t *testing.T) {
	db := openOracleForInternalTest(t)
	require.NoError(t, db.Close())

	ctx := context.Background()
	const tbl = "ACTOR"

	_, err := queryOracleObjectNames(ctx, db, `SELECT table_name FROM user_tables`)
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

	_, err = getViewMetadata(ctx, db, tbl)
	require.Error(t, err)

	_, err = getMaterializedViewMetadata(ctx, db, tbl)
	require.Error(t, err)

	_, err = getTableMetadata(ctx, db, tbl, true)
	require.Error(t, err)

	_, err = getObjectMetadata(ctx, db, tbl)
	require.Error(t, err)

	_, err = loadUserSchemaObjectsMetadata(ctx, slog.New(slog.DiscardHandler), "@h", db)
	require.Error(t, err)

	src := &source.Source{Handle: "@h", Type: drivertype.Oracle, Location: oracleTestDSN}
	_, err = getSourceMetadata(ctx, src, db, false)
	require.Error(t, err)
}

package driver_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/mattn/go-sqlite3" // Registers the sqlite3 database/sql driver.
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// TestOpeningPing exercises driver.OpeningPing against a real (in-memory
// SQLite) database/sql pool. See issue #1013.
func TestOpeningPing(t *testing.T) {
	ctx := context.Background()
	src := &source.Source{Handle: "@ping_test", Type: drivertype.SQLite, Location: "sqlite3://:memory:"}

	openDB := func(t *testing.T) *sql.DB {
		t.Helper()
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		return db
	}

	fetchOK := func(ctx context.Context, db sqlz.DB) (string, error) {
		var raw string
		if err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&raw); err != nil {
			return "", err
		}
		return "v" + raw, nil
	}

	t.Run("version_select_is_the_ping", func(t *testing.T) {
		db := openDB(t)
		v, err := driver.OpeningPing(ctx, src, db, fetchOK)
		require.NoError(t, err)
		require.NotEmpty(t, v)
	})

	t.Run("fetch_failure_falls_back_to_plain_ping", func(t *testing.T) {
		// A live server that rejects the version query must still open, with
		// an empty semver (undeterminable) and no error.
		db := openDB(t)
		fetchErr := func(context.Context, sqlz.DB) (string, error) {
			return "", errors.New("version query rejected")
		}
		v, err := driver.OpeningPing(ctx, src, db, fetchErr)
		require.NoError(t, err)
		require.Empty(t, v)
		require.NoError(t, db.PingContext(ctx), "db must remain open")
	})

	t.Run("dead_db_fails_open", func(t *testing.T) {
		db := openDB(t)
		require.NoError(t, db.Close())
		_, err := driver.OpeningPing(ctx, src, db, fetchOK)
		require.Error(t, err)
	})
}

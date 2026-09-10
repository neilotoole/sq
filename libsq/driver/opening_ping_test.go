package driver_test

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
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
		fetchErr := func(ctx context.Context, db sqlz.DB) (string, error) {
			var raw string
			if err := db.QueryRowContext(ctx, "SELECT no_such_function()").Scan(&raw); err != nil {
				return "", err
			}
			return "", errors.New("expected the version query to be rejected")
		}
		v, err := driver.OpeningPing(ctx, src, db, fetchErr)
		require.NoError(t, err)
		require.Empty(t, v)
		require.NoError(t, db.PingContext(ctx), "db must remain open")
	})

	t.Run("failed_dial_is_not_retried", func(t *testing.T) {
		// When the version fetch fails because no connection could be
		// established (e.g. wrong password), the fallback ping must not dial
		// again: a second attempt with the same credentials doubles the
		// failed-login count against server lockout thresholds.
		drvr := &failingDialDriver{}
		sql.Register(t.Name(), drvr)
		db, err := sql.Open(t.Name(), "")
		require.NoError(t, err)
		_, err = driver.OpeningPing(ctx, src, db, fetchOK)
		require.Error(t, err)
		require.Equal(t, 1, drvr.dials, "the failed dial must not be repeated by a fallback ping")
	})

	t.Run("dead_db_fails_open", func(t *testing.T) {
		db := openDB(t)
		require.NoError(t, db.Close())
		_, err := driver.OpeningPing(ctx, src, db, fetchOK)
		require.Error(t, err)
	})
}

// failingDialDriver is a database/sql driver whose every dial fails, counting
// the attempts. It models a server that rejects the login.
type failingDialDriver struct {
	dials int
}

// Open implements database/sql/driver.Driver.
func (d *failingDialDriver) Open(string) (sqldriver.Conn, error) {
	d.dials++
	return nil, errors.New("access denied")
}

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
		// With no idle connections retained, the pool holds nothing after the
		// rejected query: the fallback must not depend on pool state.
		db.SetMaxIdleConns(0)
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
		c := &failingDialConnector{}
		db := sql.OpenDB(c)
		t.Cleanup(func() { _ = db.Close() })
		_, err := driver.OpeningPing(ctx, src, db, fetchOK)
		require.Error(t, err)
		require.Equal(t, 1, c.dials, "the failed dial must not be repeated by a fallback ping")
	})

	t.Run("dead_db_fails_open", func(t *testing.T) {
		db := openDB(t)
		require.NoError(t, db.Close())
		_, err := driver.OpeningPing(ctx, src, db, fetchOK)
		require.Error(t, err)
	})
}

// failingDialConnector is a database/sql connector whose every dial fails,
// counting the attempts. It models a server that rejects the login.
type failingDialConnector struct {
	dials int
}

// Connect implements database/sql/driver.Connector.
func (c *failingDialConnector) Connect(context.Context) (sqldriver.Conn, error) {
	c.dials++
	return nil, errors.New("access denied")
}

// Driver implements database/sql/driver.Connector.
func (c *failingDialConnector) Driver() sqldriver.Driver { return failingDialDriver{c} }

// failingDialDriver is the database/sql driver behind failingDialConnector.
// It exists to satisfy the Connector contract; every open fails via the
// connector.
type failingDialDriver struct {
	c *failingDialConnector
}

// Open implements database/sql/driver.Driver.
func (d failingDialDriver) Open(string) (sqldriver.Conn, error) {
	return d.c.Connect(context.Background())
}

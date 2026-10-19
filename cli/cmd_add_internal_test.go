package cli

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// TestMungeLocationForType_DoesNotLogSecret verifies that the "Munged
// location" debug log does not emit secrets carried in the location's query
// params. Munging absolutizes a file-DB path, so the log fires with the
// before/after locations, and a SQLCipher "_auth_pass" value survives munging
// untouched.
//
// This is the same class as the CodeQL go/clear-text-logging alerts on the
// ClickHouse driver, on a path the scan did not flag.
func TestMungeLocationForType_DoesNotLogSecret(t *testing.T) {
	const passwd = "hunter2"

	buf := &bytes.Buffer{}
	ctx := lg.NewContext(context.Background(),
		slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	loc := "sqlite3://data.db?_auth_pass=" + passwd
	munged, err := mungeLocationForType(ctx, drivertype.SQLite, loc)
	require.NoError(t, err)
	// Sanity check: munging changed the location, so the log call ran.
	require.NotEqual(t, loc, munged)
	require.Contains(t, buf.String(), "Munged location")
	require.NotContains(t, buf.String(), passwd, "secret leaked to log")
}

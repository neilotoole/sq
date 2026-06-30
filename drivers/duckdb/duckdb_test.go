package duckdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestOpenSakila verifies that the DuckDB driver can open the sakila test
// fixture and execute a basic count query via the raw *sql.DB, bypassing the
// libsq record-processing pipeline.
func TestOpenSakila(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	require.NotNil(t, src)

	grip := th.Open(src)

	db, err := grip.DB(context.Background())
	require.NoError(t, err)

	var n int
	require.NoError(t, db.QueryRowContext(context.Background(),
		"SELECT count(*) FROM actor").Scan(&n))
	require.Equal(t, 200, n)
}

// TestSLQ_BasicSelect verifies that the DuckDB render dialect can translate
// a simple SLQ query into valid DuckDB SQL and execute it end-to-end through
// the libsq pipeline.
func TestSLQ_BasicSelect(t *testing.T) {
	th := testh.New(t)

	sink, err := th.QuerySLQ(`@sakila_duck | .actor | .first_name | .[0:5]`, nil)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 5)
}

func TestDBSemver(t *testing.T) {
	t.Parallel()
	th, _, _, grip, _ := testh.NewWith(t, sakila.Duck)
	v, err := grip.DBSemver(th.Context)
	require.NoError(t, err)
	require.True(t, semver.IsValid(v), "want canonical semver, got %q", v)
	require.NotEqual(t, "v0.0.0", v, "want a real engine version, got degenerate %q", v)
}

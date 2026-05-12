package duckdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestOpenSakila verifies that the DuckDB driver can open the sakila test
// fixture and execute a basic count query via the raw *sql.DB. This bypasses
// the libsq record-processing pipeline (which depends on RecordMeta —
// implemented in Phase 4 Task 4.3) and so should pass once the driver
// scaffold (Phase 1) and sakila fixtures (Phase 2) are in place.
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

package sqlitevec_test

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"regexp"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3/internal/sqlitevec"
)

func TestHeaderMatchesRuntime(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	var v string
	require.NoError(t, db.QueryRow("SELECT sqlite_version()").Scan(&v))
	require.Equal(t, sqlitevec.HeaderVersion(), v)
}

func TestRegister(t *testing.T) {
	require.NoError(t, sqlitevec.Register())
	require.NoError(t, sqlitevec.Register()) // Idempotent.

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	var v string
	require.NoError(t, db.QueryRow("SELECT vec_version()").Scan(&v))
	require.Equal(t, "v0.2.7-alpha", v)
	require.Equal(t, v, sqlitevec.Version())
}

// TestVendoredChecksums verifies that each vendored file matches the sha256
// recorded in README.md.
func TestVendoredChecksums(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	require.NoError(t, err)

	rows := regexp.MustCompile(`(?m)^\|\s*(\S+)\s*\|\s*([0-9a-f]{64})\s*\|\s*$`).FindAllStringSubmatch(string(readme), -1)
	require.Len(t, rows, 5)

	for _, row := range rows {
		b, err := os.ReadFile(row[1])
		require.NoError(t, err, row[1])
		sum := sha256.Sum256(b)
		require.Equal(t, row[2], hex.EncodeToString(sum[:]), row[1])
	}
}

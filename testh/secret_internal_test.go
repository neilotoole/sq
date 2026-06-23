package testh

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestHarnessGripsResolvesEnvPlaceholder proves the harness Grips resolves a
// ${env:...} placeholder in a source Location at connect time. Before the
// registry is wired (nil), opening such a source fails with "no secret
// registry provided"; after wiring, it resolves and opens.
func TestHarnessGripsResolvesEnvPlaceholder(t *testing.T) {
	th := New(t)

	dbPath := proj.Abs(sakila.PathSL3) // absolute path to the sqlite sakila.db
	t.Setenv("SQ_TEST_SECRET_SL3_PATH", dbPath)

	src := &source.Source{
		Handle:   "@secret_env_sl3",
		Type:     drivertype.SQLite,
		Location: "sqlite3://${env:SQ_TEST_SECRET_SL3_PATH}",
	}
	th.Add(src)

	grip := th.Open(src) // resolves ${env:...}, opens, and pings
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRowContext(th.Context,
		"SELECT COUNT(*) FROM "+sakila.TblActor).Scan(&count))
	require.Positive(t, count, "resolved sqlite source must return rows")
}

// TestNewSecretRegistrySchemes pins the harness registry to the full
// production placeholder set. If this drifts from cli/run.go, update both.
func TestNewSecretRegistrySchemes(t *testing.T) {
	require.Equal(t,
		[]string{"env", "file", "keyring", "op"},
		newSecretRegistry().Schemes())
}

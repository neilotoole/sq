package testh

import (
	"os"
	"path/filepath"
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

// TestHelperSourceResolvesEnvPlaceholder verifies that Helper.Source resolves
// ${env:...} placeholders in a source Location (so file-copy / openNew paths
// that bypass Grips still see a concrete location).
func TestHelperSourceResolvesEnvPlaceholder(t *testing.T) {
	th := New(t)

	dbPath := proj.Abs(sakila.PathSL3)
	t.Setenv("SQ_TEST_SOURCE_RESOLVE_DB", dbPath)

	src := &source.Source{
		Handle:   "@source_resolve_sl3",
		Type:     drivertype.SQLite,
		Location: "sqlite3://${env:SQ_TEST_SOURCE_RESOLVE_DB}",
	}
	th.Add(src)

	got := th.Source("@source_resolve_sl3")
	// For SQLite, Source copies the db to a temp file and rewrites Location
	// to the copy; the key assertion is that no ${...} placeholder survives.
	require.NotContains(t, got.Location, "${", "placeholder must be resolved")
	require.Equal(t, drivertype.SQLite, got.Type)

	// And it actually opens and queries.
	grip := th.Open(got)
	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	var count int
	require.NoError(t, db.QueryRowContext(th.Context,
		"SELECT COUNT(*) FROM "+sakila.TblActor).Scan(&count))
	require.Positive(t, count)
}

// TestSourceConfigFileOverride verifies that SQ_TEST_CONFIG_FILE points the
// harness at a different config file, whose ${env:...} placeholders still
// resolve.
func TestSourceConfigFileOverride(t *testing.T) {
	dbPath := proj.Abs(sakila.PathSL3)
	t.Setenv("SQ_TEST_OVERRIDE_DB", dbPath)

	cfgPath := filepath.Join(t.TempDir(), "test.sq.yml")
	cfgYAML := "config.version: v0.34.0\n" +
		"collection:\n" +
		"  sources:\n" +
		"    - handle: '@override_sl3'\n" +
		"      driver: sqlite3\n" +
		"      location: sqlite3://${env:SQ_TEST_OVERRIDE_DB}\n"
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0o600))
	t.Setenv(proj.EnvTestConfigFile, cfgPath)

	th := New(t)
	grip := th.Open(th.Source("@override_sl3"))
	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	var count int
	require.NoError(t, db.QueryRowContext(th.Context,
		"SELECT COUNT(*) FROM "+sakila.TblActor).Scan(&count))
	require.Positive(t, count, "override config source must load and resolve")
}

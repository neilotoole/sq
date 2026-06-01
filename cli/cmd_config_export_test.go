package cli_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
)

// TestCmdConfigExport_Portable verifies that without --resolve, the
// output is valid YAML and any ${scheme:path} placeholder is written
// verbatim (no resolution attempt).
func TestCmdConfigExport_Portable(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@sakila",
		Type:     drivertype.SQLite,
		Location: "sqlite3://${keyring:abc123}",
	}))

	err := tr.Exec("config", "export")
	require.NoError(t, err)

	got := tr.OutString()
	require.Contains(t, got, "config.version:")
	require.Contains(t, got, "@sakila")
	require.Contains(t, got, "${keyring:abc123}",
		"placeholder must be preserved without --resolve")
	require.False(t, strings.Contains(got, "Warning:"),
		"no stderr warning without --resolve")
	require.Equal(t, "", tr.ErrOut.String())
}

// TestCmdConfigExport_Resolve_Keyring verifies that --resolve substitutes
// keyring values into Location strings.
func TestCmdConfigExport_Resolve_Keyring(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "abc123",
		"postgres://user:hunter2@db.local:5432/sakila"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@sakila",
		Type:     drivertype.Pg,
		Location: "${keyring:abc123}",
	}))

	err := tr.Exec("config", "export", "--resolve")
	require.NoError(t, err)

	got := tr.OutString()
	require.Contains(t, got, "postgres://user:hunter2@db.local:5432/sakila",
		"keyring placeholder must be expanded with --resolve")
	require.NotContains(t, got, "${keyring:",
		"no raw placeholders after --resolve")

	// The plaintext-warning audit entry is emitted to the logger, not to
	// stderr — we don't assert log output here to avoid coupling the test
	// to log routing details. Verified by reading the implementation.
	require.Equal(t, "", tr.ErrOut.String(),
		"no stderr output even when --resolve is set")
}

// TestCmdConfigExport_Resolve_Env verifies env: placeholders are resolved.
func TestCmdConfigExport_Resolve_Env(t *testing.T) {
	gokeyring.MockInit()
	t.Setenv("SQ_TEST_DB_PASS", "envhunter")

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@sakila",
		Type:     drivertype.Pg,
		Location: "postgres://u:${env:SQ_TEST_DB_PASS}@h/db",
	}))

	require.NoError(t, tr.Exec("config", "export", "--resolve"))

	got := tr.OutString()
	require.Contains(t, got, "postgres://u:envhunter@h/db")
}

// TestCmdConfigExport_Resolve_MissingKeyring errors clearly when a
// placeholder cannot be resolved.
func TestCmdConfigExport_Resolve_MissingKeyring(t *testing.T) {
	gokeyring.MockInit() // empty keyring

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@orphan",
		Type:     drivertype.SQLite,
		Location: "sqlite3://${keyring:missing}",
	}))

	err := tr.Exec("config", "export", "--resolve")
	require.Error(t, err)
	require.Contains(t, err.Error(), "@orphan",
		"error must name the source whose placeholder failed")
	require.Contains(t, err.Error(), "missing",
		"error must reference the failing placeholder path")
}

// TestCmdConfigExport_Output_Portable verifies -o writes a regular file
// with mode 0600, with no --resolve (placeholders preserved).
func TestCmdConfigExport_Output_Portable(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@sakila",
		Type:     drivertype.SQLite,
		Location: "sqlite3://${keyring:abc123}",
	}))

	out := filepath.Join(t.TempDir(), "out.yml")
	require.NoError(t, tr.Exec("config", "export", "-o", out))

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	require.Contains(t, string(data), "${keyring:abc123}")
	require.Contains(t, string(data), "@sakila")

	if runtime.GOOS != "windows" {
		info, err := os.Stat(out)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
			"-o must create file with mode 0600 even without --resolve")
	}
}

// TestCmdConfigExport_Output_Resolve verifies --resolve -o substitutes
// secrets and still produces a 0600 file.
func TestCmdConfigExport_Output_Resolve(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "abc123",
		"postgres://user:hunter2@db.local:5432/sakila"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@sakila",
		Type:     drivertype.Pg,
		Location: "${keyring:abc123}",
	}))

	out := filepath.Join(t.TempDir(), "out.yml")
	require.NoError(t, tr.Exec("config", "export", "--resolve", "-o", out))

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	require.Contains(t, string(data), "user:hunter2@db.local")

	if runtime.GOOS != "windows" {
		info, err := os.Stat(out)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

// TestCmdConfigExport_Resolve_File verifies that ${file:PATH}
// placeholders are read from disk and spliced into Location.
func TestCmdConfigExport_Resolve_File(t *testing.T) {
	gokeyring.MockInit()

	secretPath := filepath.Join(t.TempDir(), "dsn.txt")
	require.NoError(t, os.WriteFile(secretPath,
		[]byte("postgres://u:filehunter@h/db"), 0o600))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@sakila",
		Type:     drivertype.Pg,
		Location: "${file:" + secretPath + "}",
	}))

	require.NoError(t, tr.Exec("config", "export", "--resolve"))

	got := tr.OutString()
	require.Contains(t, got, "postgres://u:filehunter@h/db",
		"file placeholder must be expanded with --resolve")
	require.NotContains(t, got, "${file:",
		"no raw file placeholders after --resolve")
}

// TestCmdConfigExport_Resolve_MultiSource verifies that --resolve
// handles a collection with multiple sources whose Locations use
// different placeholder schemes (or none), and that inline-credentialed
// sources pass through unchanged.
func TestCmdConfigExport_Resolve_MultiSource(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "pg_dsn",
		"postgres://k_user:k_pass@k.host/db"))
	t.Setenv("SQ_TEST_MYSQL_PASS", "envmysql")

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// 1) Keyring-backed.
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@pg",
		Type:     drivertype.Pg,
		Location: "${keyring:pg_dsn}",
	}))
	// 2) Env-backed (placeholder inside DSN, not whole DSN).
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@mysql",
		Type:     drivertype.MySQL,
		Location: "mysql://m_user:${env:SQ_TEST_MYSQL_PASS}@m.host/db",
	}))
	// 3) Inline plaintext credentials — no placeholders.
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@sqlite",
		Type:     drivertype.SQLite,
		Location: "sqlite3:///tmp/inline.db",
	}))

	require.NoError(t, tr.Exec("config", "export", "--resolve"))

	got := tr.OutString()

	require.Contains(t, got, "postgres://k_user:k_pass@k.host/db",
		"keyring source must be resolved")
	require.Contains(t, got, "mysql://m_user:envmysql@m.host/db",
		"env placeholder inside DSN must be resolved")
	require.Contains(t, got, "sqlite3:///tmp/inline.db",
		"inline-credentialed source must pass through verbatim")

	require.NotContains(t, got, "${keyring:")
	require.NotContains(t, got, "${env:")
}

// TestCmdConfigExport_Output_CreatesParentDir verifies that -o creates
// missing parent directories, matching the convenience the framework's
// -o auto-redirect provides for other sq commands.
func TestCmdConfigExport_Output_CreatesParentDir(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@sakila",
		Type:     drivertype.SQLite,
		Location: "sqlite3:///tmp/inline.db",
	}))

	// Path with a parent dir that does NOT yet exist.
	tmp := t.TempDir()
	out := filepath.Join(tmp, "nested", "deeper", "backup.yml")
	require.NoError(t, tr.Exec("config", "export", "-o", out),
		"-o must create missing parent dirs")

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	require.Contains(t, string(data), "@sakila")

	if runtime.GOOS != "windows" {
		// Freshly-created dirs should be 0o700 (not world-readable),
		// since the file may contain credentials.
		for _, dir := range []string{
			filepath.Join(tmp, "nested"),
			filepath.Join(tmp, "nested", "deeper"),
		} {
			info, err := os.Stat(dir)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o700), info.Mode().Perm(),
				"new parent dir %s must be 0700", dir)
		}
	}
}

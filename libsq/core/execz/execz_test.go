package execz_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/execz"
)

// TestCmd_LogValue_RedactsEnv verifies that logging a [execz.Cmd] does not
// leak env values. Env vars passed to external tools exist to carry
// connection material (e.g. PGPASSWORD set by postgres.DumpClusterCmd), so
// the log rendering must mask every env value. URL-shaped values are masked
// entirely too: partial URL redaction keeps only the userinfo password
// secret, and would leak credentials carried in the query string (e.g.
// "?sslpassword=..." or a presigned "?X-Amz-Signature=...").
func TestCmd_LogValue_RedactsEnv(t *testing.T) {
	cmd := &execz.Cmd{
		Name: "pg_dumpall",
		Env: []string{
			"PGPASSWORD=hunter2",
			"DSN=postgres://alice@db.acme.com:5432/sakila?sslpassword=hunter2",
		},
		Args: []string{"--dbname", "postgres://alice:hunter2@db.acme.com:5432/sakila"},
	}

	buf := &bytes.Buffer{}
	log := slog.New(slog.NewTextHandler(buf, nil))
	log.Info("exec", "cmd", cmd)
	got := buf.String()

	require.NotContains(t, got, "hunter2",
		"secret must not appear in log output, whether the env value is URL-shaped or not")
	require.Contains(t, got, "PGPASSWORD=",
		"env var name should remain visible")
	require.Contains(t, got, "DSN=",
		"env var name should remain visible")
	require.Contains(t, got, "db.acme.com",
		"non-sensitive parts of URL-shaped args should remain visible")
}

// TestCmd_String_IncludesCredentials pins [execz.Cmd.String]'s documented
// behavior: it is the executable (shell) rendering used by the db commands'
// --print flag, and intentionally includes credentials.
func TestCmd_String_IncludesCredentials(t *testing.T) {
	cmd := &execz.Cmd{
		Name: "pg_dumpall",
		Env:  []string{"PGPASSWORD=hunter2"},
		Args: []string{"--dbname", "postgres://alice:hunter2@db.acme.com:5432/sakila"},
	}

	require.Contains(t, cmd.String(), "PGPASSWORD=hunter2")
	require.Contains(t, cmd.String(), "postgres://alice:hunter2@db.acme.com:5432/sakila")
}

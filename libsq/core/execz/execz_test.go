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
// the log rendering must mask every env value, not just URL-shaped ones.
func TestCmd_LogValue_RedactsEnv(t *testing.T) {
	cmd := &execz.Cmd{
		Name: "pg_dumpall",
		Env:  []string{"PGPASSWORD=hunter2"},
		Args: []string{"--dbname", "postgres://alice:hunter2@db.acme.com:5432/sakila"},
	}

	buf := &bytes.Buffer{}
	log := slog.New(slog.NewTextHandler(buf, nil))
	log.Info("exec", "cmd", cmd)
	got := buf.String()

	require.NotContains(t, got, "hunter2",
		"secret must not appear in log output")
	require.Contains(t, got, "PGPASSWORD=",
		"env var name should remain visible")
	require.Contains(t, got, "db.acme.com",
		"non-sensitive URL parts should remain visible")

	// Cmd.String is the executable (shell) rendering: it intentionally
	// includes the secret, per its godoc.
	require.Contains(t, cmd.String(), "PGPASSWORD=hunter2")
}

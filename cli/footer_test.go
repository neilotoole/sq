package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/testh"
)

func TestFooter_skipsWhenStdoutNotTerminal(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	require.NoError(t, writeUpdateCheckCache(tr.Run.Files.CacheDir(), "v99.0.0"))
	require.NoError(t, tr.Exec("driver", "ls"))
	require.NotContains(t, tr.ErrOut.String(), "Update available")
}

func writeUpdateCheckCache(cacheDir, latest string) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	b, err := json.Marshal(struct {
		LatestVersion string    `json:"latest_version"`
		CheckedAt     time.Time `json:"checked_at"`
	}{
		LatestVersion: latest,
		CheckedAt:     time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(cacheDir, "update-check.json"), b, 0o600)
}

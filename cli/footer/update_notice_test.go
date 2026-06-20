package footer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/config/yamlstore"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/cli/updatecheck"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/testh/tu"
)

func TestUpdateNotice_eligible(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	optReg := &options.Registry{}
	cacheDir := tu.TempDir(t, "cache")
	fs, err := files.New(ctx, optReg, noopLockFunc, tu.TempDir(t, "temp"), cacheDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fs.Close() })

	latest := newerThan(buildinfo.Get().Version)
	require.NoError(t, writeUpdateCache(cacheDir, latest))

	ru := &run.Run{Files: fs}
	n := updateNotice{}
	require.True(t, n.Eligible(ctx, ru, nil))
}

func TestUpdateNotice_eligible_configStoreOnly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := &yamlstore.Store{Path: filepath.Join(t.TempDir(), "sq.yml")}
	latest := newerThan(buildinfo.Get().Version)
	require.NoError(t, writeUpdateCache(updatecheck.CacheDirForRun(&run.Run{ConfigStore: store}), latest))

	ru := &run.Run{ConfigStore: store}
	n := updateNotice{}
	require.True(t, n.Eligible(ctx, ru, errz.New("accepts 1 arg(s), received 0")))
}

func TestUpdateNotice_ineligible_versionCommand(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	optReg := &options.Registry{}
	cacheDir := tu.TempDir(t, "cache")
	fs, err := files.New(ctx, optReg, noopLockFunc, tu.TempDir(t, "temp"), cacheDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fs.Close() })

	latest := newerThan(buildinfo.Get().Version)
	require.NoError(t, writeUpdateCache(cacheDir, latest))

	ru := &run.Run{
		Files: fs,
		Cmd:   &cobra.Command{Use: "version"},
	}
	n := updateNotice{}
	require.False(t, n.Eligible(ctx, ru, nil))
}

func TestUpdateNotice_write(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	optReg := &options.Registry{}
	cacheDir := tu.TempDir(t, "cache")
	fs, err := files.New(ctx, optReg, noopLockFunc, tu.TempDir(t, "temp"), cacheDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fs.Close() })

	latest := newerThan(buildinfo.Get().Version)
	require.NoError(t, writeUpdateCache(cacheDir, latest))

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)

	ru := &run.Run{Files: fs}
	require.NoError(t, updateNotice{}.Write(buf, pr, ru, 40))
	require.Contains(t, buf.String(), "Update available:")
	require.Contains(t, buf.String(), latest)
}

func newerThan(current string) string {
	if semver.Compare("v99.0.0", current) > 0 {
		return "v99.0.0"
	}
	return "v999.0.0"
}

func writeUpdateCache(cacheDir, latest string) error {
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

func noopLockFunc(context.Context) (func(), error) {
	return func() {}, nil
}

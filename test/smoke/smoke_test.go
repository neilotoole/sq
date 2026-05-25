// Package smoke contains a small, fast cross-platform smoke suite that
// exercises the parts of sq historically fragile on Windows: path/DSN
// parsing, config/cache/temp dir resolution, file-based queries, and output
// rendering. It runs on every platform as part of the normal test suite, and
// is the only Windows job that runs on draft PRs (see
// .github/workflows/main.yml).
package smoke

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/config/yamlstore"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestWindowsSmoke is a fast smoke suite for Windows-fragile behavior.
// Despite the name it runs on all platforms; on Windows it is the only test
// job that runs on draft PRs.
func TestWindowsSmoke(t *testing.T) {
	t.Run("location", func(t *testing.T) {
		// These cases intentionally duplicate libsq/source/location's unit
		// tests: on draft PRs the Windows job runs ONLY this smoke package,
		// so this is the only place the C:\ drive-path parse is checked on
		// Windows. Do not "dedupe" it away.
		// Cross-platform cases.
		got, err := location.Parse("/path/to/sakila.xlsx")
		require.NoError(t, err)
		require.Equal(t, "sakila", got.Name)
		require.Equal(t, ".xlsx", got.Ext)

		got, err = location.Parse("https://server:8080/path/to/sakila.xlsx")
		require.NoError(t, err)
		require.Equal(t, "https", got.Scheme)
		require.Equal(t, "server", got.Hostname)
		require.Equal(t, 8080, got.Port)
		require.Equal(t, "sakila", got.Name)

		// Windows drive-path cases only parse as intended on Windows.
		if runtime.GOOS != "windows" {
			return
		}

		got, err = location.Parse(`sqlite3://C:\path\to\sakila.sqlite`)
		require.NoError(t, err)
		require.Equal(t, drivertype.SQLite, got.DriverType)
		require.Equal(t, "sqlite3", got.Scheme)
		require.Equal(t, "sakila", got.Name)
		require.Equal(t, ".sqlite", got.Ext)
		require.Equal(t, `C:\path\to\sakila.sqlite`, got.DSN)
	})

	t.Run("dirs", func(t *testing.T) {
		// Keep this hermetic: pin the env vars that config/cache resolution
		// consults to known absolute temp paths, so an ambient (possibly
		// relative) SQ_CONFIG or XDG_CACHE_HOME can't skew the result.
		tmp := t.TempDir()
		t.Setenv(config.EnvarConfig, filepath.Join(tmp, "config"))
		t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

		cfgDir, _, err := yamlstore.ConfigDir(nil)
		require.NoError(t, err)
		require.NotEmpty(t, cfgDir)
		require.True(t, filepath.IsAbs(cfgDir), "config dir must be absolute: %s", cfgDir)

		cacheDir := files.DefaultCacheDir()
		require.NotEmpty(t, cacheDir)
		require.True(t, filepath.IsAbs(cacheDir), "cache dir must be absolute: %s", cacheDir)

		// Exercise the rename-dir trick, which is fragile on Windows.
		base := t.TempDir()
		oldDir := filepath.Join(base, "old")
		newDir := filepath.Join(base, "new")
		require.NoError(t, os.MkdirAll(filepath.Join(oldDir, "sub"), 0o700))
		require.NoError(t, os.MkdirAll(newDir, 0o700))
		require.NoError(t, ioz.RenameDir(oldDir, newDir))
		require.DirExists(t, filepath.Join(newDir, "sub"))
	})

	t.Run("end_to_end", func(t *testing.T) {
		ctx := context.Background()

		// @sqlite is added first, so it becomes the active source.
		tr := testrun.New(ctx, t, nil)
		require.NoError(t, tr.Exec("add", proj.Abs(sakila.PathSL3), "--handle", "@sqlite"))

		tr = testrun.New(ctx, t, tr)
		require.NoError(t, tr.Exec("add", proj.Abs(sakila.PathCSVActor), "--handle", "@csv"))

		// .actor exists only in @sqlite and .data only in @csv, so a
		// mis-routed query fails loudly rather than silently returning the
		// same row count.
		// Query the active SQLite source.
		tr = testrun.New(ctx, t, tr)
		require.NoError(t, tr.Exec("--csv", "--no-header", ".actor"))
		require.Len(t, tr.BindCSV(), sakila.TblActorCount)

		// Query the CSV source explicitly.
		tr = testrun.New(ctx, t, tr)
		require.NoError(t, tr.Exec("--csv", "--no-header", "--src", "@csv", ".data"))
		require.Len(t, tr.BindCSV(), sakila.TblActorCount)
	})

	t.Run("output", func(t *testing.T) {
		ctx := context.Background()

		tr := testrun.New(ctx, t, nil)
		require.NoError(t, tr.Exec("add", proj.Abs(sakila.PathCSVActor), "--handle", "@csv"))

		tr = testrun.New(ctx, t, tr)
		require.NoError(t, tr.Exec("--json", "--src", "@csv", ".data"))

		out := tr.Out.String()
		require.NotContains(t, out, "\r", "JSON output must not contain carriage returns")
		require.NotContains(t, out, "\x1b[", "output must not contain ANSI escapes when not a TTY")
		rows := tr.BindSliceMap()
		require.Len(t, rows, sakila.TblActorCount)
		require.Contains(t, rows[0], "actor_id", "output should be actor records")
	})
}

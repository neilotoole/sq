package lockfile_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
)

// writeForeignOwner writes a lock file at fp whose recorded owner is the parent
// process (a live process that is not the test process), so that
// nightlyone/lockfile treats the lock as held by another process and TryLock
// returns ErrBusy. The pid-line format ("%d\n") matches the upstream package.
func writeForeignOwner(t *testing.T, fp string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(fp), 0o700))
	require.NoError(t, os.WriteFile(fp, fmt.Appendf(nil, "%d\n", os.Getppid()), 0o600))
}

func TestNew(t *testing.T) {
	t.Run("absolute_ok", func(t *testing.T) {
		fp := filepath.Join(t.TempDir(), "pid.lock")
		lf, err := lockfile.New(fp)
		require.NoError(t, err)
		require.Equal(t, fp, lf.String())
	})

	t.Run("absolute_nonexistent_ok", func(t *testing.T) {
		// New must accept an absolute path that doesn't exist yet.
		fp := filepath.Join(t.TempDir(), "no_such_dir", "pid.lock")
		lf, err := lockfile.New(fp)
		require.NoError(t, err)
		require.Equal(t, fp, lf.String())
	})

	t.Run("relative_error", func(t *testing.T) {
		_, err := lockfile.New(filepath.Join("relative", "pid.lock"))
		require.Error(t, err)
	})
}

func TestLockfile_String(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "pid.lock")
	lf, err := lockfile.New(fp)
	require.NoError(t, err)
	require.Equal(t, fp, lf.String())
}

func TestLockfile_Lock_noTimeout(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "pid.lock")
	lf, err := lockfile.New(fp)
	require.NoError(t, err)

	require.NoError(t, lf.Lock(context.Background(), 0))
	require.True(t, fileExists(fp), "lock file should exist after Lock")
	require.NoError(t, lf.Unlock())
}

func TestLockfile_Lock_createsParentDir(t *testing.T) {
	// The parent dir doesn't exist yet; Lock must create it.
	fp := filepath.Join(t.TempDir(), "sub", "nested", "pid.lock")
	lf, err := lockfile.New(fp)
	require.NoError(t, err)

	require.NoError(t, lf.Lock(context.Background(), 0))
	t.Cleanup(func() { _ = lf.Unlock() })
	require.True(t, fileExists(filepath.Dir(fp)), "parent dir should be created")
}

func TestLockfile_Lock_withTimeout_success(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "pid.lock")
	lf, err := lockfile.New(fp)
	require.NoError(t, err)

	require.NoError(t, lf.Lock(context.Background(), time.Second))
	require.NoError(t, lf.Unlock())
}

func TestLockfile_Lock_noTimeout_busy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("foreign-owner simulation is unreliable on Windows")
	}
	fp := filepath.Join(t.TempDir(), "pid.lock")
	writeForeignOwner(t, fp)

	lf, err := lockfile.New(fp)
	require.NoError(t, err)

	err = lf.Lock(context.Background(), 0)
	require.Error(t, err, "lock held by another live process must fail")
}

func TestLockfile_Lock_withTimeout_busy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("foreign-owner simulation is unreliable on Windows")
	}
	fp := filepath.Join(t.TempDir(), "pid.lock")
	writeForeignOwner(t, fp)

	lf, err := lockfile.New(fp)
	require.NoError(t, err)

	const timeout = 250 * time.Millisecond
	start := time.Now()
	err = lf.Lock(context.Background(), timeout)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Contains(t, err.Error(), "locked by other process")
	// It should have retried up to roughly the timeout before giving up.
	require.GreaterOrEqual(t, elapsed, timeout/2, "should retry until ~timeout")
}

func TestLockfile_Lock_contextCancelled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("foreign-owner simulation is unreliable on Windows")
	}
	fp := filepath.Join(t.TempDir(), "pid.lock")
	writeForeignOwner(t, fp)

	lf, err := lockfile.New(fp)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = lf.Lock(ctx, time.Minute)
	require.Error(t, err, "a cancelled context must abort the retry loop")
}

func TestLockfile_Lock_requireDirFails(t *testing.T) {
	// A regular file sits where the lockfile's parent dir would need to be.
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

	fp := filepath.Join(blocker, "pid.lock")
	lf, err := lockfile.New(fp)
	require.NoError(t, err)

	err = lf.Lock(context.Background(), 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parent dir")
}

func TestLockfile_Lock_withTimeout_acquireError(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("requires non-root on a permission-enforcing filesystem")
	}
	// The parent dir exists but is not writable, so the lock file can't be
	// created. This is a non-temporary error (not ErrBusy), exercising the
	// generic "acquire lock" failure branch under a timeout.
	dir := filepath.Join(t.TempDir(), "readonly")
	require.NoError(t, os.Mkdir(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) // so TempDir cleanup can remove it

	fp := filepath.Join(dir, "pid.lock")
	lf, err := lockfile.New(fp)
	require.NoError(t, err)

	err = lf.Lock(context.Background(), 200*time.Millisecond)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "locked by other process")
}

func TestLockfile_Unlock_notOwned(t *testing.T) {
	// Unlock without ever holding the lock returns an error.
	fp := filepath.Join(t.TempDir(), "pid.lock")
	lf, err := lockfile.New(fp)
	require.NoError(t, err)
	require.Error(t, lf.Unlock())
}

func TestLockfile_LockUnlockRelock(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "pid.lock")
	lf, err := lockfile.New(fp)
	require.NoError(t, err)

	require.NoError(t, lf.Lock(context.Background(), 0))
	require.NoError(t, lf.Unlock())
	// After unlocking, the lock can be acquired again.
	require.NoError(t, lf.Lock(context.Background(), time.Second))
	require.NoError(t, lf.Unlock())
}

// fileExists reports whether path exists. Local helper to avoid importing
// ioz just for a stat in tests.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

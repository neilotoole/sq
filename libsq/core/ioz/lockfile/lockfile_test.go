package lockfile_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
	"github.com/neilotoole/sq/libsq/core/lg"
)

// FIXME: Duh, this can't work, because we're in the same pid.
func TestLockfile(t *testing.T) {
	ctx := lg.NewContext(context.Background(), slogt.New(t))

	pidfile := filepath.Join(t.TempDir(), "lock.pid")
	lock, err := lockfile.New(pidfile)
	require.NoError(t, err)
	require.Equal(t, pidfile, string(lock))

	require.NoError(t, lock.Lock(ctx, 0),
		"should be able to acquire lock immediately")
	time.AfterFunc(time.Second*100, func() {
		require.NoError(t, lock.Unlock())
	})

	err = lock.Lock(ctx, time.Second)
	require.Error(t, err, "not enough time to acquire the lock")

	err = lock.Lock(ctx, time.Second*10)
	require.NoError(t, err, "should be able to acquire the lock")
}

// Package lockfile implements a pid lock file mechanism.
package lockfile

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/nightlyone/lockfile"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/retry"
)

// Lockfile is a pid file which can be locked.
type Lockfile string

// New returns a new Lockfile instance. Arg fp must be
// an absolute path (but it's legal for the path to not exist).
func New(fp string) (Lockfile, error) {
	lf, err := lockfile.New(fp)
	if err != nil {
		return "", errz.Err(err)
	}
	return Lockfile(lf), nil
}

// Lock attempts to acquire the lock, retrying if necessary, until the timeout
// expires. If timeout is zero, retry will not occur. On success, nil is
// returned. An error is returned if the lock cannot be acquired for any reason.
func (l Lockfile) Lock(ctx context.Context, timeout time.Duration) error {
	log := lg.FromContext(ctx).With(lga.Lock, l, lga.Timeout, timeout)

	dir := filepath.Dir(string(l))
	if err := ioz.RequireDir(dir); err != nil {
		return errz.Wrapf(err, "failed to create parent dir of lockfile: %s", string(l))
	}

	if timeout == 0 {
		if err := lockfile.Lockfile(l).TryLock(); err != nil {
			log.Warn("Failed to acquire pid lock", lga.Path, string(l), lga.Err, err)
			return errz.Wrapf(err, "failed to acquire pid lock: %s", l)
		}
		lg.Depth(log, slog.LevelDebug, 1, "Acquired pid lock")
		return nil
	}

	start, attempts := time.Now(), 0

	err := retry.Do(ctx, timeout,
		func() error {
			err := lockfile.Lockfile(l).TryLock()
			attempts++
			if err == nil {
				lg.Depth(log, slog.LevelDebug, 6, "Acquired pid lock")
				return nil
			}

			return err
		},
		errz.Has[lockfile.TemporaryError],
	)

	elapsed := time.Since(start)
	if err != nil {
		log.Error("Failed to acquire pid lock",
			lga.Attempts, attempts,
			lga.Elapsed, elapsed,
			lga.Err, err,
		)

		if errors.Is(err, lockfile.ErrBusy) {
			return errz.Errorf("locked by other process")
		}

		return errz.Wrapf(err, "acquire lock")
	}

	return nil
}

// Unlock a lock, if we owned it. Returns any error that
// happened during release of lock.
func (l Lockfile) Unlock() error {
	return errz.Err(lockfile.Lockfile(l).Unlock())
}

// String returns the Lockfile's absolute path.
func (l Lockfile) String() string {
	return string(l)
}

// LockFunc is a function that encapsulates locking and unlocking.
type LockFunc func(ctx context.Context) (unlock func(), err error)

// Package lockfile implements a pid lock file mechanism.
package lockfile

import (
	"context"
	"errors"
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
// an absolute path (but the path may not exist).
func New(fp string) (Lockfile, error) {
	lf, err := lockfile.New(fp)
	if err != nil {
		return "", errz.Err(err)
	}
	return Lockfile(lf), nil
}

// Lock attempts to acquire the lockfile, retrying if necessary,
// until the timeout expires. If timeout is zero, retry will not occur.
// On success, nil is returned. An error is returned if the lock cannot
// be acquired for any reason.
func (l Lockfile) Lock(ctx context.Context, timeout time.Duration) error {
	log := lg.FromContext(ctx).With(lga.Lock, l, lga.Timeout, timeout)

	dir := filepath.Dir(string(l))
	if err := ioz.RequireDir(dir); err != nil {
		return errz.Wrapf(err, "failed create parent dir of cache lock: %s", string(l))
	}

	if timeout == 0 {
		if err := lockfile.Lockfile(l).TryLock(); err != nil {
			log.Warn("Failed to acquire pid lock", lga.Err, err)
			return errz.Wrapf(err, "failed to acquire pid lock: %s", l)
		}
		log.Debug("Acquired pid lock")
		return nil
	}

	start, attempts := time.Now(), 0

	err := retry.Do(ctx, timeout,
		func() error {
			err := lockfile.Lockfile(l).TryLock()
			attempts++
			if err == nil {
				log.Debug("Acquired pid lock", lga.Attempts, attempts)
				return nil
			}

			// log.Debug("Failed to acquire pid lock, may retry", lga.Attempts, attempts, lga.Err, err)
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

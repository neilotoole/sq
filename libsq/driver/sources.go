package driver

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nightlyone/lockfile"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/retry"
	"github.com/neilotoole/sq/libsq/source"
)

var (
	_ PoolOpener        = (*Pools)(nil)
	_ JoinPoolOpener    = (*Pools)(nil)
	_ ScratchPoolOpener = (*Pools)(nil)
)

// ScratchSrcFunc is a function that returns a scratch source.
// The caller is responsible for invoking cleanFn.
type ScratchSrcFunc func(ctx context.Context, name string) (src *source.Source, cleanFn func() error, err error)

// Pools provides a mechanism for getting Pool instances.
// Note that at this time instances returned by Open are cached
// and then closed by Close. This may be a bad approach.
//
// FIXME: Why not rename driver.Pools to driver.Sources?
type Pools struct {
	log          *slog.Logger
	drvrs        Provider
	mu           sync.Mutex
	scratchSrcFn ScratchSrcFunc
	files        *source.Files
	pools        map[string]Pool
	clnup        *cleanup.Cleanup
}

// NewPools returns a Pools instances.
func NewPools(log *slog.Logger, drvrs Provider,
	files *source.Files, scratchSrcFn ScratchSrcFunc,
) *Pools {
	return &Pools{
		log:          log,
		drvrs:        drvrs,
		mu:           sync.Mutex{},
		scratchSrcFn: scratchSrcFn,
		files:        files,
		pools:        map[string]Pool{},
		clnup:        cleanup.New(),
	}
}

// Open returns an opened Pool for src. The returned Pool
// may be cached and returned on future invocations for the
// same source (where each source fields is identical).
// Thus, the caller should typically not close
// the Pool: it will be closed via d.Close.
//
// NOTE: This entire logic re caching/not-closing is a bit sketchy,
// and needs to be revisited.
//
// Open implements PoolOpener.
func (d *Pools) Open(ctx context.Context, src *source.Source) (Pool, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.doOpen(ctx, src)
}

func (d *Pools) doOpen(ctx context.Context, src *source.Source) (Pool, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)
	key := src.Handle + "_" + src.Hash()

	pool, ok := d.pools[key]
	if ok {
		return pool, nil
	}

	drvr, err := d.drvrs.DriverFor(src.Type)
	if err != nil {
		return nil, err
	}

	baseOptions := options.FromContext(ctx)
	o := options.Merge(baseOptions, src.Options)

	ctx = options.NewContext(ctx, o)
	pool, err = drvr.Open(ctx, src)
	if err != nil {
		return nil, err
	}

	d.clnup.AddC(pool)

	d.pools[key] = pool
	return pool, nil
}

// OpenScratchFor returns a scratch database instance. It is not
// necessary for the caller to close the returned Pool as
// its Close method will be invoked by d.Close.
//
// OpenScratchFor implements ScratchPoolOpener.
//
// REVISIT: do we really need to pass a source here? Just a string should do.
//
// FIXME: the problem is with passing src?
//
// FIXME: Add cacheAllowed bool?
func (d *Pools) OpenScratchFor(ctx context.Context, src *source.Source) (Pool, error) {
	const msgCloseScratch = "Close scratch db"

	_, srcCacheDBFilepath, _, err := d.getCachePaths(src)
	if err != nil {
		return nil, err
	}

	scratchSrc, cleanFn, err := d.scratchSrcFn(ctx, srcCacheDBFilepath)
	if err != nil {
		// if err is non-nil, cleanup is guaranteed to be nil
		return nil, err
	}
	d.log.Debug("Opening scratch src", lga.Src, scratchSrc)

	backingDrvr, err := d.drvrs.DriverFor(scratchSrc.Type)
	if err != nil {
		lg.WarnIfFuncError(d.log, msgCloseScratch, cleanFn)
		return nil, err
	}

	var backingPool Pool
	backingPool, err = backingDrvr.Open(ctx, scratchSrc)
	if err != nil {
		lg.WarnIfFuncError(d.log, msgCloseScratch, cleanFn)
		return nil, err
	}

	allowCache := OptIngestCache.Get(options.FromContext(ctx))
	if !allowCache {
		// If the ingest cache is disabled, we add the cleanup func
		// so the scratch DB is deleted when the session ends.
		d.clnup.AddE(cleanFn)
	}

	return backingPool, nil
}

// OpenIngest implements driver.ScratchPoolOpener.
func (d *Pools) OpenIngest(ctx context.Context, src *source.Source,
	ingestFn func(ctx context.Context, destPool Pool) error, allowCache bool,
) (Pool, error) {
	if !allowCache || src.Handle == source.StdinHandle {
		// We don't currently cache stdin.
		return d.openIngestNoCache(ctx, src, ingestFn)
	}

	return d.openIngestCache(ctx, src, ingestFn)
}

func (d *Pools) openIngestNoCache(ctx context.Context, src *source.Source,
	ingestFn func(ctx context.Context, destPool Pool) error,
) (Pool, error) {
	log := lg.FromContext(ctx)
	impl, err := d.OpenScratchFor(ctx, src)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	err = ingestFn(ctx, impl)
	elapsed := time.Since(start)

	if err != nil {
		log.Error("Ingest failed",
			lga.Src, src, lga.Dest, impl.Source(),
			lga.Elapsed, elapsed, lga.Err, err,
		)
	}

	d.log.Debug("Ingest completed",
		lga.Src, src, lga.Dest, impl.Source(),
		lga.Elapsed, elapsed)
	return impl, nil
}

func (d *Pools) openIngestCache(ctx context.Context, src *source.Source,
	ingestFn func(ctx context.Context, destPool Pool) error,
) (Pool, error) {
	log := lg.FromContext(ctx)

	lock, err := d.acquireLock(ctx, src)
	if err != nil {
		return nil, err
	}
	defer func() {
		log.Debug("About to release cache lock...", "lock", lock)
		if err = lock.Unlock(); err != nil {
			log.Warn("Failed to release cache lock", "lock", lock, lga.Err, err)
		} else {
			log.Debug("Released cache lock", "lock", lock)
		}
	}()

	cacheDir, _, checksumsPath, err := d.getCachePaths(src)
	if err != nil {
		return nil, err
	}

	log.Debug("Using cache dir", lga.Path, cacheDir)

	ingestFilePath, err := d.files.Filepath(ctx, src)
	if err != nil {
		return nil, err
	}

	var (
		impl        Pool
		foundCached bool
	)
	if impl, foundCached, err = d.OpenCachedFor(ctx, src); err != nil {
		return nil, err
	}
	if foundCached {
		log.Debug("Ingest cache HIT: found cached copy of source",
			lga.Src, src, "cached", impl.Source(),
		)
		return impl, nil
	}

	log.Debug("Ingest cache MISS: no cache for source", lga.Src, src)

	impl, err = d.OpenScratchFor(ctx, src)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	err = ingestFn(ctx, impl)
	elapsed := time.Since(start)

	if err != nil {
		log.Error("Ingest failed",
			lga.Src, src, lga.Dest, impl.Source(),
			lga.Elapsed, elapsed, lga.Err, err,
		)
		return nil, err
	}

	log.Debug("Ingest completed", lga.Src, src, lga.Dest, impl.Source(), lga.Elapsed, elapsed)

	// Write the checksums file.
	var sum ioz.Checksum
	if sum, err = ioz.FileChecksum(ingestFilePath); err != nil {
		log.Warn("Failed to compute checksum for source file; caching not in effect",
			lga.Src, src, lga.Dest, impl.Source(), lga.Path, ingestFilePath, lga.Err, err)
		return impl, nil //nolint:nilerr
	}

	if err = ioz.WriteChecksumFile(checksumsPath, sum, ingestFilePath); err != nil {
		log.Warn("Failed to write checksum; file caching not in effect",
			lga.Src, src, lga.Dest, impl.Source(), lga.Path, ingestFilePath, lga.Err, err)
	}

	return impl, nil
}

// getCachePaths returns the paths to the cache files for src.
// There is no guarantee that these files exist, or are accessible.
// It's just the paths.
func (d *Pools) getCachePaths(src *source.Source) (srcCacheDir, cacheDB, checksums string, err error) {
	if srcCacheDir, err = source.CacheDirFor(src); err != nil {
		return "", "", "", err
	}

	checksums = filepath.Join(srcCacheDir, "checksums.txt")
	cacheDB = filepath.Join(srcCacheDir, "cached.db")
	return srcCacheDir, cacheDB, checksums, nil
}

// acquireLock acquires a lock for src. The caller
// is responsible for unlocking the lock, e.g.:
//
//	defer lg.WarnIfFuncError(d.log, "failed to unlock cache lock", lock.Unlock)
//
// The lock acquisition process is retried with backoff.
func (d *Pools) acquireLock(ctx context.Context, src *source.Source) (lockfile.Lockfile, error) {
	lock, err := d.getLockfileFor(src)
	if err != nil {
		return "", err
	}

	err = retry.Do(ctx, time.Second*5,
		lock.TryLock,
		func(err error) bool {
			var temporaryError lockfile.TemporaryError
			return errors.As(err, &temporaryError)
		},
	)
	if err != nil {
		return "", errz.Wrap(err, "failed to get lock")
	}

	lg.FromContext(ctx).Debug("Acquired cache lock", "lock", lock)
	return lock, nil
}

// getLockfileFor returns a lockfile for src. It doesn't
// actually acquire the lock.
func (d *Pools) getLockfileFor(src *source.Source) (lockfile.Lockfile, error) {
	srcCacheDir, _, _, err := d.getCachePaths(src)
	if err != nil {
		return "", err
	}

	if err = os.MkdirAll(srcCacheDir, 0o750); err != nil {
		return "", errz.Err(err)
	}
	lockPath := filepath.Join(srcCacheDir, "pid.lock")
	return lockfile.New(lockPath)
}

// OpenCachedFor implements ScratchPoolOpener.
func (d *Pools) OpenCachedFor(ctx context.Context, src *source.Source) (Pool, bool, error) {
	_, cacheDBPath, checksumsPath, err := d.getCachePaths(src)
	if err != nil {
		return nil, false, err
	}

	if !ioz.FileAccessible(checksumsPath) {
		return nil, false, nil
	}

	mChecksums, err := ioz.ReadChecksumsFile(checksumsPath)
	if err != nil {
		return nil, false, err
	}

	drvr, err := d.drvrs.DriverFor(src.Type)
	if err != nil {
		return nil, false, err
	}

	if drvr.DriverMetadata().IsSQL {
		return nil, false, errz.Errorf("open file cache for source %s: driver {%s} is SQL, not document",
			src.Handle, src.Type)
	}

	srcFilepath, err := d.files.Filepath(ctx, src)
	if err != nil {
		return nil, false, err
	}
	d.log.Debug("Got srcFilepath for src",
		lga.Src, src, lga.Path, srcFilepath)

	cachedChecksum, ok := mChecksums[srcFilepath]
	if !ok {
		return nil, false, nil
	}

	srcChecksum, err := ioz.FileChecksum(srcFilepath)
	if err != nil {
		return nil, false, err
	}

	if srcChecksum != cachedChecksum {
		return nil, false, nil
	}

	// The checksums match, so we can use the cached DB,
	// if it exists.
	if !ioz.FileAccessible(cacheDBPath) {
		return nil, false, nil
	}

	backingType, err := d.files.DriverType(ctx, cacheDBPath)
	if err != nil {
		return nil, false, err
	}

	backingSrc := &source.Source{
		Handle:   src.Handle + "_cached",
		Location: "sqlite3://" + cacheDBPath,
		Type:     backingType,
	}

	backingPool, err := d.doOpen(ctx, backingSrc)
	if err != nil {
		return nil, false, errz.Wrapf(err, "open cached DB for source %s", src.Handle)
	}

	return backingPool, true, nil
}

// OpenJoin opens an appropriate database for use as
// a work DB for joining across sources.
//
// Note: There is much work to be done on this method. At this time, only
// two sources are supported. Ultimately OpenJoin should be able to
// inspect the join srcs and use heuristics to determine the best
// location for the join to occur (to minimize copying of data for
// the join etc.). Currently the implementation simply delegates
// to OpenScratch.
//
// OpenJoin implements JoinPoolOpener.
func (d *Pools) OpenJoin(ctx context.Context, srcs ...*source.Source) (Pool, error) {
	var names []string
	for _, src := range srcs {
		names = append(names, src.Handle[1:])
	}

	d.log.Debug("OpenJoin", "sources", strings.Join(names, ","))
	return d.OpenScratchFor(ctx, srcs[0])
}

// Close closes d, invoking Close on any instances opened via d.Open.
func (d *Pools) Close() error {
	d.log.Debug("Closing databases(s)...", lga.Count, d.clnup.Len())
	return d.clnup.Run()
}

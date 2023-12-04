package driver

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/source/drivertype"

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

var _ GripOpener = (*Grips)(nil)

// ScratchSrcFunc is a function that returns a scratch source.
// The caller is responsible for invoking cleanFn.
type ScratchSrcFunc func(ctx context.Context, name string) (src *source.Source, cleanFn func() error, err error)

// Grips provides a mechanism for getting Grip instances.
// Note that at this time instances returned by Open are cached
// and then closed by Close. This may be a bad approach.
type Grips struct {
	log          *slog.Logger
	drvrs        Provider
	mu           sync.Mutex
	scratchSrcFn ScratchSrcFunc
	files        *source.Files
	grips        map[string]Grip
	clnup        *cleanup.Cleanup
}

// NewGrips returns a Grips instances.
func NewGrips(log *slog.Logger, drvrs Provider,
	files *source.Files, scratchSrcFn ScratchSrcFunc,
) *Grips {
	return &Grips{
		log:          log,
		drvrs:        drvrs,
		mu:           sync.Mutex{},
		scratchSrcFn: scratchSrcFn,
		files:        files,
		grips:        map[string]Grip{},
		clnup:        cleanup.New(),
	}
}

// Open returns an opened Grip for src. The returned Grip
// may be cached and returned on future invocations for the
// same source (where each source fields is identical).
// Thus, the caller should typically not close
// the Grip: it will be closed via d.Close.
//
// NOTE: This entire logic re caching/not-closing is a bit sketchy,
// and needs to be revisited.
//
// Open implements GripOpener.
func (gs *Grips) Open(ctx context.Context, src *source.Source) (Grip, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)
	gs.mu.Lock()
	defer gs.mu.Unlock()
	return gs.doOpen(ctx, src)
}

// DriverFor returns the driver for typ.
func (gs *Grips) DriverFor(typ drivertype.Type) (Driver, error) {
	return gs.drvrs.DriverFor(typ)
}

// IsSQLSource returns true if src's driver is a SQLDriver.
func (gs *Grips) IsSQLSource(src *source.Source) bool {
	if src == nil {
		return false
	}

	drvr, err := gs.drvrs.DriverFor(src.Type)
	if err != nil {
		return false
	}

	if _, ok := drvr.(SQLDriver); ok {
		return true
	}

	return false
}

func (gs *Grips) getKey(src *source.Source) string {
	return src.Handle + "_" + src.Hash()
}

func (gs *Grips) doOpen(ctx context.Context, src *source.Source) (Grip, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)
	key := gs.getKey(src)

	grip, ok := gs.grips[key]
	if ok {
		return grip, nil
	}

	drvr, err := gs.drvrs.DriverFor(src.Type)
	if err != nil {
		return nil, err
	}

	baseOptions := options.FromContext(ctx)
	o := options.Merge(baseOptions, src.Options)

	ctx = options.NewContext(ctx, o)
	grip, err = drvr.Open(ctx, src)
	if err != nil {
		return nil, err
	}

	gs.clnup.AddC(grip)

	gs.grips[key] = grip
	return grip, nil
}

// OpenScratch returns a scratch database instance. It is not
// necessary for the caller to close the returned Grip as
// its Close method will be invoked by d.Close.
func (gs *Grips) OpenScratch(ctx context.Context, src *source.Source) (Grip, error) {
	const msgCloseScratch = "Close scratch db"

	cacheDir, srcCacheDBFilepath, _, err := gs.getCachePaths(src)
	if err != nil {
		return nil, err
	}

	if err = ioz.RequireDir(cacheDir); err != nil {
		return nil, err
	}

	scratchSrc, cleanFn, err := gs.scratchSrcFn(ctx, srcCacheDBFilepath)
	if err != nil {
		// if err is non-nil, cleanup is guaranteed to be nil
		return nil, err
	}
	gs.log.Debug("Opening scratch src", lga.Src, scratchSrc)

	backingDrvr, err := gs.drvrs.DriverFor(scratchSrc.Type)
	if err != nil {
		lg.WarnIfFuncError(gs.log, msgCloseScratch, cleanFn)
		return nil, err
	}

	var backingGrip Grip
	backingGrip, err = backingDrvr.Open(ctx, scratchSrc)
	if err != nil {
		lg.WarnIfFuncError(gs.log, msgCloseScratch, cleanFn)
		return nil, err
	}

	allowCache := OptIngestCache.Get(options.FromContext(ctx))
	if !allowCache {
		// If the ingest cache is disabled, we add the cleanup func
		// so the scratch DB is deleted when the session ends.
		gs.clnup.AddE(cleanFn)
	}

	return backingGrip, nil
}

// OpenIngest implements driver.GripOpenIngester.
func (gs *Grips) OpenIngest(ctx context.Context, src *source.Source, allowCache bool,
	ingestFn func(ctx context.Context, dest Grip) error,
) (Grip, error) {
	var grip Grip
	var err error

	if !allowCache || src.Handle == source.StdinHandle {
		// We don't currently cache stdin. Probably we never will?
		grip, err = gs.openIngestNoCache(ctx, src, ingestFn)
	} else {
		grip, err = gs.openIngestCache(ctx, src, ingestFn)
	}

	if err != nil {
		return nil, err
	}

	return grip, nil
}

func (gs *Grips) openIngestNoCache(ctx context.Context, src *source.Source,
	ingestFn func(ctx context.Context, destGrip Grip) error,
) (Grip, error) {
	log := lg.FromContext(ctx)
	impl, err := gs.OpenScratch(ctx, src)
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
		lg.WarnIfCloseError(log, lgm.CloseDB, impl)
		return nil, err
	}

	gs.log.Info("Ingest completed",
		lga.Src, src, lga.Dest, impl.Source(),
		lga.Elapsed, elapsed)
	return impl, nil
}

func (gs *Grips) openIngestCache(ctx context.Context, src *source.Source,
	ingestFn func(ctx context.Context, destGrip Grip) error,
) (Grip, error) {
	log := lg.FromContext(ctx)

	lock, err := gs.acquireLock(ctx, src)
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

	cacheDir, _, checksumsPath, err := gs.getCachePaths(src)
	if err != nil {
		return nil, err
	}

	if err = ioz.RequireDir(cacheDir); err != nil {
		return nil, err
	}

	log.Debug("Using cache dir", lga.Path, cacheDir)

	ingestFilePath, err := gs.files.Filepath(ctx, src)
	if err != nil {
		return nil, err
	}

	var (
		impl        Grip
		foundCached bool
	)
	if impl, foundCached, err = gs.openCachedFor(ctx, src); err != nil {
		return nil, err
	}
	if foundCached {
		log.Debug("Ingest cache HIT: found cached copy of source",
			lga.Src, src, "cached", impl.Source(),
		)
		return impl, nil
	}

	log.Debug("Ingest cache MISS: no cache for source", lga.Src, src)

	impl, err = gs.OpenScratch(ctx, src)
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
		lg.WarnIfCloseError(log, lgm.CloseDB, impl)
		return nil, err
	}

	log.Info("Ingest completed", lga.Src, src, lga.Dest, impl.Source(), lga.Elapsed, elapsed)

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
func (gs *Grips) getCachePaths(src *source.Source) (srcCacheDir, cacheDB, checksums string, err error) {
	if srcCacheDir, err = gs.files.CacheDirFor(src); err != nil {
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
func (gs *Grips) acquireLock(ctx context.Context, src *source.Source) (lockfile.Lockfile, error) {
	lock, err := gs.getLockfileFor(src)
	if err != nil {
		return "", err
	}

	err = retry.Do(ctx, time.Second*5,
		func() error {
			lg.FromContext(ctx).Debug("Attempting to acquire cache lock", lga.Lock, lock)
			return lock.TryLock()
		},
		func(err error) bool {
			var temporaryError lockfile.TemporaryError
			return errors.As(err, &temporaryError)
		},
	)
	if err != nil {
		return "", errz.Wrap(err, "failed to get lock")
	}

	lg.FromContext(ctx).Debug("Acquired cache lock", lga.Lock, lock)
	return lock, nil
}

// getLockfileFor returns a lockfile for src. It doesn't
// actually acquire the lock.
func (gs *Grips) getLockfileFor(src *source.Source) (lockfile.Lockfile, error) {
	srcCacheDir, _, _, err := gs.getCachePaths(src)
	if err != nil {
		return "", err
	}

	if err = ioz.RequireDir(srcCacheDir); err != nil {
		return "", err
	}
	lockPath := filepath.Join(srcCacheDir, "pid.lock")
	return lockfile.New(lockPath)
}

func (gs *Grips) openCachedFor(ctx context.Context, src *source.Source) (Grip, bool, error) {
	_, cacheDBPath, checksumsPath, err := gs.getCachePaths(src)
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

	drvr, err := gs.drvrs.DriverFor(src.Type)
	if err != nil {
		return nil, false, err
	}

	if drvr.DriverMetadata().IsSQL {
		return nil, false, errz.Errorf("open file cache for source %s: driver {%s} is SQL, not document",
			src.Handle, src.Type)
	}

	// FIXME: Not too sure invoking files.Filepath here is the right approach?
	srcFilepath, err := gs.files.Filepath(ctx, src)
	if err != nil {
		return nil, false, err
	}
	gs.log.Debug("Got srcFilepath for src",
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

	backingType, err := gs.files.DriverType(ctx, cacheDBPath)
	if err != nil {
		return nil, false, err
	}

	backingSrc := &source.Source{
		Handle:   src.Handle + "_cached",
		Location: "sqlite3://" + cacheDBPath,
		Type:     backingType,
	}

	backingGrip, err := gs.doOpen(ctx, backingSrc)
	if err != nil {
		return nil, false, errz.Wrapf(err, "open cached DB for source %s", src.Handle)
	}

	return backingGrip, true, nil
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
func (gs *Grips) OpenJoin(ctx context.Context, srcs ...*source.Source) (Grip, error) {
	var names []string
	for _, src := range srcs {
		names = append(names, src.Handle[1:])
	}

	gs.log.Debug("OpenJoin", "sources", strings.Join(names, ","))
	return gs.OpenScratch(ctx, srcs[0])
}

// Close closes d, invoking Close on any instances opened via d.Open.
func (gs *Grips) Close() error {
	gs.log.Debug("Closing databases(s)...", lga.Count, gs.clnup.Len())
	return gs.clnup.Run()
}

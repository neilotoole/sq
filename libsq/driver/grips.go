package driver

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
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
	return src.Handle
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
// its Close method will be invoked by Grips.Close.
func (gs *Grips) OpenScratch(ctx context.Context, src *source.Source) (Grip, error) {
	const msgCloseScratch = "Close scratch db"

	cacheDir, srcCacheDBFilepath, _, err := gs.files.CachePaths(src)
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
	log := lg.FromContext(ctx).With(lga.Handle, src.Handle)
	ctx = lg.NewContext(ctx, log)

	lock, err := gs.files.CacheLockFor(src)
	if err != nil {
		return nil, err
	}

	lockTimeout := source.OptCacheLockTimeout.Get(options.FromContext(ctx))
	bar := progress.FromContext(ctx).NewTimeoutWaiter(
		src.Handle+": acquire lock",
		time.Now().Add(lockTimeout),
	)

	err = lock.Lock(ctx, lockTimeout)
	bar.Stop()
	if err != nil {
		return nil, errz.Wrap(err, src.Handle+": acquire cache lock")
	}

	defer func() {
		if err = lock.Unlock(); err != nil {
			log.Warn("Failed to release cache lock", lga.Lock, lock, lga.Err, err)
		}
	}()

	var impl Grip
	var foundCached bool
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

	if err = gs.files.WriteIngestChecksum(ctx, src, impl.Source()); err != nil {
		log.Warn("Failed to write checksum for source file; caching not in effect",
			lga.Src, src, lga.Dest, impl.Source(), lga.Err, err)
	}

	return impl, nil
}

// openCachedFor returns the cached backing grip for src.
// If not cached, exists returns false.
func (gs *Grips) openCachedFor(ctx context.Context, src *source.Source) (backingGrip Grip, exists bool, err error) {
	var backingSrc *source.Source
	backingSrc, exists, err = gs.files.CachedBackingSourceFor(ctx, src)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}

	if backingGrip, err = gs.doOpen(ctx, backingSrc); err != nil {
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

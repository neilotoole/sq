package driver

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// ScratchSrcFunc is a function that returns a scratch source.
// The caller is responsible for invoking cleanFn.
type ScratchSrcFunc func(ctx context.Context, name string) (src *source.Source, cleanFn func() error, err error)

// Grips provides a mechanism for getting Grip instances.
// Note that at this time instances returned by Open are cached
// and then closed by Close. This may be a bad approach.
type Grips struct {
	drvrs        Provider
	mu           sync.Mutex
	scratchSrcFn ScratchSrcFunc
	files        *source.Files
	grips        map[string]Grip
	clnup        *cleanup.Cleanup
}

// NewGrips returns a Grips instances.
func NewGrips(drvrs Provider, files *source.Files, scratchSrcFn ScratchSrcFunc) *Grips {
	return &Grips{
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

	g, err := gs.doOpen(ctx, src)
	if err != nil {
		return nil, err
	}
	gs.clnup.AddC(g)
	return g, nil
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

	gs.grips[key] = grip
	return grip, nil
}

// OpenEphemeral returns an ephemeral scratch Grip instance. It is not
// necessary for the caller to close the returned Grip as
// its Close method will be invoked by Grips.Close.
func (gs *Grips) OpenEphemeral(ctx context.Context) (Grip, error) {
	const msgCloseDB = "Close ephemeral db"
	gs.mu.Lock()
	defer gs.mu.Unlock()
	log := lg.FromContext(ctx)

	dir := filepath.Join(gs.files.TempDir(), fmt.Sprintf("ephemeraldb_%s_%d", stringz.Uniq8(), os.Getpid()))
	if err := ioz.RequireDir(dir); err != nil {
		return nil, err
	}

	clnup := cleanup.New()
	clnup.AddE(func() error {
		return errz.Wrap(os.RemoveAll(dir), "remove ephemeral db dir")
	})

	fp := filepath.Join(dir, "ephemeral.sqlite.db")
	src, cleanFn, err := gs.scratchSrcFn(ctx, fp)
	if err != nil {
		// if err is non-nil, cleanup is guaranteed to be nil
		return nil, err
	}
	src.Handle = "@ephemeral_" + stringz.Uniq8()

	clnup.AddE(cleanFn)
	drvr, err := gs.drvrs.DriverFor(src.Type)
	if err != nil {
		lg.WarnIfFuncError(log, msgCloseDB, clnup.Run)
		return nil, err
	}

	var grip Grip
	if grip, err = drvr.Open(ctx, src); err != nil {
		lg.WarnIfFuncError(log, msgCloseDB, clnup.Run)
		return nil, err
	}

	g := &cleanOnCloseGrip{
		Grip:  grip,
		clnup: clnup,
	}
	gs.clnup.AddC(g)
	log.Info("Opened ephemeral db", lga.Src, g.Source())
	return g, nil
}

func (gs *Grips) openNewCacheGrip(ctx context.Context, src *source.Source) (grip Grip,
	cleanFn func() error, err error,
) {
	const msgRemoveScratch = "Remove cache db"
	log := lg.FromContext(ctx)

	cacheDir, srcCacheDBFilepath, _, err := gs.files.CachePaths(src)
	if err != nil {
		return nil, nil, err
	}

	if err = ioz.RequireDir(cacheDir); err != nil {
		return nil, nil, err
	}

	scratchSrc, cleanFn, err := gs.scratchSrcFn(ctx, srcCacheDBFilepath)
	if err != nil {
		// if err is non-nil, cleanup is guaranteed to be nil
		return nil, nil, err
	}
	log.Debug("Opening scratch cache src", lga.Src, scratchSrc)

	backingDrvr, err := gs.drvrs.DriverFor(scratchSrc.Type)
	if err != nil {
		lg.WarnIfFuncError(log, msgRemoveScratch, cleanFn)
		return nil, nil, err
	}

	var backingGrip Grip
	if backingGrip, err = backingDrvr.Open(ctx, scratchSrc); err != nil {
		lg.WarnIfFuncError(log, msgRemoveScratch, cleanFn)
		// The os.Remove call may be unnecessary, but doesn't hurt.
		lg.WarnIfError(log, msgRemoveScratch, os.Remove(srcCacheDBFilepath))
		return nil, nil, err
	}

	log.Info("Opened new cache db", lga.Src, backingGrip.Source())
	return backingGrip, cleanFn, nil
}

// OpenIngest implements driver.GripOpenIngester. It opens a Grip, ingesting
// the source's data into the Grip. If allowCache is true, the ingest cache DB
// is used if possible. If allowCache is false, any existing ingest cache DB
// is not utilized, and is overwritten by the ingestion process.
func (gs *Grips) OpenIngest(ctx context.Context, src *source.Source, allowCache bool,
	ingestFn func(ctx context.Context, dest Grip) error,
) (Grip, error) {
	log := lg.FromContext(ctx).With(lga.Handle, src.Handle)
	ctx = lg.NewContext(ctx, log)
	unlock, err := gs.files.CacheLockAcquire(ctx, src)
	if err != nil {
		return nil, err
	}
	defer unlock()

	if !allowCache || src.Handle == source.StdinHandle {
		// Note that we can never cache stdin, because it's a stream
		// that is effectively unique each time.
		return gs.openIngestGripNoCache(ctx, src, ingestFn)
	}

	return gs.openIngestGripCache(ctx, src, ingestFn)
}

func (gs *Grips) openIngestGripNoCache(ctx context.Context, src *source.Source,
	ingestFn func(ctx context.Context, destGrip Grip) error,
) (Grip, error) {
	log := lg.FromContext(ctx)

	// Clear any existing ingest cache (but don't delete any downloads).
	// Note that we have already acquired the cache lock at this point.
	if err := gs.files.CacheClearSource(ctx, src, false); err != nil {
		return nil, err
	}

	impl, cleanFn, err := gs.openNewCacheGrip(ctx, src)
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
		lg.WarnIfFuncError(log, "Remove cache DB after failed ingest", cleanFn)
		return nil, err
	}

	// Because this is a no-cache situation, we need to clear the
	// cache db on close.
	g := &cleanOnCloseGrip{
		Grip:  impl,
		clnup: cleanup.New().AddE(cleanFn),
	}

	log.Info("Ingest completed",
		lga.Src, src, lga.Dest, g.Source(),
		lga.Elapsed, elapsed)
	return g, nil
}

func (gs *Grips) openIngestGripCache(ctx context.Context, src *source.Source,
	ingestFn func(ctx context.Context, destGrip Grip) error,
) (Grip, error) {
	log := lg.FromContext(ctx)

	var impl Grip
	var foundCached bool
	var err error
	if impl, foundCached, err = gs.openCachedGripFor(ctx, src); err != nil {
		return nil, err
	}
	if foundCached {
		log.Debug("Ingest cache HIT: found cached copy of source",
			lga.Src, src, "cached", impl.Source(),
		)
		return impl, nil
	}

	log.Debug("Ingest cache MISS: no cache for source", lga.Src, src)

	var cleanFn func() error
	impl, cleanFn, err = gs.openNewCacheGrip(ctx, src)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	err = ingestFn(ctx, impl)
	elapsed := time.Since(start)

	if err != nil {
		log.Error("Ingest failed", lga.Src, src, lga.Dest, impl.Source(), lga.Elapsed, elapsed, lga.Err, err)
		lg.WarnIfCloseError(log, lgm.CloseDB, impl)
		lg.WarnIfFuncError(log, "Remove cache DB after failed ingest", cleanFn)
		return nil, err
	}

	log.Info("Ingest completed", lga.Src, src, lga.Dest, impl.Source(), lga.Elapsed, elapsed)

	if err = gs.files.WriteIngestChecksum(ctx, src, impl.Source()); err != nil {
		log.Error("Failed to write checksum for cache DB",
			lga.Src, src, lga.Dest, impl.Source(), lga.Err, err)
		lg.WarnIfCloseError(log, lgm.CloseDB, impl)
		lg.WarnIfFuncError(log, "Remove cache DB after failed ingest checksum write", cleanFn)
		return nil, err
	}

	return impl, nil
}

// openCachedGripFor returns the cached backing grip for src.
// If not cached, exists returns false.
func (gs *Grips) openCachedGripFor(ctx context.Context, src *source.Source) (backingGrip Grip, exists bool, err error) {
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

// OpenJoin opens an appropriate Grip for use as
// a work DB for joining across sources.
//
// REVISIT: There is much work to be done on this method. Ultimately OpenJoin
// should be able to inspect the join srcs and use heuristics to determine
// the best location for the join to occur (to minimize copying of data for
// the join etc.).
func (gs *Grips) OpenJoin(ctx context.Context, srcs ...*source.Source) (Grip, error) {
	const msgCloseJoinDB = "Close join db"
	gs.mu.Lock()
	defer gs.mu.Unlock()

	log := lg.FromContext(ctx)

	var buf bytes.Buffer
	for _, src := range srcs {
		buf.WriteString(src.Handle)
	}

	sum := checksum.Sum(buf.Bytes())
	dir := filepath.Join(gs.files.TempDir(), fmt.Sprintf("joindb_%s_%s_%d", sum, stringz.Uniq8(), os.Getpid()))
	if err := ioz.RequireDir(dir); err != nil {
		return nil, err
	}

	clnup := cleanup.New()
	clnup.AddE(func() error {
		err := errz.Wrap(os.RemoveAll(dir), "remove join db dir")
		if err != nil {
			lg.FromContext(ctx).Warn("Failed to remove join db dir", lga.Path, dir, lga.Err, err)
			return err
		}

		lg.FromContext(ctx).Debug("Removed join db dir", lga.Path, dir)
		return nil
	})

	fp := filepath.Join(dir, "join.sqlite.db")
	joinSrc, cleanFn, err := gs.scratchSrcFn(ctx, fp)
	if err != nil {
		// if err is non-nil, cleanup is guaranteed to be nil
		return nil, err
	}
	joinSrc.Handle = "@join_" + stringz.Uniq8()

	clnup.AddE(cleanFn)
	drvr, err := gs.drvrs.DriverFor(joinSrc.Type)
	if err != nil {
		lg.WarnIfFuncError(log, msgCloseJoinDB, clnup.Run)
		return nil, err
	}

	log.Debug("Opening join db", lga.Path, fp)
	var grip Grip
	if grip, err = drvr.Open(ctx, joinSrc); err != nil {
		lg.WarnIfFuncError(log, msgCloseJoinDB, clnup.Run)
		return nil, err
	}

	g := &cleanOnCloseGrip{
		Grip:  grip,
		clnup: clnup,
	}
	gs.clnup.AddC(g)
	return g, nil
}

// Close closes gs, invoking any cleanup funcs.
func (gs *Grips) Close() error {
	return gs.clnup.Run()
}

var _ Grip = (*cleanOnCloseGrip)(nil)

// cleanOnCloseGrip is Grip decorator, invoking clnup after the backing Grip is
// closed, thus permitting arbitrary cleanup on Grip.Close. Subsequent
// invocations of Close are no-ops and return the same error.
type cleanOnCloseGrip struct {
	Grip
	once     sync.Once
	closeErr error
	clnup    *cleanup.Cleanup
}

// Close implements Grip. It invokes the underlying Grip's Close
// method, and then the closeFn, returning a combined error.
func (g *cleanOnCloseGrip) Close() error {
	g.once.Do(func() {
		g.closeErr = errz.Append(g.Grip.Close(), g.clnup.Run())
	})
	return g.closeErr
}

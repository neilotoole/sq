package driver

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
)

// ScratchSrcFunc is a function that returns a scratch source.
// The caller is responsible for invoking cleanFn.
type ScratchSrcFunc func(ctx context.Context, name string) (src *source.Source, cleanFn func() error, err error)

// Grips provides a mechanism for getting Grip instances.
// Note that at this time instances returned by Open are cached
// and then closed by Close. This may be a bad approach.
type Grips struct {
	drvrs        Provider
	closeErr     error
	scratchSrcFn ScratchSrcFunc

	files *files.Files

	// secretReg resolves ${scheme:path} placeholders in source Locations
	// at open time (see ResolveSourceSecrets). It may be nil, in which
	// case the first (uncached) open of a source whose Location contains
	// placeholders fails; cache hits are served before resolution runs.
	secretReg *secret.Registry

	// grips caches open Grip instances, keyed by gripCacheKey: the source
	// handle plus the access mode (read-write, implicit read-only, or
	// explicit read-only) passed to Open. Keying on the mode means a
	// source opened both read-only and read-write within one run gets two
	// coexisting grips, each opened with the correct mode, regardless of
	// which open happened first (gh #779). Both grips are registered on
	// clnup, so Close releases them all.
	grips map[string]Grip

	clnup     *cleanup.Cleanup
	closeOnce sync.Once
	mu        sync.Mutex
}

// gripCacheKey returns the gs.grips cache key for opening the source
// with the given handle in mode: the handle plus the access mode.
// Explicit read-only (e.g. sq sql --readonly) is distinct from the
// implicit hint because drivers may treat it more forcefully (DuckDB
// overrides access_mode=AUTOMATIC only for an explicit hint). The key is
// derived from the handle alone, never the location, so computing it
// requires no secret resolution. Handles cannot contain NUL, so the
// separator is unambiguous.
func gripCacheKey(mode AccessMode, handle string) string {
	return handle + "\x00" + mode.suffix()
}

// NewGrips returns a Grips instance. secretReg resolves secret
// placeholders in source Locations at open time; it may be nil if no
// source uses placeholders.
func NewGrips(drvrs Provider, fs *files.Files, secretReg *secret.Registry,
	scratchSrcFn ScratchSrcFunc,
) *Grips {
	return &Grips{
		drvrs:        drvrs,
		mu:           sync.Mutex{},
		scratchSrcFn: scratchSrcFn,
		files:        fs,
		secretReg:    secretReg,
		grips:        map[string]Grip{},
		clnup:        cleanup.New(),
	}
}

// Open returns an opened Grip for src. The returned Grip may be cached and
// returned on future invocations for the same source handle and access
// mode. The cache key is the handle plus mode only: it deliberately
// ignores src.Location and src.Options, so a second Open of the same
// handle in the same mode returns the existing grip even if those fields
// differ. Thus, the caller should typically not close the Grip: it will
// be closed via d.Close.
//
// The access mode is given by mode (pass ModeReadWrite for a normal open,
// ModeReadOnly or ModeReadOnlyExplicit for a read-only open). The cache
// is keyed by source handle and mode, so a read-only open and a
// read-write open of the same source yield independent grips, each
// connected in the requested mode: call ordering across modes does not
// matter. A cache hit is served before secret resolution runs, so
// repeated opens of an already-open source don't repeat resolver work.
//
// Arg mode is forwarded to Driver.Open.
//
// NOTE: This entire logic re caching/not-closing is a bit sketchy,
// and needs to be revisited.
func (gs *Grips) Open(ctx context.Context, src *source.Source, mode AccessMode) (Grip, error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	g, err := gs.doOpen(ctx, src, mode)
	if err != nil {
		return nil, err
	}
	gs.clnup.AddC(g)
	gs.grips[gripCacheKey(mode, src.Handle)] = g
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

// ResolveSourceSecrets returns a clone of src with any ${scheme:path}
// placeholders in src.Location resolved via reg, and any $$ escapes
// reduced to a literal $. If src.Location contains no well-formed
// placeholders, the $$ escape is still honored (a clone is returned
// when the location changes; reg may be nil, since nothing resolves).
// This is what lets a literal location be stored escaped, e.g. by the
// v0.54.0 config upgrade, and reach the driver byte-identically. If
// placeholders are present but reg is nil, an error is returned: a
// placeholder on a source that has reached this point is always meant
// to be resolved, and a silent passthrough would surface later as a
// confusing "connection refused" or DSN-parse error from the driver.
//
// Detection uses secret.ExtractRefs rather than a `${`-substring scan
// so that literal "${" sequences (e.g. an escaped "$${env:X}" or a
// password that happens to contain "${") don't trigger resolution.
//
// When the location changes, the resolved value also receives the
// driver-specific canonicalization (location.MungeForDriver) that
// "sq add" applies to literal locations: a ${env:DB_PATH} placeholder
// resolving to a bare file path like /data/sakila.db becomes
// sqlite3:///data/sakila.db, which is the form the file-based drivers
// require. Munging is idempotent, so a secret value already in
// canonical form passes through unchanged. The munged form exists
// only on the returned clone; the stored template is never modified.
//
// ResolveSourceSecrets is idempotent: the returned clone is marked
// SecretsResolved, and an already-resolved source passes through
// unchanged, so accidental double-resolution cannot reinterpret
// resolved literal bytes as a template.
//
// Exposed (rather than unexported) so callers and tests can drive the
// resolution directly. Grips.doOpen calls it once at entry; drivers do
// not need to call it themselves.
func ResolveSourceSecrets(ctx context.Context, reg *secret.Registry,
	src *source.Source,
) (*source.Source, error) {
	if src == nil || src.SecretsResolved {
		// Already-resolved Locations hold literal bytes, not template
		// bytes; running them through resolution again would corrupt
		// any '$' they contain.
		return src, nil
	}
	refs, err := secret.ExtractRefs(src.Location)
	if err != nil {
		return nil, errz.Wrapf(err, "parse placeholders for %s", src.Handle)
	}

	var resolved string
	if len(refs) == 0 {
		if resolved = secret.Unescape(src.Location); resolved == src.Location {
			return src, nil
		}
	} else {
		if reg == nil {
			return nil, errz.Errorf("resolve placeholders for %s: no secret registry provided", src.Handle)
		}
		if resolved, err = reg.Expand(ctx, src.Location); err != nil {
			return nil, errz.Wrapf(err, "resolve secrets for %s", src.Handle)
		}
	}

	// At add time, a literal file-DB location gets driver-specific
	// munging (e.g. /data/sakila.db -> sqlite3:///data/sakila.db), but
	// a placeholder location is opaque then, so it's stored unmunged.
	// Apply the same munging to the resolved value here, on the clone
	// only: the stored template must remain untouched. MungeForDriver
	// is idempotent and a passthrough for non-file driver types, so an
	// already-canonical location is unaffected. Munging is gated on
	// resolution actually changing the bytes: if expansion is a no-op
	// (a secret value byte-identical to its own placeholder), the
	// still-placeholder-shaped string must not be reinterpreted as a
	// file path; it passes through for the driver to reject.
	if resolved != src.Location {
		if resolved, err = location.MungeForDriver(src.Type, resolved); err != nil {
			// Don't echo the resolved location: it is secret material.
			if len(refs) > 0 {
				return nil, errz.Wrapf(err, "source %s: invalid location after resolving placeholders", src.Handle)
			}
			return nil, errz.Wrapf(err, "source %s: invalid location", src.Handle)
		}
	}

	clone := src.Clone()
	clone.Location = resolved
	clone.SecretsResolved = true
	return clone, nil
}

func (gs *Grips) doOpen(ctx context.Context, src *source.Source, mode AccessMode) (Grip, error) {
	// The cache key derives from the handle and the access mode, not the
	// location, so the lookup happens before secret resolution: a cache
	// hit (e.g. one Grips.Open per table during inspect) must not pay for
	// resolution again.
	grip, ok := gs.grips[gripCacheKey(mode, src.Handle)]
	if ok {
		return grip, nil
	}

	var err error
	if src, err = ResolveSourceSecrets(ctx, gs.secretReg, src); err != nil {
		return nil, err
	}

	drvr, err := gs.drvrs.DriverFor(src.Type)
	if err != nil {
		return nil, err
	}

	baseOptions := options.FromContext(ctx)
	o := options.Merge(baseOptions, src.Options)

	ctx = options.NewContext(ctx, o)
	grip, err = drvr.Open(ctx, src, mode)
	if err != nil {
		return nil, err
	}

	return grip, nil
}

// OpenEphemeral returns an ephemeral scratch Grip instance. It is not
// necessary for the caller to close the returned Grip as its Close method
// will be invoked by Grips.Close.
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
	if grip, err = drvr.Open(ctx, src, ModeReadWrite); err != nil {
		lg.WarnIfFuncError(log, msgCloseDB, clnup.Run)
		return nil, err
	}

	g := &cleanOnCloseGrip{
		Grip:  grip,
		clnup: clnup,
	}
	gs.clnup.AddC(g)
	log.Info("Opened ephemeral db", lga.Src, g.Source())
	gs.grips[gripCacheKey(ModeReadWrite, g.Source().Handle)] = g
	return g, nil
}

func (gs *Grips) openNewCacheGrip(ctx context.Context, src *source.Source) (grip Grip,
	cleanFn func() error, err error,
) {
	const msgRemoveScratch = "Remove cache db"
	log := lg.FromContext(ctx)

	if err = gs.files.CacheClearSource(ctx, src, false); err != nil {
		return nil, nil, err
	}

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
	if backingGrip, err = backingDrvr.Open(ctx, scratchSrc, ModeReadWrite); err != nil {
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
		log.Error(
			"Ingest failed",
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
		log.Info(
			"Ingest cache HIT: found cached ingest DB",
			lga.Src, src, "cached", impl.Source(),
		)
		return impl, nil
	}

	log.Info("Ingest cache MISS: no cache for source", lga.Src, src)

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

	if backingGrip, err = gs.doOpen(ctx, backingSrc, ModeReadWrite); err != nil {
		return nil, false, errz.Wrapf(err, "open cached DB for source %s", src.Handle)
	}

	return backingGrip, true, nil
}

// OpenJoin opens an appropriate Grip for use as a work DB for joining across sources.
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
	if grip, err = drvr.Open(ctx, joinSrc, ModeReadWrite); err != nil {
		lg.WarnIfFuncError(log, msgCloseJoinDB, clnup.Run)
		return nil, err
	}

	g := &cleanOnCloseGrip{
		Grip:  grip,
		clnup: clnup,
	}
	gs.clnup.AddC(g)
	gs.grips[gripCacheKey(ModeReadWrite, g.Source().Handle)] = g
	return g, nil
}

// Close closes gs, invoking any cleanup funcs.
func (gs *Grips) Close() error {
	gs.closeOnce.Do(func() {
		gs.closeErr = gs.clnup.Run()
	})
	return gs.closeErr
}

var _ Grip = (*cleanOnCloseGrip)(nil)

// cleanOnCloseGrip is Grip decorator, invoking clnup after the backing Grip is
// closed, thus permitting arbitrary cleanup on Grip.Close. Subsequent
// invocations of Close are no-ops and return the same error.
type cleanOnCloseGrip struct {
	Grip
	closeErr error
	clnup    *cleanup.Cleanup
	once     sync.Once
}

// Close implements Grip. It invokes the underlying Grip's Close
// method, and then the closeFn, returning a combined error.
func (g *cleanOnCloseGrip) Close() error {
	g.once.Do(func() {
		err1 := g.Grip.Close()
		err2 := g.clnup.Run()
		g.closeErr = errz.Append(err1, err2)
	})
	return g.closeErr
}

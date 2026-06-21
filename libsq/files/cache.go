package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
)

// OptCacheLockTimeout is the time allowed to acquire a cache lock.
//
// See also: driver.OptIngestCache.
var OptCacheLockTimeout = options.NewDuration(
	"cache.lock.timeout",
	nil,
	time.Second*5,
	"Wait timeout to acquire cache lock",
	`Wait timeout to acquire cache lock. During this period, retry will occur
if the lock is already held by another process. If zero, no retry occurs.`,
)

// CacheDir returns the cache dir. It is not guaranteed that the
// returned dir exists.
func (fs *Files) CacheDir() string {
	return fs.cacheDir
}

// TempDir returns the temp dir. It is not guaranteed that the
// returned dir exists.
func (fs *Files) TempDir() string {
	return fs.tempDir
}

// CacheDirFor gets the cache dir for handle. It is not guaranteed
// that the returned dir exists or is accessible.
func (fs *Files) CacheDirFor(src *source.Source) (dir string, err error) {
	handle := src.Handle
	if err = source.ValidHandle(handle); err != nil {
		return "", errz.Wrapf(err, "cache dir: invalid handle: %s", handle)
	}

	if handle == source.StdinHandle {
		// stdin is different input every time, so we need a unique
		// cache dir. In practice, stdin probably isn't using this function.
		handle += "_" + stringz.UniqN(32)
	}

	return filepath.Join(fs.sourceHandleDir(handle), fs.sourceHash(src)), nil
}

// sourceHandleDir returns the parent dir of all of handle's cache dirs:
// CacheDirFor returns sourceHandleDir(handle)/<location-hash>. This is
// the single encoding of the handle-to-path layout; keep CacheDirFor and
// CacheClearSourceAll in sync by changing only this function.
func (fs *Files) sourceHandleDir(handle string) string {
	return filepath.Join(
		fs.cacheDir,
		"sources",
		filepath.Join(strings.Split(strings.TrimPrefix(handle, "@"), "/")...),
	)
}

// WriteIngestChecksum is invoked (after successful ingestion) to write the
// checksum of the source document file vs the ingest DB. Thus, if the source
// document changes, the checksum will no longer match, and the ingest DB
// will be considered invalid.
func (fs *Files) WriteIngestChecksum(ctx context.Context, src, backingSrc *source.Source) (err error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	log := lg.FromContext(ctx)
	ingestFilePath, err := fs.filepath(src)
	if err != nil {
		return err
	}

	if location.TypeOf(src.Location) == location.TypeHTTP {
		// If the source is remote, check if there was a download,
		// and if so, make sure it's completed.
		stream, ok := fs.streams[src.Handle]
		if ok {
			select {
			case <-stream.Filled():
			case <-stream.Done():
			case <-ctx.Done():
				return ctx.Err()
			}

			if err = stream.Err(); err != nil && !errors.Is(err, io.EOF) {
				return err
			}
		}

		// If we got this far, either there's no stream, or the stream is done,
		// which means that the download cache has been updated, and contains
		// the fresh cached body file that we'll use to calculate the checksum.
		// So, we'll go ahead and do the checksum stuff below.
	}

	// Now, we need to write a checksum file that contains the computed checksum
	// value from ingestFilePath.
	var sum checksum.Checksum
	if sum, err = checksum.ForFile(ingestFilePath); err != nil {
		log.Warn("Failed to compute checksum for source file; caching not in effect",
			lga.Src, src, lga.Dest, backingSrc, lga.Path, ingestFilePath, lga.Err, err)
		return err
	}

	var srcCacheDir, checksumsPath string
	if srcCacheDir, _, checksumsPath, err = fs.CachePaths(src); err != nil {
		return err
	}

	// checksum.WriteFile doesn't create parent dirs, so ensure the cache leaf
	// dir exists; otherwise the write fails when this is the first thing to
	// touch the leaf.
	if err = ioz.RequireDir(srcCacheDir); err != nil {
		return errz.Wrap(err, "write ingest checksum")
	}

	if err = checksum.WriteFile(checksumsPath, sum, ingestFilePath); err != nil {
		log.Warn("Failed to write checksum; file caching not in effect",
			lga.Src, src, lga.Dest, backingSrc, lga.Path, ingestFilePath, lga.Err, err)
	}
	return err
}

// CachedBackingSourceFor returns the underlying backing source for src, if
// it exists. If it does not exist, ok returns false.
func (fs *Files) CachedBackingSourceFor(ctx context.Context, src *source.Source) (backingSrc *source.Source,
	ok bool, err error,
) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	switch location.TypeOf(src.Location) {
	case location.TypeFile:
		return fs.cachedBackingSourceForFile(ctx, src)
	case location.TypeHTTP:
		return fs.cachedBackingSourceForRemoteFile(ctx, src)
	default:
		return nil, false, errz.Errorf("caching not applicable for source: %s", src.Handle)
	}
}

// cachedBackingSourceForFile returns the underlying cached backing
// source for src, if it exists.
func (fs *Files) cachedBackingSourceForFile(ctx context.Context, src *source.Source) (*source.Source, bool, error) {
	if _, dlFileExists := fs.downloadedFiles[src.Handle]; !dlFileExists {
		// It's NOT the case that the file has just been downloaded.
		// So, we check if there's an active download happening...
		if _, dlStreamExists := fs.streams[src.Handle]; dlStreamExists {
			lg.FromContext(ctx).Debug("Source has download stream, so backing cache DB not valid", lga.Src, src)
			return nil, false, nil
		}
	}

	_, cacheDBPath, checksumsPath, err := fs.CachePaths(src)
	if err != nil {
		return nil, false, err
	}

	if !ioz.FileAccessible(checksumsPath) {
		return nil, false, nil
	}

	mChecksums, err := checksum.ReadFile(checksumsPath)
	if err != nil {
		return nil, false, err
	}

	srcFilepath, err := fs.filepath(src)
	if err != nil {
		return nil, false, err
	}

	cachedChecksum, ok := mChecksums[srcFilepath]
	if !ok {
		return nil, false, nil
	}

	srcChecksum, err := checksum.ForFile(srcFilepath)
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

	backingSrc := &source.Source{
		Handle:   src.Handle + "_cached",
		Location: "sqlite3://" + cacheDBPath,
		Type:     drivertype.SQLite,
		// The cache path is an internally constructed literal, not a
		// placeholder template: mark it so resolution is a no-op.
		SecretsResolved: true,
	}

	lg.FromContext(ctx).Debug("Found cached backing source DB", lga.Src, src, "backing_src", backingSrc)
	return backingSrc, true, nil
}

// cachedBackingSourceForRemoteFile returns the underlying cached backing
// source for src, if it exists.
func (fs *Files) cachedBackingSourceForRemoteFile(ctx context.Context, src *source.Source) (*source.Source,
	bool, error,
) {
	log := lg.FromContext(ctx)

	downloadedFile, _, err := fs.maybeStartDownload(ctx, src, true)
	if err != nil {
		return nil, false, err
	}

	if downloadedFile == "" {
		log.Debug("No cached download file", lga.Src, src)
		return nil, false, nil
	}

	log.Debug("Found cached download file", lga.Src, src, lga.Path, downloadedFile)
	return fs.cachedBackingSourceForFile(ctx, src)
}

// CachePaths returns the paths to the cache files for src.
// There is no guarantee that these files exist, or are accessible.
// It's just the paths.
func (fs *Files) CachePaths(src *source.Source) (srcCacheDir, cacheDB, checksums string, err error) {
	if srcCacheDir, err = fs.CacheDirFor(src); err != nil {
		return "", "", "", err
	}

	checksums = filepath.Join(srcCacheDir, "checksums.txt")
	cacheDB = filepath.Join(srcCacheDir, "cache.sqlite.db")
	return srcCacheDir, cacheDB, checksums, nil
}

// sourceHash generates a hash for src. The hash is based on the
// member fields of src, with special handling for src.Options.
// Only the opts that affect data ingestion (options.TagIngestMutate)
// are incorporated in the hash. The generated hash is used to
// determine the cache dir for src. Thus, if a source is mutated
// (e.g. the remote http location changes), a new cache dir results.
func (fs *Files) sourceHash(src *source.Source) string {
	if src == nil {
		return ""
	}

	buf := bytes.Buffer{}
	buf.WriteString(src.Handle)
	buf.WriteString(string(src.Type))
	buf.WriteString(src.Location)
	buf.WriteString(src.Catalog)
	buf.WriteString(src.Schema)

	mUsedKeys := make(map[string]any)
	if src.Options != nil {
		keys := src.Options.Keys()
		for _, k := range keys {
			opt := fs.optRegistry.Get(k)
			switch {
			case opt == nil,
				!opt.IsSet(src.Options),
				!opt.HasTag(options.TagIngestMutate):
				continue
			default:
			}

			buf.WriteString(k)
			v := src.Options[k]
			fmt.Fprintf(&buf, "%v", v)
			mUsedKeys[k] = v
		}
	}

	sum := checksum.Sum(buf.Bytes())
	return sum
}

// cacheLockFor returns the lock file for src's cache.
func (fs *Files) cacheLockFor(src *source.Source) (lockfile.Lockfile, error) {
	cacheDir, err := fs.CacheDirFor(src)
	if err != nil {
		return "", errz.Wrapf(err, "cache lock for %s", src.Handle)
	}

	lf, err := lockfile.New(filepath.Join(cacheDir, "pid.lock"))
	if err != nil {
		return "", errz.Wrapf(err, "cache lock for %s", src.Handle)
	}

	return lf, nil
}

// CacheLockAcquire acquires the cache lock for src. The caller must invoke
// the returned unlock func.
func (fs *Files) CacheLockAcquire(ctx context.Context, src *source.Source) (unlock func(), err error) {
	lock, err := fs.cacheLockFor(src)
	if err != nil {
		return nil, err
	}

	return lockAcquire(ctx, lock, src.Handle)
}

// lockAcquire acquires lock, honoring [OptCacheLockTimeout] from ctx
// options and displaying a progress waiter labeled with label. The caller
// must invoke the returned unlock func.
func lockAcquire(ctx context.Context, lock lockfile.Lockfile, label string) (unlock func(), err error) {
	lockTimeout := OptCacheLockTimeout.Get(options.FromContext(ctx))
	log := lg.FromContext(ctx).With(lga.Timeout, lockTimeout, lga.Lock, lock)
	log.Debug("Acquiring cache lock")

	bar := progress.FromContext(ctx).NewTimeoutWaiter(
		label+": acquire lock",
		time.Now().Add(lockTimeout),
	)

	err = lock.Lock(ctx, lockTimeout)
	bar.Stop()
	if err != nil {
		return nil, errz.Wrap(err, label+": acquire cache lock")
	}

	return func() {
		if err = lock.Unlock(); err != nil {
			log.Warn("Failed to release cache lock", lga.Err, err)
		}
	}, nil
}

// CacheClearAll clears the entire cache dir.
// Note that this operation is distinct from Files.doCacheSweep.
func (fs *Files) CacheClearAll(ctx context.Context) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.doCacheClearAll(ctx)
}

// CacheClearSource clears the ingest cache for src. If arg downloads is true,
// the source's download dir is also cleared. The caller should typically
// first acquire the cache lock for src via Files.cacheLockFor.
//
// Note that this clears only the cache dir for src's current location
// hash (see CacheDirFor). It is used internally by ingest, which holds
// that dir's lock and knows the resolved location. For clearing a
// source's cache wholesale, see CacheClearSourceAll.
func (fs *Files) CacheClearSource(ctx context.Context, src *source.Source, clearDownloads bool) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.doCacheClearSource(ctx, src, clearDownloads)
}

// CacheClearSourceAll clears the ingest cache for src by removing every
// cache dir belonging to src.Handle, regardless of location hash. The
// per-source cache dir leaf (see CacheDirFor) is keyed on a hash that
// incorporates src.Location: for a location containing ${scheme:path}
// placeholders that's the resolved location, whose hash cannot be
// recomputed if the secret has rotated or become unavailable. Removing
// every leaf under the handle's dir catches them all, including leaves
// orphaned by changed options, catalog, or schema, and requires no
// secret resolution. Downloads are cleared too.
//
// Each leaf's cache lock is acquired (honoring [OptCacheLockTimeout])
// before that leaf is cleared, so a concurrent ingest is not disrupted.
//
// collHandles is the set of all handles in the active collection. It is
// needed because a handle may coincide with a group prefix of another
// handle (e.g. @prod and @prod/db/x), in which case the nested source's
// cache dirs live below this handle's dir and must be left untouched.
// Collection.Add and the sq mv paths now reject creating such nesting,
// but it can exist in configs created before that validation, or
// hand-edited YAML, which is never re-validated for nesting.
func (fs *Files) CacheClearSourceAll(ctx context.Context, src *source.Source, collHandles []string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	log := lg.FromContext(ctx)

	handle := src.Handle
	if err := source.ValidHandle(handle); err != nil {
		return errz.Wrapf(err, "clear cache: invalid handle: %s", handle)
	}

	handleDir := fs.sourceHandleDir(handle)
	if !ioz.DirExists(handleDir) {
		return nil
	}

	// Child dirs of handleDir are location-hash leaves, except where
	// another source's handle nests under this handle: its first path
	// segment below this handle names a child dir that must survive.
	nested := map[string]bool{}
	prefix := strings.TrimPrefix(handle, "@") + "/"
	for _, h := range collHandles {
		if hp := strings.TrimPrefix(h, "@"); strings.HasPrefix(hp, prefix) {
			nested[strings.SplitN(strings.TrimPrefix(hp, prefix), "/", 2)[0]] = true
		}
	}

	entries, err := os.ReadDir(handleDir)
	if err != nil {
		return errz.Wrapf(err, "%s: clear cache", handle)
	}

	for _, entry := range entries {
		if !entry.IsDir() || nested[entry.Name()] {
			continue
		}

		// Acquire the leaf's cache lock before touching it: a concurrent
		// ingest (Grips.OpenIngest) holds this lock for the duration of
		// the ingest, and must not have the cache yanked out from under
		// it mid-write.
		leafDir := filepath.Join(handleDir, entry.Name())
		lock, lockErr := lockfile.New(filepath.Join(leafDir, "pid.lock"))
		if lockErr != nil {
			return errz.Wrapf(lockErr, "%s: clear cache", handle)
		}
		unlock, lockErr := lockAcquire(ctx, lock, handle)
		if lockErr != nil {
			return lockErr
		}

		err = clearCacheDirContents(leafDir, true, handle)
		unlock()
		if err != nil {
			return err
		}

		// The leaf is now empty: its contents are cleared and unlock
		// removed pid.lock. Remove the dir itself, non-recursively: if a
		// concurrent process re-acquired the lock in the interim, the
		// dir is non-empty and Remove fails, which is fine, as the
		// leaf's cache contents are already gone.
		_ = os.Remove(leafDir)
	}

	// Best-effort: prune handleDir (and the handle's group dirs) if now
	// empty. Failure is fine: doCacheSweep prunes empty dirs eventually.
	_ = os.Remove(handleDir)

	log.With(lga.Src, src, lga.Dir, handleDir).Info("Cleared source cache")
	return nil
}

func (fs *Files) doCacheClearSource(ctx context.Context, src *source.Source, clearDownloads bool) error {
	cacheDir, err := fs.CacheDirFor(src)
	if err != nil {
		return err
	}

	if !ioz.DirExists(cacheDir) {
		return nil
	}

	if err = clearCacheDirContents(cacheDir, clearDownloads, src.Handle); err != nil {
		return err
	}

	lg.FromContext(ctx).
		With("clear_downloads", clearDownloads, lga.Src, src, lga.Dir, cacheDir).
		Info("Cleared source cache")
	return nil
}

// clearCacheDirContents removes the contents of the cache dir, always
// preserving pid.lock (which may be held, including by the caller), and
// preserving the download dir if clearDownloads is false. Arg handle is
// used for error messages only.
func clearCacheDirContents(cacheDir string, clearDownloads bool, handle string) error {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return errz.Wrapf(err, "%s: clear cache", handle)
	}

	for _, entry := range entries {
		switch entry.Name() {
		case "pid.lock":
			continue
		case "download":
			if !clearDownloads {
				continue
			}
		default:
		}

		if err = os.RemoveAll(filepath.Join(cacheDir, entry.Name())); err != nil {
			return errz.Wrapf(err, "%s: clear cache", handle)
		}
	}
	return nil
}

func (fs *Files) doCacheClearAll(ctx context.Context) error {
	log := lg.FromContext(ctx).With(lga.Dir, fs.cacheDir)
	log.Debug("Clearing cache dir")
	if !ioz.DirExists(fs.cacheDir) {
		log.Debug("Cache dir does not exist")
		return nil
	}

	// Instead of directly deleting the existing cache dir, we first move it
	// aside, then delete it. This should help with the situation where
	// another sq instance has an open pid lock in the cache dir.
	//
	// The relocation target is a sibling of the cache dir (same parent, hence
	// same filesystem) rather than the global temp dir: os.Rename can't move
	// across filesystems (EXDEV), and the cache dir and os.TempDir are
	// routinely on different mounts (e.g. ~/.cache vs a tmpfs /tmp).
	parent := filepath.Dir(fs.cacheDir)
	if err := ioz.RequireDir(parent); err != nil {
		return errz.Wrap(err, "cache clear")
	}
	relocateDir := filepath.Join(parent, "dead_cache_"+stringz.Uniq8())
	if err := os.Rename(fs.cacheDir, relocateDir); err != nil {
		return errz.Wrap(err, "cache clear: relocate")
	}

	if err := os.RemoveAll(relocateDir); err != nil {
		log.Warn("Could not delete relocated cache dir", lga.Path, relocateDir, lga.Err, err)
	}

	// Recreate the cache dir.
	if err := ioz.RequireDir(fs.cacheDir); err != nil {
		return errz.Wrap(err, "cache clear")
	}

	return nil
}

// doCacheSweep sweeps the cache dir, making a best-effort attempt
// to remove any empty directories. Note that this operation is
// distinct from Files.CacheClearAll.
//
// REVISIT: This doesn't really do as much as desired. It should
// also be able to detect orphaned src cache dirs and delete those.
func (fs *Files) doCacheSweep() {
	ctx, cancelFn := context.WithTimeout(context.Background(), time.Millisecond*500)
	defer cancelFn()

	log := fs.log.With(lga.Dir, fs.cacheDir)

	if unlock, err := fs.cfgLockFn(ctx); err != nil {
		log.Warn("Sweep cache dir: failed to acquire config lock", lga.Lock, fs.cfgLockFn, lga.Err, err)
		return
	} else {
		defer unlock()
	}

	count, err := ioz.PruneEmptyDirTree(ctx, fs.cacheDir)
	if err != nil {
		log.Warn("Problem sweeping cache dir", lga.Err, err, "deleted_dirs", count)
		return
	}

	log.Info("Swept cache dir", "deleted_dirs", count)
}

// DefaultCacheDir returns the sq cache dir. This is generally
// in USER_CACHE_DIR/*/sq, but could also be in TEMP_DIR/*/sq/cache
// or similar. It is not guaranteed that the returned dir exists
// or is accessible.
func DefaultCacheDir() (dir string) {
	var err error
	if dir, err = os.UserCacheDir(); err != nil {
		// Some systems may not have a user cache dir, so we fall back
		// to the system temp dir.
		dir = filepath.Join(DefaultTempDir(), "cache")
		return dir
	}

	dir = filepath.Join(dir, "sq")
	return dir
}

// DefaultTempDir returns the default sq temp dir. It is not
// guaranteed that the returned dir exists or is accessible.
func DefaultTempDir() (dir string) {
	return filepath.Join(os.TempDir(), "sq")
}

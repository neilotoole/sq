package source

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// OptCacheLockTimeout is the time allowed to acquire cache lock.
//
// See also: [driver.OptIngestCache].
var OptCacheLockTimeout = options.NewDuration(
	"cache.lock.timeout",
	"",
	0,
	time.Second*5,
	"Wait timeout to acquire cache lock",
	`Wait timeout to acquire cache lock. During this period, retry will occur
if the lock is already held by another process. If zero, no retry occurs.`,
	options.TagSource,
	options.TagSQL,
)

// CacheDirFor gets the cache dir for handle. It is not guaranteed
// that the returned dir exists or is accessible.
func (fs *Files) CacheDirFor(src *Source) (dir string, err error) {
	handle := src.Handle
	if err = ValidHandle(handle); err != nil {
		return "", errz.Wrapf(err, "cache dir: invalid handle: %s", handle)
	}

	if handle == StdinHandle {
		// stdin is different input every time, so we need a unique
		// cache dir. In practice, stdin probably isn't using this function.
		handle += "_" + stringz.UniqN(32)
	}

	dir = filepath.Join(
		fs.cacheDir,
		"sources",
		filepath.Join(strings.Split(strings.TrimPrefix(handle, "@"), "/")...),
		fs.sourceHash(src),
	)

	return dir, nil
}

// sourceHash generates a hash for src. The hash is based on the
// member fields of src, with special handling for src.Options.
// Only the opts that affect data ingestion (options.TagIngestMutate)
// are incorporated in the hash.
func (fs *Files) sourceHash(src *Source) string {
	if src == nil {
		return ""
	}

	buf := bytes.Buffer{}
	buf.WriteString(src.Handle)
	buf.WriteString(string(src.Type))
	buf.WriteString(src.Location)
	buf.WriteString(src.Catalog)
	buf.WriteString(src.Schema)

	// FIXME: Revisit this
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
			buf.WriteString(fmt.Sprintf("%v", v))
			mUsedKeys[k] = v
		}
	}

	sum := checksum.Sum(buf.Bytes())
	return sum
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

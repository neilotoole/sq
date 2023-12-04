package source

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
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
		src.Hash(),
	)

	return dir, nil
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

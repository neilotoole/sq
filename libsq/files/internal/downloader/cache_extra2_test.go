// This file contains additional white-box tests for cache.go, targeting the
// cachePromote error/cleanup branches, the checksumsMatch "no cached checksum"
// branch, and the clear() error branch. These are reached by constructing
// responseCacher and cache literals directly (package downloader), since the
// public Downloader API only exercises them on the happy path.

package downloader

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
)

// newStagingCacher builds a responseCacher backed by a real staging dir with a
// "body" file, plus a main dir path, for white-box promotion tests. The caller
// supplies the wrapped body.
func newStagingCacher(t *testing.T, body io.ReadCloser) (rc *responseCacher, stagingDir string) {
	t.Helper()
	dir := t.TempDir()
	stagingDir = filepath.Join(dir, "staging")
	require.NoError(t, ioz.RequireDir(stagingDir))

	f, err := os.Create(filepath.Join(stagingDir, "body"))
	require.NoError(t, err)

	rc = &responseCacher{
		log:        lgt.New(t),
		stagingDir: stagingDir,
		mainDir:    filepath.Join(dir, "main"),
		body:       body,
		f:          f,
	}
	return rc, stagingDir
}

// errCloser is an io.ReadCloser whose Read returns EOF and whose Close returns
// a configurable error, used to drive responseCacher.cachePromote's
// body-close-error branch.
type errCloser struct {
	closeErr error
}

func (c *errCloser) Read([]byte) (int, error) { return 0, io.EOF }

func (c *errCloser) Close() error { return c.closeErr }

// TestResponseCacher_cachePromote_statError covers the r.f.Stat() error branch
// (cache.go 622-624) and the deferred staging cleanup (615-618): the staging
// body file handle is closed before cachePromote runs, so Stat fails.
func TestResponseCacher_cachePromote_statError(t *testing.T) {
	rc, stagingDir := newStagingCacher(t, io.NopCloser(bytes.NewBufferString("data")))

	// Close the staging file handle so r.f.Stat() fails inside cachePromote.
	require.NoError(t, rc.f.Close())

	err := rc.cachePromote()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to stat staging cache body file")

	// Deferred cleanup removed the staging dir and cleared the field.
	require.Empty(t, rc.stagingDir)
	_, statErr := os.Stat(stagingDir)
	require.True(t, errors.Is(statErr, os.ErrNotExist))
}

// TestResponseCacher_cachePromote_bodyCloseError covers the r.body.Close()
// error branch (cache.go 635-637): Stat and f.Close succeed, but the wrapped
// body's Close returns an error.
func TestResponseCacher_cachePromote_bodyCloseError(t *testing.T) {
	wantErr := errors.New("body close boom")
	rc, stagingDir := newStagingCacher(t, &errCloser{closeErr: wantErr})

	err := rc.cachePromote()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to close http response body")

	// f was closed and cleared; staging dir removed by deferred cleanup.
	require.Nil(t, rc.f)
	require.Empty(t, rc.stagingDir)
	_, statErr := os.Stat(stagingDir)
	require.True(t, errors.Is(statErr, os.ErrNotExist))
}

// TestResponseCacher_cachePromote_renameError covers the ioz.RenameDir error
// branch (cache.go 649-651): all earlier steps succeed (checksum written), but
// the main dir's parent is read-only, so the staging->main rename fails. The
// deferred cleanup then removes the staging dir.
//
// Skipped on Windows (chmod semantics differ) and as root (bypasses perms).
func TestResponseCacher_cachePromote_renameError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission test not reliable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses filesystem permission checks")
	}

	dir := t.TempDir()
	stagingDir := filepath.Join(dir, "staging")
	require.NoError(t, ioz.RequireDir(stagingDir))

	fpBody := filepath.Join(stagingDir, "body")
	f, err := os.Create(fpBody)
	require.NoError(t, err)
	_, err = f.WriteString("payload")
	require.NoError(t, err)

	// mainDir lives under a read-only parent so RenameDir (an os.Rename into
	// that parent) fails.
	roParent := filepath.Join(dir, "ro")
	require.NoError(t, os.MkdirAll(roParent, 0o755))
	mainDir := filepath.Join(roParent, "main")
	require.NoError(t, os.Chmod(roParent, 0o500))
	t.Cleanup(func() { _ = os.Chmod(roParent, 0o755) })

	rc := &responseCacher{
		log:        lgt.New(t),
		stagingDir: stagingDir,
		mainDir:    mainDir,
		body:       io.NopCloser(bytes.NewBufferString("payload")),
		f:          f,
	}

	err = rc.cachePromote()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to write download cache")

	// Deferred cleanup removed the staging dir.
	require.Empty(t, rc.stagingDir)
	_, statErr := os.Stat(stagingDir)
	require.True(t, errors.Is(statErr, os.ErrNotExist))
}

// TestCache_checksumsMatch_noCachedChecksum covers the cachedChecksum-miss
// branch of checksumsMatch (cache.go 304-306): a body file exists but there's
// no checksums.txt, so cachedChecksum returns ("", false).
func TestCache_checksumsMatch_noCachedChecksum(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	mainDir := filepath.Join(c.dir, "main")
	require.NoError(t, ioz.RequireDir(mainDir))
	// Body present, but no checksums.txt.
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "body"), []byte("body"), 0o600))

	req := newCacheTestReq(t, ctx)
	// Discard the sum (always empty on failure) so the source's //nolint:unparam
	// on checksumsMatch stays meaningful, matching TestCache_checksumsMatch_missingBody.
	_, ok := c.checksumsMatch(req)
	require.False(t, ok)
}

// TestCache_clear_removeError covers the error branch of clear() (cache.go
// 341-344): the cache dir's parent is read-only, so os.RemoveAll on the cache
// dir fails. Skipped on Windows and as root.
func TestCache_clear_removeError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission test not reliable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses filesystem permission checks")
	}

	ctx := lg.NewContext(context.Background(), lgt.New(t))
	parent := t.TempDir()
	cacheDir := filepath.Join(parent, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	// Put a file inside so RemoveAll must unlink it (a populated dir under a
	// read-only parent can't be removed).
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "f"), []byte("x"), 0o600))
	require.NoError(t, os.Chmod(parent, 0o500))
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	c := &cache{dir: cacheDir}
	err := c.clear(ctx)
	require.Error(t, err)
}

package source

import (
	"bytes"
	"context"
	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/stretchr/testify/require"
	"net/url"
	"path"
	"path/filepath"
	"testing"
)

func TestDownloader2_Download(t *testing.T) {
	log := slogt.New(t)
	ctx := lg.NewContext(context.Background(), log)
	const dlURL = urlActorCSV
	const wantContentLength = sizeActorCSV
	u, err := url.Parse(dlURL)
	require.NoError(t, err)
	wantFilename := path.Base(u.Path)
	require.Equal(t, "actor.csv", wantFilename)

	cacheDir, err := filepath.Abs(filepath.Join("testdata", "downloader", "cache-dir-2"))
	require.NoError(t, err)
	t.Logf("cacheDir: %s", cacheDir)

	log.Debug("huzzah")
	dl, err := newDownloader2(cacheDir, "sq/dev", dlURL)
	require.NoError(t, err)
	//require.NoError(t, dl.ClearCache(ctx))

	buf := &bytes.Buffer{}
	written, cachedFp, err := dl.Download2(ctx, buf)
	_ = written
	_ = cachedFp
	require.NoError(t, err)
	//require.Equal(t, wantContentLength, written)
	//require.Equal(t, wantContentLength, int64(buf.Len()))

	buf.Reset()
	written, cachedFp, err = dl.Download2(ctx, buf)
	require.NoError(t, err)
	//require.Equal(t, wantContentLength, written)
	//require.Equal(t, wantContentLength, int64(buf.Len()))
}

func TestDownloader2_Download_Legacy(t *testing.T) {
	ctx := lg.NewContext(context.Background(), slogt.New(t))
	const dlURL = urlActorCSV
	const wantContentLength = sizeActorCSV
	u, err := url.Parse(dlURL)
	require.NoError(t, err)
	wantFilename := path.Base(u.Path)
	require.Equal(t, "actor.csv", wantFilename)

	cacheDir, err := filepath.Abs(filepath.Join("testdata", "downloader", "cache-dir-2"))
	require.NoError(t, err)
	t.Logf("cacheDir: %s", cacheDir)

	dl, err := newDownloader2(cacheDir, "sq/dev", dlURL)
	require.NoError(t, err)
	//require.NoError(t, dl.ClearCache(ctx))

	buf := &bytes.Buffer{}
	written, cachedFp, err := dl.Download2(ctx, buf)
	_ = cachedFp
	require.NoError(t, err)
	require.Equal(t, wantContentLength, written)
	require.Equal(t, wantContentLength, int64(buf.Len()))

	buf.Reset()
	written, cachedFp, err = dl.Download(ctx, buf)
	require.NoError(t, err)
	require.Equal(t, wantContentLength, written)
	require.Equal(t, wantContentLength, int64(buf.Len()))
}

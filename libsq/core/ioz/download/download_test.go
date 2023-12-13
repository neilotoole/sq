package download_test

import (
	"bytes"
	"context"
	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/download"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const (
	urlPaymentLargeCSV = "https://sqio-public.s3.amazonaws.com/testdata/payment-large.gen.csv"
	urlActorCSV        = "https://sq.io/testdata/actor.csv"
	sizeActorCSV       = int64(7641)
	sizeGzipActorCSV   = int64(1968)
)

func TestDownload(t *testing.T) {
	log := slogt.New(t)
	ctx := lg.NewContext(context.Background(), log)
	const dlURL = urlActorCSV

	// FIXME: switch to temp dir
	cacheDir, err := filepath.Abs(filepath.Join("testdata", "downloader", "cache-dir-2"))
	require.NoError(t, err)
	t.Logf("cacheDir: %s", cacheDir)

	dl := download.New(cacheDir, download.OptUserAgent("sq/dev"))
	require.NoError(t, dl.Clear(ctx))

	var (
		destBuf = &bytes.Buffer{}
		gotFp   string
		gotErr  error
	)
	reset := func() {
		destBuf.Reset()
		gotFp = ""
		gotErr = nil
	}

	h := download.Handler{
		Cached: func(cachedFilepath string) {
			gotFp = cachedFilepath
		},
		Uncached: func() (wc io.WriteCloser, errFn func(error)) {
			return ioz.WriteCloser(destBuf),
				func(err error) {
					gotErr = err
				}
		},
		Error: func(err error) {
			gotErr = err
		},
	}

	require.Equal(t, download.Uncached, dl.State(ctx, dlURL))
	dl.Get(ctx, dlURL, h)
	require.NoError(t, gotErr)
	require.Empty(t, gotFp)
	require.Equal(t, sizeActorCSV, int64(destBuf.Len()))

	require.Equal(t, download.Fresh, dl.State(ctx, dlURL))

	reset()
	dl.Get(ctx, dlURL, h)
	require.NoError(t, gotErr)
	require.Equal(t, 0, destBuf.Len())
	require.NotEmpty(t, gotFp)
	gotFileBytes, err := os.ReadFile(gotFp)
	require.NoError(t, err)
	require.Equal(t, sizeActorCSV, int64(len(gotFileBytes)))

	require.Equal(t, download.Fresh, dl.State(ctx, dlURL))

	require.NoError(t, dl.Clear(ctx))
	require.Equal(t, download.Uncached, dl.State(ctx, dlURL))
}

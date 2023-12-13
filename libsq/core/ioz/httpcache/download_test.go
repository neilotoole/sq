package httpcache_test

import (
	"bytes"
	"context"
	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/httpcache"
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

func TestTransport_Fetch(t *testing.T) {
	log := slogt.New(t)
	ctx := lg.NewContext(context.Background(), log)
	const dlURL = urlActorCSV

	cacheDir, err := filepath.Abs(filepath.Join("testdata", "downloader", "cache-dir-2"))
	require.NoError(t, err)
	t.Logf("cacheDir: %s", cacheDir)

	dl := httpcache.NewTransport(cacheDir, httpcache.OptUserAgent("sq/dev"))
	require.NoError(t, dl.Delete(ctx))

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

	h := httpcache.Handler{
		Cached: func(cachedFilepath string) error {
			gotFp = cachedFilepath
			return nil
		},
		Uncached: func() (wc io.WriteCloser, errFn func(error), err error) {
			return ioz.WriteCloser(destBuf),
				func(err error) {
					gotErr = err
				},
				nil
		},
		Error: func(err error) {
			gotErr = err
		},
	}

	//req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	////if d.userAgent != "" {
	////	req.Header.Set("User-Agent", d.userAgent)
	////}
	dl.Fetch(ctx, dlURL, h)
	require.NoError(t, gotErr)
	require.Empty(t, gotFp)
	require.Equal(t, sizeActorCSV, int64(destBuf.Len()))

	reset()
	dl.Fetch(ctx, dlURL, h)
	require.NoError(t, gotErr)
	require.Equal(t, 0, destBuf.Len())
	require.NotEmpty(t, gotFp)
	gotFileBytes, err := os.ReadFile(gotFp)
	require.NoError(t, err)
	require.Equal(t, sizeActorCSV, int64(len(gotFileBytes)))

}

package source

import (
	"bytes"
	"context"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/tu"
)

func TestGetRemoteChecksum(t *testing.T) {
	// sq add https://sq.io/testdata/actor.csv
	//
	// content-length: 7641
	// date: Thu, 07 Dec 2023 06:31:10 GMT
	// etag: "069dbf690a12d5eb853feb8e04aeb49e-ssl"

	// TODO
}

const (
	urlPaymentLargeCSV = "https://sqio-public.s3.amazonaws.com/testdata/payment-large.gen.csv"
	urlActorCSV        = "https://sq.io/testdata/actor.csv"
	sizeActorCSV       = int64(7641)
	sizeGzipActorCSV   = int64(1968)
)

func TestFetchHTTPHeader_sqio(t *testing.T) {
	header, err := fetchHTTPResponse(context.Background(), urlActorCSV)
	require.NoError(t, err)
	require.NotNil(t, header)

	// TODO
}

func TestDownloader_Download(t *testing.T) {
	ctx := lg.NewContext(context.Background(), slogt.New(t))
	const dlURL = urlActorCSV
	const wantContentLength = sizeActorCSV
	u, err := url.Parse(dlURL)
	require.NoError(t, err)
	wantFilename := path.Base(u.Path)
	require.Equal(t, "actor.csv", wantFilename)

	cacheDir, err := filepath.Abs(filepath.Join("testdata", "downloader", "cache-dir-1"))
	require.NoError(t, err)
	t.Logf("cacheDir: %s", cacheDir)

	dl := newDownloader(http.DefaultClient, cacheDir, dlURL)
	require.NoError(t, dl.ClearCache(ctx))

	buf := &bytes.Buffer{}
	written, cachedFp, err := dl.Download(ctx, buf)
	require.NoError(t, err)
	require.FileExists(t, cachedFp)
	require.Equal(t, wantContentLength, written)
	require.Equal(t, wantContentLength, int64(buf.Len()))

	s := tu.ReadFileToString(t, dl.headerFile())
	t.Logf("header.txt\n\n" + s + "\n")

	s = tu.ReadFileToString(t, dl.checksumFile())
	t.Logf("checksum.txt\n\n" + s + "\n")

	gotSums, err := checksum.ReadFile(dl.checksumFile())
	require.NoError(t, err)

	isCached, cachedSum, cachedFp := dl.Cached(ctx)
	require.True(t, isCached)
	wantKey := filepath.Join("dl", wantFilename)
	wantFp, err := filepath.Abs(filepath.Join(dl.cacheDir, wantKey))
	require.NoError(t, err)
	require.Equal(t, wantFp, cachedFp)
	fileSum, ok := gotSums[wantKey]
	require.True(t, ok)
	require.Equal(t, cachedSum, fileSum)

	isCurrent, err := dl.CachedIsCurrent(ctx)
	require.NoError(t, err)
	require.True(t, isCurrent)
}

func TestFetchHTTPHeader_HEAD_fallback_GET(t *testing.T) {
	b := proj.ReadFile("drivers/csv/testdata/sakila-csv/actor.csv")
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Length", strconv.Itoa(len(b)))
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(b)
		require.NoError(t, err)
	}))
	t.Cleanup(srvr.Close)

	u := srvr.URL

	resp, err := fetchHTTPResponse(context.Background(), u)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, len(b), int(resp.ContentLength))
}

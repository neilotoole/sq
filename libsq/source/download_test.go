package source

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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
)

func TestFetchHTTPHeader_sqio(t *testing.T) {
	header, err := fetchHTTPHeader(context.Background(), urlActorCSV)
	require.NoError(t, err)
	require.NotNil(t, header)

	// TODO
}

func TestDownloader_Download(t *testing.T) {
	ctx := lg.NewContext(context.Background(), slogt.New(t))

	cacheDir, err := filepath.Abs(filepath.Join("testdata", "downloader", "cache-dir-1"))
	require.NoError(t, err)
	t.Logf("cacheDir: %s", cacheDir)
	dl := newDownloader(http.DefaultClient, cacheDir, urlActorCSV)
	buf := &bytes.Buffer{}
	written, fp, err := dl.Download(ctx, buf)
	require.NoError(t, err)
	require.FileExists(t, fp)
	require.Equal(t, sizeActorCSV, written)
	require.Equal(t, sizeActorCSV, int64(buf.Len()))

	s := tu.ReadFileToString(t, dl.headerFile())
	t.Logf("header.txt\n\n" + s)

	s = tu.ReadFileToString(t, dl.checksumFile())
	t.Logf("checksum.txt\n\n" + s)

	// TODO
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

	header, err := fetchHTTPHeader(context.Background(), u)
	assert.NoError(t, err)
	assert.NotNil(t, header)
}

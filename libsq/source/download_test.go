package source

import (
	"context"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestGetRemoteChecksum(t *testing.T) {
	// sq add https://sq.io/testdata/actor.csv
	//
	// content-length: 7641
	// date: Thu, 07 Dec 2023 06:31:10 GMT
	// etag: "069dbf690a12d5eb853feb8e04aeb49e-ssl"

	// TODO
}

func TestFetchHTTPHeader_sqio(t *testing.T) {
	u := "https://sq.io/testdata/actor.csv"

	header, err := fetchHTTPHeader(context.Background(), u)
	require.NoError(t, err)
	require.NotNil(t, header)

	// TODO
}

func TestFetchHTTPHeader_HEAD_fallback_GET(t *testing.T) {
	b := proj.ReadFile("drivers/csv/testdata/sakila-csv/actor.csv")
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set(http.CanonicalHeaderKey("Content-Length"), strconv.Itoa(len(b)))
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(b)
		require.NoError(t, err)

	}))
	t.Cleanup(srvr.Close)

	u := srvr.URL

	header, err := fetchHTTPHeader(context.Background(), u)
	assert.NoError(t, err)
	assert.NotNil(t, header)

	//u := "https://sq.io/testdata/actor.csv"
	//
	//header, allowed, err := fetchHTTPHeader(context.Background(), u)
	//require.NoError(t, err)
	//require.True(t, allowed)
	//require.NotNil(t, header)
	//
	//// TODO
}

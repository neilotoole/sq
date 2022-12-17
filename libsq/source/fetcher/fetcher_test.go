package fetcher_test

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/fetcher"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestFetcherHTTP(t *testing.T) {
	wantData := proj.ReadFile(sakila.PathCSVActor)
	buf := &bytes.Buffer{}

	f := &fetcher.Fetcher{}
	err := f.Fetch(context.Background(), sakila.URLActorCSV, buf)
	require.NoError(t, err)

	require.Equal(t, wantData, buf.Bytes())
}

func TestFetcherConfig(t *testing.T) {
	ctx := context.Background()
	serverSleepy := new(time.Duration)

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(*serverSleepy)
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0) // hush the server logging
	server.StartTLS()
	defer server.Close()

	fetchr := &fetcher.Fetcher{}
	// No config, expect error because of bad cert
	err := fetchr.Fetch(ctx, server.URL, io.Discard)
	require.Error(t, err, "expect untrusted cert error")

	cfg := &fetcher.Config{InsecureSkipVerify: true}

	// Config as field of Fetcher
	fetchr = &fetcher.Fetcher{Config: cfg}
	err = fetchr.Fetch(ctx, server.URL, io.Discard)
	require.NoError(t, err)

	// Test timeout
	cfg.Timeout = time.Millisecond * 100

	// Have the server sleep for longer than that
	*serverSleepy = time.Millisecond * 200
	fetchr = &fetcher.Fetcher{Config: cfg}
	err = fetchr.Fetch(ctx, server.URL, io.Discard)
	require.Error(t, err, "should have seen a client timeout")

	// Make the client timeout larger than server sleep time
	cfg.Timeout = time.Millisecond * 500
	err = fetchr.Fetch(ctx, server.URL, io.Discard)
	require.NoError(t, err)
}

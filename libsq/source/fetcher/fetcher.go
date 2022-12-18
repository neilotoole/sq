// Package fetcher provides a mechanism for fetching files
// from URLs.
package fetcher

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/context/ctxhttp"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// Config parameterizes Fetcher behavior.
type Config struct {
	// Timeout is the request timeout.
	Timeout time.Duration

	// Skip verification of insecure transports.
	InsecureSkipVerify bool
}

// Fetcher can fetch files from URLs. If field Config is nil,
// defaults are used. At this time, only HTTP/HTTPS is supported,
// but it's possible other schemes (such as FTP) will be
// supported in future.
type Fetcher struct {
	Config *Config
}

// Fetch writes the body of the document at fileURL to w.
func (f *Fetcher) Fetch(ctx context.Context, fileURL string, w io.Writer) error {
	return fetchHTTP(ctx, f.Config, fileURL, w)
}

func httpClient(cfg *Config) *http.Client {
	var client = *http.DefaultClient

	var tr *http.Transport
	if client.Transport == nil {
		tr = (http.DefaultTransport.(*http.Transport)).Clone()
	} else {
		tr = (client.Transport.(*http.Transport)).Clone()
	}

	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		tr.TLSClientConfig = tr.TLSClientConfig.Clone()
	}

	if cfg != nil {
		tr.TLSClientConfig.InsecureSkipVerify = cfg.InsecureSkipVerify
		client.Timeout = cfg.Timeout
	}

	client.Transport = tr

	return &client
}

func fetchHTTP(ctx context.Context, cfg *Config, fileURL string, w io.Writer) error {
	c := httpClient(cfg)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}

	resp, err := ctxhttp.Do(ctx, c, req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return errz.Errorf("http: returned non-200 status code (%s) from: %s", resp.Status, fileURL)
	}

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		_ = resp.Body.Close()
		return errz.Wrapf(err, "http: failed to read body from: %s", fileURL)
	}

	return errz.Err(resp.Body.Close())
}

// Schemes is the set of supported schemes.
func (f *Fetcher) Schemes() []string {
	return []string{"http", "https"}
}

// Supported returns true if loc is a supported URL.
func (f *Fetcher) Supported(loc string) bool {
	u, err := url.ParseRequestURI(loc)
	if err != nil {
		return false
	}

	if stringz.InSlice(f.Schemes(), u.Scheme) {
		return true
	}

	return false
}

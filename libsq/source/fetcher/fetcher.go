// Package fetcher provides a mechanism for fetching files
// from URLs.
//
// At this time package fetcher also contains the legacy FetchFile
// method and associated code, while is deprecated and will be removed.
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
// defaults are used.
type Fetcher struct {
	Config *Config
}

// Fetch writes the body of the document at url to w.
// If cfg is nil, f's config is used.
func (f *Fetcher) Fetch(ctx context.Context, cfg *Config, url string, w io.Writer) error {
	if cfg == nil {
		cfg = f.Config
	}

	return fetchHTTP(ctx, cfg, url, w)
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
		tr.TLSClientConfig = &tls.Config{}
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

func fetchHTTP(ctx context.Context, cfg *Config, url string, w io.Writer) error {
	c := httpClient(cfg)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := ctxhttp.Do(ctx, c, req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return errz.Errorf("http: returned non-200 status code (%s) from: %s", resp.Status, url)
	}

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		_ = resp.Body.Close()
		return errz.Wrapf(err, "http: failed to read body from: %s", url)
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

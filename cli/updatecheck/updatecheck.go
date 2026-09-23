// Package updatecheck fetches and caches the latest sq release version.
package updatecheck

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

const (
	cacheFileName   = "update-check.json"
	defaultCacheTTL = 24 * time.Hour
	fetchTimeout    = 500 * time.Millisecond
	brewFormulaURL  = `https://raw.githubusercontent.com/Homebrew/homebrew-core/HEAD/Formula/s/sq.rb`
)

// Cache holds a cached latest-version lookup.
type Cache struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

// CacheDirForRun returns the cache dir for ru after preRun has initialized Files.
func CacheDirForRun(ru *run.Run) string {
	if ru == nil || ru.Files == nil {
		return ""
	}
	return ru.Files.CacheDir()
}

// FetchLatestWithWait fetches the latest version, waiting up to timeout.
// Used by sq version where a fresh result is preferred over cache-only.
func FetchLatestWithWait(ctx context.Context, cacheDir string, timeout time.Duration) (string, error) {
	return fetchLatestWithWait(ctx, cacheDir, timeout, fetchBrewVersion)
}

func fetchLatestWithWait(
	ctx context.Context, cacheDir string, timeout time.Duration, fetch func(context.Context) (string, error),
) (string, error) {
	if timeout <= 0 {
		timeout = fetchTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type fetchResult struct {
		err error
		raw string
	}

	resultCh := make(chan fetchResult, 1)
	go func() {
		raw, err := fetch(ctx)
		resultCh <- fetchResult{raw: raw, err: err}
	}()

	select {
	case <-ctx.Done():
		if latest, ok := readFreshCache(cacheDir); ok {
			return latest, nil
		}
		return "", nil
	case result := <-resultCh:
		if result.err != nil {
			lg.WarnIfError(lg.FromContext(ctx), "Get brew version", result.err)
			if latest, ok := readFreshCache(cacheDir); ok {
				return latest, nil
			}
			return "", nil
		}
		raw := result.raw

		latest, err := NormalizeVersion(raw)
		if err != nil {
			if cached, ok := readFreshCache(cacheDir); ok {
				return cached, nil
			}
			return "", err
		}
		if latest != "" && cacheDir != "" {
			_ = writeCache(cacheDir, Cache{LatestVersion: latest, CheckedAt: time.Now().UTC()})
		}
		return latest, nil
	}
}

// NormalizeVersion returns a semver-valid version with a "v" prefix.
func NormalizeVersion(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if !strings.HasPrefix(raw, "v") {
		raw = "v" + raw
	}
	if !semver.IsValid(raw) {
		return "", errz.Errorf("invalid semver from brew formula: {%s}", raw)
	}
	return raw, nil
}

func cachePath(cacheDir string) string {
	return filepath.Join(cacheDir, cacheFileName)
}

func readCache(cacheDir string) (Cache, bool) {
	if cacheDir == "" {
		return Cache{}, false
	}

	b, err := os.ReadFile(cachePath(cacheDir))
	if err != nil {
		return Cache{}, false
	}

	var c Cache
	if err := json.Unmarshal(b, &c); err != nil {
		return Cache{}, false
	}
	return c, true
}

func readFreshCache(cacheDir string) (string, bool) {
	c, ok := readCache(cacheDir)
	if !ok || c.LatestVersion == "" {
		return "", false
	}
	if time.Since(c.CheckedAt) >= defaultCacheTTL {
		return "", false
	}
	return c.LatestVersion, true
}

func writeCache(cacheDir string, c Cache) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return errz.Err(err)
	}

	b, err := json.Marshal(c)
	if err != nil {
		return errz.Err(err)
	}

	f, err := os.CreateTemp(cacheDir, cacheFileName+".*.tmp")
	if err != nil {
		return errz.Err(err)
	}
	tmp := f.Name()

	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return errz.Err(err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return errz.Err(err)
	}
	if err := os.Rename(tmp, cachePath(cacheDir)); err != nil {
		_ = os.Remove(tmp)
		return errz.Err(err)
	}
	return nil
}

func fetchBrewVersion(ctx context.Context) (string, error) {
	lg.FromContext(ctx).Debug("Fetching brew formula for version check", lga.URL, brewFormulaURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, brewFormulaURL, http.NoBody)
	if err != nil {
		return "", errz.Err(err)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose
	if err != nil {
		return "", errz.Wrap(err, "failed to check sq brew formula")
	}
	defer ioz.Close(ctx, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", errz.Errorf("failed to check sq brew formula: %d %s",
			resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errz.Wrap(err, "failed to read sq brew formula body")
	}

	return versionFromBrewFormula(body)
}

func versionFromBrewFormula(f []byte) (string, error) {
	var (
		val        string
		urlVersion string
	)

	sc := bufio.NewScanner(bytes.NewReader(f))
	for sc.Scan() {
		val = strings.TrimSpace(sc.Text())

		if strings.HasPrefix(val, `version "`) {
			val = val[9:]
			val = strings.TrimSuffix(val, `"`)
			if !semver.IsValid("v" + val) {
				return "", errz.Errorf("invalid brew formula: invalid semver {%s}", val)
			}
			return val, nil
		}

		if strings.HasPrefix(val, `url "`) && strings.Contains(val, "/tags/v") {
			idx := strings.Index(val, "/tags/v")
			if idx != -1 {
				verStart := idx + len("/tags/v")
				remainder := val[verStart:]
				if before, _, ok := strings.Cut(remainder, ".tar.gz"); ok {
					val = before
				} else if before, _, ok := strings.Cut(remainder, ".zip"); ok {
					val = before
				} else {
					continue
				}
				if !semver.IsValid("v" + val) {
					return "", errz.Errorf("invalid brew formula: invalid semver in URL {%s}", val)
				}
				urlVersion = val
			}
		}

		if strings.HasPrefix(val, "bottle") {
			break
		}
	}

	if sc.Err() != nil {
		return "", errz.Wrap(sc.Err(), "invalid brew formula")
	}

	if urlVersion != "" {
		return urlVersion, nil
	}

	return "", errz.New("invalid brew formula")
}

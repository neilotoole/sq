// Package fixtsrv serves the published Sakila fixtures over 127.0.0.1, so
// tests that need a real HTTP fetch do not depend on a live host being
// reachable. Before gh #1158 these fetches went to raw.githubusercontent.com,
// and a single slow response failed the CI job.
//
// The server starts on first use and is never closed: it lives for the life of
// the test binary, the same way testh/proj derives and holds SQ_ROOT.
package fixtsrv

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"

	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/pubfixt"
)

// EnvFixtureURL is the name of the envar holding the fixture server's base
// URL. testh/testdata/test.sq.yml resolves ${env:SQ_TEST_FIXTURE_URL} from it.
const EnvFixtureURL = "SQ_TEST_FIXTURE_URL"

// cacheControl is sent with every fixture response. It is required, not
// cosmetic. The downloader's getFreshness only assigns a non-zero lifetime
// from a max-age directive or an Expires header, so a bare http.FileServer
// (which sends neither) makes every cached entry Stale, breaking the Fresh
// assertions in TestDownloader. raw.githubusercontent.com sends max-age=300,
// which is what this mirrors.
const cacheControl = "public, max-age=300"

// start launches the server once per process and publishes its URL in
// EnvFixtureURL. os.Setenv, not testing.T.Setenv: the latter panics when
// called from a parallel test, and TestSmoke and TestDriver_Open are parallel.
var start = sync.OnceValue(func() string {
	srvr := httptest.NewServer(handler(Dir()))
	if err := os.Setenv(EnvFixtureURL, srvr.URL); err != nil {
		panic(err)
	}
	return srvr.URL
})

// Dir returns the absolute path of the fixture dir being served.
func Dir() string {
	return proj.Abs(pubfixt.PublishedDir)
}

// BaseURL returns the base URL of the fixture server, starting it on the
// first call and setting the EnvFixtureURL envar.
func BaseURL() string {
	return start()
}

// URL returns the fixture server URL for the named file, e.g. "actor.csv".
func URL(name string) string {
	return BaseURL() + "/" + name
}

// Size returns the on-disk size of the named fixture. It panics if the file
// cannot be stat'd, matching the proj.ReadFile idiom in this package tree.
//
// Tests use this instead of a hardcoded byte count so that regenerating a
// fixture needs no corresponding edit in Go.
func Size(name string) int {
	fi, err := os.Stat(filepath.Join(Dir(), name))
	if err != nil {
		panic(err)
	}
	return int(fi.Size())
}

// handler serves dir, adding the Cache-Control header described above.
func handler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", cacheControl)
		fs.ServeHTTP(w, r)
	})
}

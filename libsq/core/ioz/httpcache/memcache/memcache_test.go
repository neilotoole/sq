//go:build !appengine

package memcache

import (
	"net"
	"testing"

	"github.com/neilotoole/sq/libsq/core/ioz/httpcache/test"
)

const testServer = "localhost:11211"

func TestMemCache(t *testing.T) {
	conn, err := net.Dial("tcp", testServer)
	if err != nil {
		// TODO: rather than skip the test, fall back to a faked memcached server
		t.Skipf("skipping test; no server running at %s", testServer)
	}
	_, _ = conn.Write([]byte("flush_all\r\n")) // flush memcache
	conn.Close()

	test.Cache(t, New(testServer))
}

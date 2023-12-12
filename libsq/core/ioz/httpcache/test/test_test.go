package test_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq/core/ioz/httpcache"
	"github.com/neilotoole/sq/libsq/core/ioz/httpcache/test"
)

func TestMemoryCache(t *testing.T) {
	test.Cache(t, httpcache.NewMemoryCache())
}

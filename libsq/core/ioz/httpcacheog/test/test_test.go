package test_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq/core/ioz/httpcacheog"
	"github.com/neilotoole/sq/libsq/core/ioz/httpcacheog/test"
)

func TestMemoryCache(t *testing.T) {
	test.Cache(t, httpcache.NewMemoryCache())
}

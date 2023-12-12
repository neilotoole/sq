package test

import (
	"bytes"
	"context"
	"testing"

	"github.com/neilotoole/sq/libsq/core/ioz/httpcacheog"
)

// Cache excercises a httpcache.Cache implementation.
func Cache(t *testing.T, cache httpcache.Cache) {
	key := "testKey"
	_, ok := cache.Get(context.Background(), key)
	if ok {
		t.Fatal("retrieved key before adding it")
	}

	val := []byte("some bytes")
	cache.Set(context.Background(), key, val)

	retVal, ok := cache.Get(context.Background(), key)
	if !ok {
		t.Fatal("could not retrieve an element we just added")
	}
	if !bytes.Equal(retVal, val) {
		t.Fatal("retrieved a different value than what we put in")
	}

	cache.Delete(context.Background(), key)

	_, ok = cache.Get(context.Background(), key)
	if ok {
		t.Fatal("deleted key still present")
	}
}

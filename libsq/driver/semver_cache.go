package driver

import "sync"

// SemverCache memoizes a DBSemver result. It caches only on success, so a
// failed (e.g. context-cancelled) first fetch does not poison later calls. The
// server version is immutable for a connection's lifetime, so a single
// successful fetch is reused. Embed it in a grip and route DBSemver through Get.
type SemverCache struct {
	val string
	mu  sync.Mutex
	ok  bool
}

// Get returns the memoized value, invoking fetch (and caching) on first success.
func (c *SemverCache) Get(fetch func() (string, error)) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ok {
		return c.val, nil
	}
	v, err := fetch()
	if err != nil {
		return "", err
	}
	c.val, c.ok = v, true
	return v, nil
}

// Prime seeds the cache with v, so that a later Get returns v without invoking
// fetch. It is for callers that already hold the server version, e.g. from the
// version select that doubles as the opening ping (see [OpeningPing]). An empty
// v (version undeterminable) is ignored, leaving the cache unprimed so that Get
// fetches as normal.
func (c *SemverCache) Prime(v string) {
	if v == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.val, c.ok = v, true
}

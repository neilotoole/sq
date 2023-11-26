package fscache

import (
	"time"
)

// Entry represents a cached item.
type Entry struct {
	name  string
	inUse bool
}

// InUse returns if this Cache entry is in use.
func (e *Entry) InUse() bool {
	return e.inUse
}

// Name returns the File.Name() of this entry.
func (e *Entry) Name() string {
	return e.name
}

// CacheAccessor implementors provide ways to observe and interact with
// the cached entries, mainly used for cache-eviction.
type CacheAccessor interface {
	FileSystemStater
	EnumerateEntries(enumerator func(key string, e Entry) bool)
	RemoveFile(key string)
}

// Haunter implementors are used to perform cache-eviction (Next is how long to wait
// until next evication, Haunt preforms the eviction).
type Haunter interface {
	Haunt(c CacheAccessor)
	Next() time.Duration
}

type reaperHaunterStrategy struct {
	reaper Reaper
}

type lruHaunterStrategy struct {
	haunter LRUHaunter
}

// NewLRUHaunterStrategy returns a simple scheduleHaunt which provides an implementation LRUHaunter strategy
func NewLRUHaunterStrategy(haunter LRUHaunter) Haunter {
	return &lruHaunterStrategy{
		haunter: haunter,
	}
}

func (h *lruHaunterStrategy) Haunt(c CacheAccessor) {
	for _, key := range h.haunter.Scrub(c) {
		c.RemoveFile(key)
	}

}

func (h *lruHaunterStrategy) Next() time.Duration {
	return h.haunter.Next()
}

// NewReaperHaunterStrategy returns a simple scheduleHaunt which provides an implementation Reaper strategy
func NewReaperHaunterStrategy(reaper Reaper) Haunter {
	return &reaperHaunterStrategy{
		reaper: reaper,
	}
}

func (h *reaperHaunterStrategy) Haunt(c CacheAccessor) {
	c.EnumerateEntries(func(key string, e Entry) bool {
		if e.InUse() {
			return true
		}

		fileInfo, err := c.Stat(e.Name())
		if err != nil {
			return true
		}

		if h.reaper.Reap(key, fileInfo.AccessTime(), fileInfo.ModTime()) {
			c.RemoveFile(key)
		}

		return true
	})
}

func (h *reaperHaunterStrategy) Next() time.Duration {
	return h.reaper.Next()
}

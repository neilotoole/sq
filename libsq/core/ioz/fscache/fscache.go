package fscache

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/djherbis/stream"
)

// Cache works like a concurrent-safe map for streams.
type Cache interface {
	// Get manages access to the streams in the cache.
	// If the key does not exist, w != nil and you can start writing to the stream.
	// If the key does exist, w == nil.
	// r will always be non-nil as long as err == nil and you must close r when you're done reading.
	// Get can be called concurrently, and writing and reading is concurrent safe.
	Get(key string) (ReadAtCloser, io.WriteCloser, error)

	// Remove deletes the stream from the cache, blocking until the underlying
	// file can be deleted (all active streams finish with it).
	// It is safe to call Remove concurrently with Get.
	Remove(key string) error

	// Exists checks if a key is in the cache.
	// It is safe to call Exists concurrently with Get.
	Exists(key string) bool

	// Clean will empty the cache and delete the cache folder.
	// Clean is not safe to call while streams are being read/written.
	Clean() error
}

// FSCache is a Cache which uses a Filesystem to read/write cached data.
type FSCache struct {
	mu      sync.RWMutex
	files   map[string]fileStream
	km      func(string) string
	fs      FileSystem
	haunter Haunter
}

// SetKeyMapper will use the given function to transform any given Cache key into the result of km(key).
// This means that internally, the cache will only track km(key), and forget the original key. The consequences
// of this are that Enumerate will return km(key) instead of key, and Filesystem will give km(key) to Create
// and expect Reload() to return km(key).
// The purpose of this function is so that the internally managed key can be converted to a string that is
// allowed as a filesystem path.
func (c *FSCache) SetKeyMapper(km func(string) string) *FSCache {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.km = km
	return c
}

func (c *FSCache) mapKey(key string) string {
	if c.km == nil {
		return key
	}
	return c.km(key)
}

// ReadAtCloser is an io.ReadCloser, and an io.ReaderAt. It supports both so that Range
// Requests are possible.
type ReadAtCloser interface {
	io.ReadCloser
	io.ReaderAt
}

type fileStream interface {
	next() (*CacheReader, error)
	InUse() bool
	io.WriteCloser
	remove() error
	Name() string
}

// New creates a new Cache using NewFs(dir, perms).
// expiry is the duration after which an un-accessed key will be removed from
// the cache, a zero value expiro means never expire.
func New(dir string, perms os.FileMode, expiry time.Duration) (*FSCache, error) {
	fs, err := NewFs(dir, perms)
	if err != nil {
		return nil, err
	}
	var grim Reaper
	if expiry > 0 {
		grim = &reaper{
			expiry: expiry,
			period: expiry,
		}
	}
	return NewCache(fs, grim)
}

// NewCache creates a new Cache based on FileSystem fs.
// fs.Files() are loaded using the name they were created with as a key.
// Reaper is used to determine when files expire, nil means never expire.
func NewCache(fs FileSystem, grim Reaper) (*FSCache, error) {
	if grim != nil {
		return NewCacheWithHaunter(fs, NewReaperHaunterStrategy(grim))
	}

	return NewCacheWithHaunter(fs, nil)
}

// NewCacheWithHaunter create a new Cache based on FileSystem fs.
// fs.Files() are loaded using the name they were created with as a key.
// Haunter is used to determine when files expire, nil means never expire.
func NewCacheWithHaunter(fs FileSystem, haunter Haunter) (*FSCache, error) {
	c := &FSCache{
		files:   make(map[string]fileStream),
		haunter: haunter,
		fs:      fs,
	}
	err := c.load()
	if err != nil {
		return nil, err
	}
	if haunter != nil {
		c.scheduleHaunt()
	}

	return c, nil
}

func (c *FSCache) scheduleHaunt() {
	c.haunt()
	time.AfterFunc(c.haunter.Next(), c.scheduleHaunt)
}

func (c *FSCache) haunt() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.haunter.Haunt(&accessor{c: c})
}

func (c *FSCache) load() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fs.Reload(func(key, name string) {
		c.files[key] = c.oldFile(name)
	})
}

// Exists returns true iff this key is in the Cache (may not be finished streaming).
func (c *FSCache) Exists(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.files[c.mapKey(key)]
	return ok
}

// Get obtains a ReadAtCloser for the given key, and may return a WriteCloser to write the original cache data
// if this is a cache-miss.
func (c *FSCache) Get(key string) (r ReadAtCloser, w io.WriteCloser, err error) {
	c.mu.RLock()
	key = c.mapKey(key)
	f, ok := c.files[key]
	if ok {
		r, err = f.next()
		c.mu.RUnlock()
		return r, nil, err
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	f, ok = c.files[key]
	if ok {
		r, err = f.next()
		return r, nil, err
	}

	f, err = c.newFile(key)
	if err != nil {
		return nil, nil, err
	}

	r, err = f.next()
	if err != nil {
		f.Close()
		c.fs.Remove(f.Name())
		return nil, nil, err
	}

	c.files[key] = f

	return r, f, err
}

// Remove removes the specified key from the cache.
func (c *FSCache) Remove(key string) error {
	c.mu.Lock()
	key = c.mapKey(key)
	f, ok := c.files[key]
	delete(c.files, key)
	c.mu.Unlock()

	if ok {
		return f.remove()
	}
	return nil
}

// Clean resets the cache removing all keys and data.
func (c *FSCache) Clean() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files = make(map[string]fileStream)
	return c.fs.RemoveAll()
}

type accessor struct {
	c *FSCache
}

func (a *accessor) Stat(name string) (FileInfo, error) {
	return a.c.fs.Stat(name)
}

func (a *accessor) EnumerateEntries(enumerator func(key string, e Entry) bool) {
	for k, f := range a.c.files {
		if !enumerator(k, Entry{name: f.Name(), inUse: f.InUse()}) {
			break
		}
	}
}

func (a *accessor) RemoveFile(key string) {
	key = a.c.mapKey(key)
	f, ok := a.c.files[key]
	delete(a.c.files, key)
	if ok {
		a.c.fs.Remove(f.Name())
	}
}

type cachedFile struct {
	handleCounter
	stream *stream.Stream
}

func (c *FSCache) newFile(name string) (fileStream, error) {
	s, err := stream.NewStream(name, c.fs)
	if err != nil {
		return nil, err
	}
	cf := &cachedFile{
		stream: s,
	}
	cf.inc()
	return cf, nil
}

func (c *FSCache) oldFile(name string) fileStream {
	return &reloadedFile{
		fs:   c.fs,
		name: name,
	}
}

type reloadedFile struct {
	handleCounter
	fs             FileSystem
	name           string
	io.WriteCloser // nop Write & Close methods. will never be called.
}

func (f *reloadedFile) Name() string { return f.name }

func (f *reloadedFile) remove() error {
	f.waitUntilFree()
	return f.fs.Remove(f.name)
}

func (f *reloadedFile) next() (*CacheReader, error) {
	r, err := f.fs.Open(f.name)
	if err == nil {
		f.inc()
	}
	return &CacheReader{
		ReadAtCloser: r,
		cnt:          &f.handleCounter,
	}, err
}

func (f *cachedFile) Name() string { return f.stream.Name() }

func (f *cachedFile) remove() error { return f.stream.Remove() }

func (f *cachedFile) next() (*CacheReader, error) {
	reader, err := f.stream.NextReader()
	if err != nil {
		return nil, err
	}
	f.inc()
	return &CacheReader{
		ReadAtCloser: reader,
		cnt:          &f.handleCounter,
	}, nil
}

func (f *cachedFile) Write(p []byte) (int, error) {
	return f.stream.Write(p)
}

func (f *cachedFile) Close() error {
	defer f.dec()
	return f.stream.Close()
}

// CacheReader is a ReadAtCloser for a Cache key that also tracks open readers.
type CacheReader struct {
	ReadAtCloser
	cnt *handleCounter
}

// Close frees the underlying ReadAtCloser and updates the open reader counter.
func (r *CacheReader) Close() error {
	defer r.cnt.dec()
	return r.ReadAtCloser.Close()
}

// Size returns the current size of the stream being read, the boolean it
// returns is true iff the stream is done being written (otherwise Size may change).
// An error is returned if the Size fails to be computed or is not supported
// by the underlying filesystem.
func (r *CacheReader) Size() (int64, bool, error) {
	switch v := r.ReadAtCloser.(type) {
	case *stream.Reader:
		size, done := v.Size()
		return size, done, nil

	case interface{ Stat() (os.FileInfo, error) }:
		fi, err := v.Stat()
		if err != nil {
			return 0, false, err
		}
		return fi.Size(), true, nil

	default:
		return 0, false, fmt.Errorf("reader does not support stat")
	}
}

type handleCounter struct {
	cnt int64
	grp sync.WaitGroup
}

func (h *handleCounter) inc() {
	h.grp.Add(1)
	atomic.AddInt64(&h.cnt, 1)
}

func (h *handleCounter) dec() {
	atomic.AddInt64(&h.cnt, -1)
	h.grp.Done()
}

func (h *handleCounter) InUse() bool {
	return atomic.LoadInt64(&h.cnt) > 0
}

func (h *handleCounter) waitUntilFree() {
	h.grp.Wait()
}

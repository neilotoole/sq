package fscache

import (
	"sort"
	"time"
)

type lruHaunterKV struct {
	Key   string
	Value Entry
}

// LRUHaunter is used to control when there are too many streams
// or the size of the streams is too big.
// It is called once right after loading, and then it is run
// again after every Next() period of time.
type LRUHaunter interface {
	// Returns the amount of time to wait before the next scheduled Reaping.
	Next() time.Duration

	// Given a CacheAccessor, return keys to reap list.
	Scrub(c CacheAccessor) []string
}

// NewLRUHaunter returns a simple haunter which runs every "period"
// and scrubs older files when the total file size is over maxSize or
// total item count is over maxItems.
// If maxItems or maxSize are 0, they won't be checked
func NewLRUHaunter(maxItems int, maxSize int64, period time.Duration) LRUHaunter {
	return &lruHaunter{
		period:   period,
		maxItems: maxItems,
		maxSize:  maxSize,
	}
}

type lruHaunter struct {
	period   time.Duration
	maxItems int
	maxSize  int64
}

func (j *lruHaunter) Next() time.Duration {
	return j.period
}

func (j *lruHaunter) Scrub(c CacheAccessor) (keysToReap []string) {
	var count int
	var size int64
	var okFiles []lruHaunterKV

	c.EnumerateEntries(func(key string, e Entry) bool {
		if e.InUse() {
			return true
		}

		fileInfo, err := c.Stat(e.Name())
		if err != nil {
			return true
		}

		count++
		size = size + fileInfo.Size()
		okFiles = append(okFiles, lruHaunterKV{
			Key:   key,
			Value: e,
		})

		return true
	})

	sort.Slice(okFiles, func(i, j int) bool {
		iFileInfo, err := c.Stat(okFiles[i].Value.Name())
		if err != nil {
			return false
		}

		iLastRead := iFileInfo.AccessTime()

		jFileInfo, err := c.Stat(okFiles[j].Value.Name())
		if err != nil {
			return false
		}

		jLastRead := jFileInfo.AccessTime()

		return iLastRead.Before(jLastRead)
	})

	collectKeysToReapFn := func() bool {
		var key *string
		var err error
		key, count, size, err = j.removeFirst(c, &okFiles, count, size)
		if err != nil {
			return false
		}
		if key != nil {
			keysToReap = append(keysToReap, *key)
		}

		return true
	}

	if j.maxItems > 0 {
		for count > j.maxItems {
			if !collectKeysToReapFn() {
				break
			}
		}
	}

	if j.maxSize > 0 {
		for size > j.maxSize {
			if !collectKeysToReapFn() {
				break
			}
		}
	}

	return keysToReap
}

func (j *lruHaunter) removeFirst(fsStater FileSystemStater, items *[]lruHaunterKV, count int, size int64) (*string, int, int64, error) {
	var f lruHaunterKV

	f, *items = (*items)[0], (*items)[1:]

	fileInfo, err := fsStater.Stat(f.Value.Name())
	if err != nil {
		return nil, count, size, err
	}

	count--
	size = size - fileInfo.Size()

	return &f.Key, count, size, nil
}

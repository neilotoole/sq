package upgrademu

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_UpgradableLock(t *testing.T) {
	t.Run("can't get upgradableReadLock while writeLock held", func(t *testing.T) {
		m := RWMutex{}
		m.Lock()
		i := atomic.Int32{}
		go func() {
			m.UpgradableRLock()
			defer m.UpgradableRUnlock()
			i.Add(1)
		}()
		time.Sleep(time.Millisecond * 10)
		assert.Equal(t, int32(0), i.Load())
		m.Unlock()
	})

	t.Run("can't get write lock while upgradableReadLock upgraded to write", func(t *testing.T) {
		m := RWMutex{}
		m.UpgradableRLock()

		m.UpgradeWLock()
		i := atomic.Int32{}
		go func() {
			m.Lock()
			defer m.Unlock()
			i.Add(1)
		}()
		time.Sleep(time.Millisecond * 10)
		assert.Equal(t, int32(0), i.Load())
		m.UpgradableRUnlock()
	})

	t.Run("upgradable read lock allows other readers before upgrade", func(t *testing.T) {
		m := RWMutex{}
		m.UpgradableRLock()

		waitGroup := sync.WaitGroup{}
		for range 10 {
			waitGroup.Go(func() {
				m.RLock()
				defer m.RLock()
			})
		}
		waitGroup.Wait()
		m.UpgradableRUnlock()
	})

	t.Run("upgradable read lock prevents other readers after upgrade", func(t *testing.T) {
		m := RWMutex{}
		m.UpgradableRLock()

		m.UpgradeWLock()
		i := atomic.Int32{}
		go func() {
			m.RLock()
			defer m.RUnlock()
			i.Add(1)
		}()
		time.Sleep(time.Millisecond * 10)
		assert.Equal(t, int32(0), i.Load())
		m.UpgradableRUnlock()
	})

	t.Run("get upgradable read-lock while there are other readers", func(t *testing.T) {
		m := RWMutex{}

		m.RLock()
		defer m.RUnlock()

		m.RLock()
		defer m.RUnlock()

		m.UpgradableRLock()
		defer m.UpgradableRUnlock()
	})

	// this is to avoid a failing upgrade under read-lock. Once an upgradeable-lock is acquired, it must be able to
	// upgrade to write-lock without causing deadlock
	t.Run("prevent getting a second upgradable-lock even after upgrade", func(t *testing.T) {
		m := RWMutex{}

		m.RLock()
		defer m.RUnlock()

		m.RLock()
		defer m.RUnlock()

		m.UpgradableRLock()
		i := atomic.Int32{}
		go func() {
			m.UpgradableRLock()
			defer m.UpgradableRUnlock()
			i.Add(1)
		}()
		time.Sleep(time.Millisecond * 10)
		assert.Equal(t, int32(0), i.Load())
		m.UpgradableRUnlock()
	})
}

// Based on source code copyright by The Go Authors.
//
// Copyright (c) 2009 The Go Authors. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//   * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//   * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//   * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package upgrademu

import (
	"sync"
	"sync/atomic"
	_ "unsafe"
)

// RWMutex is an enhanced version of the standard sync.RWMutex.
// It has the all methods sync.RWMutex with exact same semantics.
// It gives more methods to give upgradable-read feature.
//
// The new semantics for upgradable-read are as follows:
// Multiple goroutines can get read-lock together with a single upgradable-read-lock.
// Only one goroutine can have a write-lock and no read-lock/upgradable-read-lock can be acquired in this state.
// There can be only a single goroutine keeping the upgrade-read-lock.
// RWMutex is not reentrant.
//
// Usage of the RWMutex:
//
//	mutex.UpgradableRLock()
//	defer mutex.UpgradableRUnlock()
//	// read-lock acquired section. We can return here safely if an error occurs
//	mutex.UpgradeWLock()
//	// critical section with exclusive right access
type RWMutex struct {
	w           sync.Mutex   // held if there are pending writers
	writerSem   uint32       // semaphore for writers to wait for completing readers
	readerSem   uint32       // semaphore for readers to wait for completing writers
	readerCount atomic.Int32 // number of pending readers
	// number of departing readers. A negative number.
	// Number of readers left while under write lock(while write lock waiting for readers to leave)
	readerWait atomic.Int32
	// Keep track if an upgradeable read-lock is upgraded to write-lock or not. Always accessed under w locked
	upgraded           bool
	upgradableReadMode bool
}

//go:linkname semaphoreAcquire sync.runtime_Semacquire
func semaphoreAcquire(s *uint32)

//go:linkname semaphoreRelease sync.runtime_Semrelease
func semaphoreRelease(s *uint32, handoff bool, skipframes int)

const rwmutexMaxReaders = 1 << 30

// RLock is same as sync.RWMutex.RLock
func (rw *RWMutex) RLock() {
	if rw.readerCount.Add(1) < 0 {
		// A writer is pending, wait for it.
		semaphoreAcquire(&rw.readerSem)
	}
}

// TryRLock is same as sync.RWMutex
func (rw *RWMutex) TryRLock() bool {
	for {
		c := rw.readerCount.Load()
		if c < 0 {
			return false
		}
		if rw.readerCount.CompareAndSwap(c, c+1) {
			return true
		}
	}
}

// RUnlock is same as sync.RWMutex
func (rw *RWMutex) RUnlock() {
	if r := rw.readerCount.Add(-1); r < 0 {
		// Outlined slow-path to allow the fast-path to be inlined
		rw.rUnlockSlow(r)
	}
}

func (rw *RWMutex) rUnlockSlow(r int32) {
	if r+1 == 0 || r+1 == -rwmutexMaxReaders {
		panic("sync: RUnlock of unlocked RWMutex")
	}
	// A writer is pending.
	if rw.readerWait.Add(-1) == 0 {
		// The last reader unblocks the writer.
		semaphoreRelease(&rw.writerSem, false, 1)
	}
}

// Lock is same as sync.RWMutex
func (rw *RWMutex) Lock() {
	// First, resolve competition with other writers.
	rw.w.Lock()
	// Announce to readers there is a pending writer.
	r := rw.readerCount.Add(-rwmutexMaxReaders) + rwmutexMaxReaders
	// Wait for active readers.
	if r != 0 && rw.readerWait.Add(r) != 0 {
		semaphoreAcquire(&rw.writerSem)
	}
}

// TryLock is same as sync.RWMutex
func (rw *RWMutex) TryLock() bool {
	if !rw.w.TryLock() {
		return false
	}
	if !rw.readerCount.CompareAndSwap(0, -rwmutexMaxReaders) {
		rw.w.Unlock()
		return false
	}
	return true
}

// Unlock is same as sync.RWMutex
func (rw *RWMutex) Unlock() {
	// Announce to readers there is no active writer.
	r := rw.readerCount.Add(rwmutexMaxReaders)
	if r >= rwmutexMaxReaders {
		panic("sync: Unlock of unlocked RWMutex")
	}
	// Unblock blocked readers, if any.
	for i := 0; i < int(r); i++ {
		semaphoreRelease(&rw.readerSem, false, 0)
	}
	// Allow other writers to proceed.
	rw.w.Unlock()
}

// UpgradeWLock upgrade the read lock to the write lock
func (rw *RWMutex) UpgradeWLock() {
	if !rw.upgradableReadMode {
		panic("sync: Upgrade outside of upgradableReadLock not allowed")
	}
	rw.upgraded = true
	// Announce to readers there is a pending writer.
	r := rw.readerCount.Add(-rwmutexMaxReaders) + rwmutexMaxReaders
	// Wait for active readers.
	if r != 0 && rw.readerWait.Add(r) != 0 {
		semaphoreAcquire(&rw.writerSem)
	}
}

// UpgradableRUnlock unlocks either the write-lock if it is upgraded
// or unlock just the upgradeableRead-lock if not upgraded
func (rw *RWMutex) UpgradableRUnlock() {
	rw.upgradableReadMode = false
	if rw.upgraded {
		rw.upgraded = false
		rw.Unlock()
	} else {
		rw.w.Unlock()
	}
}

// UpgradableRLock acquires an upgradable-read-lock which can be later upgraded to write-lock,
// Example usage:
//
//	mutex.UpgradableRLock()
//	defer mutex.UpgradableRUnlock()
//	// read-lock acquired section. We can return here safely if an error occurs
//	mutex.UpgradeWLock()
//	// critical section with exclusive right access
func (rw *RWMutex) UpgradableRLock() {
	// First, resolve competition with other writers.
	// Disallow writers to acquire the lock
	rw.w.Lock()
	if rw.readerCount.Load() < 0 {
		panic("reader count can not be negative. We have the write lock")
	}
	rw.upgradableReadMode = true
}

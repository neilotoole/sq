package progress

import (
	"testing"
	"time"
)

// TestNopBar exercises the no-op bar, including its unexported methods, which
// are otherwise unreachable from the external test package.
func TestNopBar(_ *testing.T) {
	var b Bar = nopBar{}
	b.Incr(1)
	b.markShown()
	b.markHidden()
	b.refresh(time.Now())
	b.Stop()
}

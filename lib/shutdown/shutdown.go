package shutdown

import (
	"os"
	"sync"

	"github.com/neilotoole/go-lg/lg"
)

var shutdownFns = []func() error{}
var mu sync.Mutex

func Add(shutdownFn func() error) {
	mu.Lock()
	defer mu.Unlock()

	if shutdownFn == nil {
		return
	}

	shutdownFns = append(shutdownFns, shutdownFn)
}

func Shutdown(code int) {
	mu.Lock()
	defer mu.Unlock()
	lg.Debugf("Shutdown() invoked with code: %d", code)
	ret := runShutdownFns()

	if code != 0 {
		ret = code
	}
	lg.Debugf("shutting down for real, with code: %d", ret)
	os.Exit(ret)
}

func runShutdownFns() int {

	ret := 0

	for i, fn := range shutdownFns {
		lg.Debugf("running shutdown function %d of %d...", i+1, len(shutdownFns))
		if fn == nil {
			// should never happen
			lg.Debugf("skipping nil shutdown fn [%d]", i)
			continue
		}
		err := fn()
		if err != nil {
			lg.Errorf("shutdown function error: %v", err)
			ret = -1
		}
	}

	return ret
}

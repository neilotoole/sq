// Package shutdown provides a mechanism for executing cleanup/shutdown tasks
// as the application is exiting. Use shutdown.Add() to register a task.
// This shutdown package is a stopgap mechanism until the codebase has stabilized
// at which point each package/construct will handle their own cleanup.
package shutdown

import (
	"os"
	"sync"

	"github.com/neilotoole/go-lg/lg"
)

var shutdownFns = []func() error{}
var mu sync.Mutex

// Add a function to execute during shutdown (nil functions are ignored).
func Add(fn func() error) {
	mu.Lock()
	defer mu.Unlock()

	if fn == nil {
		return
	}

	shutdownFns = append(shutdownFns, fn)
}

// Shutdown executes all functions registered via Add(). If all functions execute
// without error then os.Exit(code) is invoked, otherwise os.Exit(1) is invoked.
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

		if fn == nil {
			// should never happen
			continue
		}
		lg.Debugf("running shutdown function %d of %d...", i+1, len(shutdownFns))
		err := fn()
		if err != nil {
			lg.Errorf("shutdown function error: %v", err)
			ret = 1
		}
	}

	return ret
}

package testing

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"
)

/*
TODO: move this package to its own repo?
e.g. by keeping them in a var DEBUGOT_TESTS
TODO: implement the rest of the testing.T functions
TODO: run should use goroutines, and wait for the goroutine to finish. This allows us to implement t.Skip() etc.
*/

type T struct {
	Name   string // Name of test.
	Failed bool
}

func (t *T) Errorf(format string, args ...interface{}) {
	writeMu.Lock()
	defer writeMu.Unlock()
	t.Failed = true

	printErr(fmt.Sprintf("\n\nERROR: %s\n\n", t.Name))
	msg := fmt.Sprintf(format, args...)
	printErr(msg)

}

func (t *T) FailNow() {
	writeMu.Lock()
	defer writeMu.Unlock()
	t.Failed = true
	printErr(fmt.Sprintf("\n\nFAILED: %s\n\n", t.Name))
}

func NewWith(name string) *T {

	return &T{Name: name}
}

func NewT() *T {

	return &T{Name: callerName()}
}

func callerName() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}

func funcName(funk interface{}) string {
	//s := fmt.Println("Name of function : " + runtime.FuncForPC(reflect.ValueOf(funk).Pointer()).Name())
	return runtime.FuncForPC(reflect.ValueOf(funk).Pointer()).Name()

}

type Test func(t *T)

var suiteMu = &sync.Mutex{}
var writeMu = &sync.Mutex{}

func Run(suite string, tests ...Test) {

	suiteMu.Lock()
	defer suiteMu.Unlock()

	time.Sleep(time.Millisecond * 100)

	printOut(fmt.Sprintf("\n\n>>> Running suite %q...\n", suite))

	ts := make([]*T, len(tests))

	for i, test := range tests {

		//name :=
		ts[i] = NewWith(funcName(test))
		os.Stderr.Sync()
		os.Stdout.Sync()
		test(ts[i])
	}

	failed := 0

	for _, t := range ts {
		if t.Failed {
			failed++
		}
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	if failed == 0 {
		printOut(fmt.Sprintf("\n*** suite %q: PASS all %d tests ***\n", suite, len(tests)))
		return
	}

	printErr(fmt.Sprintf("\n*** suite %q: FAIL %d of %d tests ***\n", suite, failed, len(tests)))
	os.Exit(1)

}

func printOut(s string) {
	os.Stderr.Sync()
	os.Stdout.Sync()
	os.Stdout.WriteString(s)
	os.Stdout.Sync()
}

func printErr(s string) {
	os.Stderr.Sync()
	os.Stdout.Sync()
	os.Stderr.WriteString(s)
	os.Stderr.Sync()
}

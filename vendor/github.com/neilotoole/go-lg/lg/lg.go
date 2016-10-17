/*
Package lg is a yet another simple logging package, intended primarily for code
debugging/tracing purposes. It outputs in Apache httpd error log format.

	lg.Debugf("the answer is: %d", 42)
	// results in
	I [24/Aug/2016:20:26:41 -0600] [example.go:13:example.MyFunction] the answer is: 42

By default, lg outputs to stdout/stderr, but you can specify an alternative
destination using lg.Use(), or by setting the envar "__LG_LOG_FILEPATH"
somewhere in your app's bootstrap process using

	os.Setenv("__LG_LOG_FILE_PATH", "/path/to/file.log").

You can use lg.Levels() to specify which log levels to produce output for;
lg.Exclude() to prevent logging for specified packages; and lg.Disable() / lg.Enable()
to disable/enable logging entirely.

See https://github.com/neilotoole/go-lg for more information.
*/
package lg

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

func init() {

	// if the envar is set, then we use that as the default log location.
	envar := "__LG_LOG_FILEPATH"
	path, ok := os.LookupEnv(envar)
	if ok {

		if path == "/dev/null" {
			Use(ioutil.Discard)
			return
		}

		parent := filepath.Dir(path)
		err := os.MkdirAll(parent, os.ModePerm)
		if err != nil {
			fmt.Fprintf(os.Stderr, fmt.Sprintf("Error: logging disabled: unable to create parent dir for log file path %q: %v", path, err))
			return
		}

		logFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, fmt.Sprintf("Error: logging disabled: unable to initialize log file %q: %v", path, err))
			return
		}
		Use(logFile)
	}
}

// Enable turns on log output.
func Enable() {
	//enabled = true
	filter = filter | levelEnabled
}

// Disable turns off log output.
func Disable() {
	filter = filter &^ levelEnabled
}

var filter = levelEnabled | LevelDebug | LevelWarn | LevelError

// Excluded is a list of (fully-qualified) package names to be excluded from
// log output. Any sub-packages will also be excluded.
var Excluded = []string{}

// Exclude appends the provided (full-qualified) package names to the list of packages
// to be excluded from log output. Any sub-packages will also be excluded.
func Exclude(pkgs ...string) {

	Mu.Lock()
	defer Mu.Unlock()
	Excluded = append(Excluded, pkgs...)
}

// Level represents log levels.
type Level uint8

const (
	levelEnabled Level = 1
	LevelDebug         = 2
	LevelWarn          = 4
	LevelError         = 8
	LevelAll           = 14
)

func (lv Level) binary() string {
	return fmt.Sprintf("%08b", lv)
}

// Levels specifies the complete set of log levels to produce output for.
func Levels(levels ...Level) {
	Mu.Lock()
	defer Mu.Unlock()

	// clear the levels
	filter = filter &^ LevelAll
	for _, lvl := range levels {
		// and enable each level
		filter = filter | lvl
	}
}

// apacheFormat is the standard apache timestamp format.
const apacheFormat = `02/Jan/2006:15:04:05 -0700`

var wOut io.Writer = os.Stdout
var wErr io.Writer = os.Stderr

// Mu is the lg package's mutex. The mutex is exposed for the rare circumstance where
// the client needs to mutate package variables (e.g. ExcludePkgs or ExcludeLevels)
// in a concurrency situation. Typically these vars are configured once during the
// init() phase.
var Mu sync.Mutex

// LongFnName determines whether the full path/to/pkg.func is used. Default is pkg.func.
var LongFnName = false

// LongFilePath determines whether the full /path/to/file.go is used. Default is file.go.
var LongFilePath = false

// Use specifies the log output destination. The default is os.Stdout/os.Stderr.
func Use(dest io.Writer) {
	Mu.Lock()
	defer Mu.Unlock()
	wOut = dest
	wErr = dest
}

// Debugf logs a debug message.
func Debugf(format string, v ...interface{}) {
	log(false, 1, LevelDebug, format, v...)
}

// Warnf logs a warning message.
func Warnf(format string, v ...interface{}) {
	log(false, 1, LevelWarn, format, v...)
}

// Errorf logs an error message.
func Errorf(format string, v ...interface{}) {
	log(false, 1, LevelError, format, v...)
}

// Fatalf is similar to Errorf, but calls os.Exit(1) after logging the message.
// Additionally, if the log destination is not os.Stdout or os.Stderr, then
// the message is also printed to os.Stderr. This function will invoke os.Exit(1)
// even if logging is disabled.
func Fatalf(format string, v ...interface{}) {

	Mu.Lock()
	defer Mu.Unlock()

	msg := fmt.Sprintf(format, v...)
	if allowed(LevelError) {
		log(true, 1, LevelError, msg)

		if wOut != os.Stdout && wOut != os.Stderr {
			fmt.Fprintln(os.Stderr, msg)
		}
	}
	os.Exit(1)
}

// allowed returns true if logging is enabled and the specified logging level
// is allowed.
func allowed(level Level) bool {
	return filter&levelEnabled > 0 && filter&level > 0
}

func log(locked bool, calldepth int, level Level, format string, v ...interface{}) {

	if !allowed(level) {
		return
	}

	t := time.Now()
	exclPkgs := Excluded

	pc := make([]uintptr, 10) // at least 1 entry needed
	runtime.Callers(2+calldepth, pc)
	fnName := runtime.FuncForPC(pc[0]).Name()

	if len(exclPkgs) > 0 || !LongFnName {
		// fnName looks like github.com/neilotoole/go-lg/test/filter/pkg2.LogDebug
		fnNameParts := strings.Split(fnName, "/")
		lastPart := fnNameParts[len(fnNameParts)-1]

		if !LongFnName {
			fnName = lastPart
		}

		if len(exclPkgs) > 0 {
			// we need only the package part of the last element
			// e.g. pkg2.LogDebug -> pkg2
			fnNameParts[len(fnNameParts)-1] = lastPart[:strings.IndexRune(lastPart, '.')]
			pkgName := strings.Join(fnNameParts, "/")
			for _, exclPkg := range exclPkgs {
				if strings.Index(pkgName, exclPkg) == 0 {
					return
				}
			}
		}
	}

	_, file, line, ok := runtime.Caller(calldepth + 1)
	if !ok {
		file = "???"
		line = 0
	} else if !LongFilePath {
		// We just want the file name, not the whole path
		parts := strings.Split(file, "/")
		file = parts[len(parts)-1]
	}

	stamp := t.Format(apacheFormat)
	var lvlText string
	switch level {
	case LevelError:
		lvlText = "E"
	case LevelWarn:
		lvlText = "W"
	default:
		lvlText = "I"
	}

	// E [08/Jun/2013:11:28:58 -0700] [ql.go:60] ql.ToSQL: my message text
	tpl := `%s [%s] [%s:%d:%s] %s`
	str := fmt.Sprintf(tpl, lvlText, stamp, file, line, fnName, fmt.Sprintf(format, v...))
	if !locked {
		Mu.Lock()
		defer Mu.Unlock()
	}

	if level == LevelError {
		fmt.Fprintln(wErr, str)
		return
	}
	fmt.Fprintln(wOut, str)
}

// Log is the logging interface.
type Log interface {
	Debugf(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
}

// Depth returns a log interface that logs calls as per the package-level Debugf
// and Errorf function, but using the additional call depth parameter. This is
// useful, for example, in situations where a utility function is logging on
// behalf of its parent function.
//
//    lg.Depth(1).Errorf("a bad thing happened in my parent: %v", err)
func Depth(calldepth int) Log {
	return &calldepthLogger{depth: calldepth}
}

type calldepthLogger struct {
	depth int
}

// Debugf logs a debug message.
func (cd *calldepthLogger) Debugf(format string, v ...interface{}) {
	log(false, 1+cd.depth, LevelDebug, format, v...)
}

// Warnf logs a debug message.
func (cd *calldepthLogger) Warnf(format string, v ...interface{}) {
	log(false, 1+cd.depth, LevelWarn, format, v...)
}

// Errorf logs an error message.
func (cd *calldepthLogger) Errorf(format string, v ...interface{}) {
	log(false, 1+cd.depth, LevelError, format, v...)

}

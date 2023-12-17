package errz

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"runtime"
	"strconv"
	"strings"
)

var _ Opt = (*Skip)(nil)

// Skip is an Opt that can be passed to Err or New that
// indicates how many frames to skip when recording the stack trace.
// This is useful when wrapping errors in helper functions.
//
//	func handleErr(err error) error {
//		slog.Default().Error("Oh noes", "err", err)
//		return errz.Err(err, errz.Skip(1))
//	}
//
// Skipping too many frames will panic.
type Skip int

func (s Skip) apply(e *errz) {
	*(e.stack) = (*e.stack)[int(s):]
}

const unknown = "unknown"

// Frame represents a program counter inside a stack frame.
// For historical reasons if Frame is interpreted as a uintptr
// its value represents the program counter + 1.
type Frame uintptr

// pc returns the program counter for this frame;
// multiple frames may have the same PC value.
func (f Frame) pc() uintptr { return uintptr(f) - 1 }

// file returns the full path to the file that contains the
// function for this Frame's pc.
func (f Frame) file() string {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return unknown
	}
	file, _ := fn.FileLine(f.pc())
	return file
}

// line returns the line number of source code of the
// function for this Frame's pc.
func (f Frame) line() int {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return 0
	}
	_, line := fn.FileLine(f.pc())
	return line
}

// name returns the name of this function, if known.
func (f Frame) name() string {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return unknown
	}
	return fn.Name()
}

// Format formats the frame according to the fmt.Formatter interface.
//
//	%s    source file
//	%d    source line
//	%n    function name
//	%v    equivalent to %s:%d
//
// Format accepts flags that alter the printing of some verbs, as follows:
//
//	%+s   function name and path of source file relative to the compile time
//	      GOPATH separated by \n\t (<funcname>\n\t<path>)
//	%+v   equivalent to %+s:%d
func (f Frame) Format(s fmt.State, verb rune) {
	switch verb {
	case 's':
		switch {
		case s.Flag('+'):
			_, _ = io.WriteString(s, f.name())
			_, _ = io.WriteString(s, "\n\t")
			_, _ = io.WriteString(s, f.file())
		default:
			_, _ = io.WriteString(s, path.Base(f.file()))
		}
	case 'd':
		_, _ = io.WriteString(s, strconv.Itoa(f.line()))
	case 'n':
		_, _ = io.WriteString(s, funcName(f.name()))
	case 'v':
		f.Format(s, 's')
		_, _ = io.WriteString(s, ":")
		f.Format(s, 'd')
	}
}

// MarshalText formats a stacktrace Frame as a text string. The output is the
// same as that of fmt.Sprintf("%+v", f), but without newlines or tabs.
func (f Frame) MarshalText() ([]byte, error) {
	name := f.name()
	if name == unknown {
		return []byte(name), nil
	}
	return []byte(fmt.Sprintf("%s %s:%d", name, f.file(), f.line())), nil
}

// StackTrace contains a stack of Frames from innermost (newest)
// to outermost (oldest), as well as the error value that resulted
// in this stack trace.
type StackTrace struct {
	// Error is the error value that resulted in this stack trace.
	Error error

	// Frames is the ordered list of frames that make up this stack trace.
	Frames Frames
}

// Frames is the ordered list of frames that make up a stack trace.
type Frames []Frame

// Format formats the stack of Frames according to the fmt.Formatter interface.
//
//	%s	lists source files for each Frame in the stack
//	%v	lists the source file and line number for each Frame in the stack
//
// Format accepts flags that alter the printing of some verbs, as follows:
//
//	%+v   Prints filename, function, and line number for each Frame in the stack.
func (fs Frames) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		switch {
		case s.Flag('+'):
			for i, f := range fs {
				if i != 0 {
					_, _ = io.WriteString(s, "\n")
				}
				f.Format(s, verb)
			}
		case s.Flag('#'):
			fmt.Fprintf(s, "%#v", []Frame(fs))
		default:
			fs.formatSlice(s, verb)
		}
	case 's':
		fs.formatSlice(s, verb)
	}
}

// formatSlice will format this Frames into the given buffer as a slice of
// Frame, only valid when called with '%s' or '%v'.
func (fs Frames) formatSlice(s fmt.State, verb rune) {
	_, _ = io.WriteString(s, "[")
	for i, f := range fs {
		if i > 0 {
			_, _ = io.WriteString(s, " ")
		}
		f.Format(s, verb)
	}
	_, _ = io.WriteString(s, "]")
}

// LogValue implements slog.LogValuer.
func (st *StackTrace) LogValue() slog.Value {
	if st == nil || len(st.Frames) == 0 {
		return slog.Value{}
	}

	return slog.StringValue(fmt.Sprintf("%+v", st.Frames))
}

// stack represents a stack of program counters.
type stack []uintptr

func (s *stack) Format(st fmt.State, verb rune) {
	if s == nil {
		fmt.Fprint(st, "<nil>")
	}
	switch verb { //nolint:gocritic
	case 'v':
		switch { //nolint:gocritic
		case st.Flag('+'):
			for _, pc := range *s {
				f := Frame(pc)
				fmt.Fprintf(st, "\n%+v", f)
			}
		}
	}
}

type stackTracer interface {
	stackTrace() *StackTrace
	inner() error
}

func (s *stack) stackTrace() *StackTrace {
	f := make([]Frame, len(*s))
	for i := 0; i < len(f); i++ {
		f[i] = Frame((*s)[i])
	}
	return &StackTrace{Frames: f}
}

func callers(skip int) *stack {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	// var st stack = pcs[0:n]
	var st stack = pcs[skip:n]
	return &st
}

// funcName removes the path prefix component of a function's name reported by func.Name().
func funcName(name string) string {
	i := strings.LastIndex(name, "/")
	name = name[i+1:]
	i = strings.Index(name, ".")
	return name[i+1:]
}

// Stacks returns all stack trace(s) attached to err. If err has been wrapped
// more than once, there may be multiple stack traces. Generally speaking, the
// final stack trace is the most interesting; you can use [errz.LastStack] if
// you're just interested in that one.
//
// The returned [StackTrace.Frames] can be printed using fmt "%+v".
func Stacks(err error) []*StackTrace {
	if err == nil {
		return nil
	}

	var stacks []*StackTrace
	for err != nil {
		if tracer, ok := err.(stackTracer); ok { //nolint:errorlint
			st := tracer.stackTrace()
			if st != nil {
				stacks = append(stacks, st)
			}
		}

		// err = errors.Unwrap(err)
		err = errors.Unwrap(err)
	}

	return stacks
}

// LastStack returns the last of any stack trace(s) attached to err, or nil.
// Contrast with [errz.Stacks], which returns all stack traces attached
// to any error in the chain. But if you only want to examine one stack,
// the final stack trace is usually the most interesting, which is why this
// function exists.
//
// The returned StackTrace.Frames can be printed using fmt "%+v".
func LastStack(err error) *StackTrace {
	if err == nil {
		return nil
	}

	var (
		// var ez *errz
		ok     bool
		tracer stackTracer
		inner  error
	)
	for err != nil {
		tracer, ok = err.(stackTracer) //nolint:errorlint
		if !ok || tracer == nil {
			return nil
		}

		inner = tracer.inner()
		if inner == nil {
			return tracer.stackTrace()
		}

		//nolint:errorlint
		if _, ok = inner.(stackTracer); !ok {
			return tracer.stackTrace()
		}

		err = inner
	}

	return nil
}

package typetest

import (
	"reflect"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// Matcher is an expected record value that's checked by a rule rather than
// compared for equality: for a value that can't be known in advance, such as
// the current time that an expression default like now() fills in. It's
// only an expectation, never an input, and only on a Defaulted column whose
// case sets DefaultExpr: Resolve rejects any other use. See NewExprCase.
type Matcher interface {
	// Match returns nil if got, the value read back, satisfies the matcher,
	// and otherwise an error saying what was wanted and what was got.
	Match(got any) error

	// String describes the matcher, for failure messages.
	String() string
}

// NotNullOf returns a Matcher for any non-NULL value of Go type T. A typed
// nil, such as a nil []byte, counts as NULL.
func NotNullOf[T any]() Matcher {
	return notNullOf[T]{}
}

type notNullOf[T any] struct{}

// Match implements Matcher.
func (notNullOf[T]) Match(got any) error {
	if _, ok := got.(T); ok && !isNil(got) {
		return nil
	}
	if got == nil {
		return errz.Errorf("want non-NULL %s, got NULL", reflect.TypeFor[T]())
	}
	// fmtGoValue shows a typed nil as such: []byte(nil), not [].
	return errz.Errorf("want non-NULL %s, got %T: %s", reflect.TypeFor[T](), got, fmtGoValue(got))
}

// String implements Matcher.
func (notNullOf[T]) String() string {
	return "NotNullOf[" + reflect.TypeFor[T]().String() + "]"
}

// isNil reports whether v is nil, or a typed nil: a nil chan, func,
// interface, map, pointer or slice.
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() { //nolint:exhaustive // Only these kinds can be nil.
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

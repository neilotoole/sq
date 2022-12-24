// Package tutil contains basic generic test utilities.
package tutil

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/stretchr/testify/require"
)

// SkipIff skips t if b is true. If msgAndArgs is non-empty, its first
// element must be a string, which can be a format string if there are
// additional elements.
//
// Examples:
//
//	tutil.SkipIff(t, a == b)
//	tutil.SkipIff(t, a == b, "skipping because a == b")
//	tutil.SkipIff(t, a == b, "skipping because a is %v and b is %v", a, b)
func SkipIff(t testing.TB, b bool, format string, args ...any) {
	if b {
		if format == "" {
			t.SkipNow()
		} else {
			t.Skipf(format, args...)
		}
	}
}

// StructFieldValue extracts the value of fieldName from arg strct.
// If strct is nil, nil is returned.
// The function will panic if strct is not a struct (or pointer to struct), or if
// the struct does not have fieldName. The returned value may be nil if the
// field is a pointer and is nil.
//
// Note that this function uses reflection, and may panic. It is only
// to be used by test code.
//
// See also: SliceFieldValues, SliceFieldKeyValues.
func StructFieldValue(fieldName string, strct any) any {
	if strct == nil {
		return nil
	}

	// zv is the zero value of reflect.Value, which can be returned by FieldByName
	zv := reflect.Value{}

	e := reflect.Indirect(reflect.ValueOf(strct))
	if e.Kind() != reflect.Struct {
		panic(fmt.Sprintf("strct expected to be struct but was %s", e.Kind()))
	}

	f := e.FieldByName(fieldName)
	if f == zv { //nolint:govet
		// According to govet:
		//
		//   reflectvaluecompare: avoid using == with reflect.Value
		//
		// Maybe we should be using f.IsZero instead?

		panic(fmt.Sprintf("struct (%T) does not have field {%s}", strct, fieldName))
	}
	fieldValue := f.Interface()
	return fieldValue
}

// SliceFieldValues takes a slice of structs, and returns a slice
// containing the value of fieldName for each element of slice.
//
// Note that slice can be []interface{}, or a typed slice (e.g. []*Person).
// If slice is nil, nil is returned. If slice has len zero, an empty slice
// is returned. The function panics if slice is not a slice, or if any element
// of slice is not a struct (excepting nil elements).
//
// Note that this function uses reflection, and may panic. It is only
// to be used by test code.
//
// See also: StructFieldValue, SliceFieldKeyValues.
func SliceFieldValues(fieldName string, slice any) []any {
	if slice == nil {
		return nil
	}

	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic(fmt.Sprintf("arg slice expected to be a slice, but was {%T}", slice))
	}

	iSlice := InterfaceSlice(slice)
	retVals := make([]any, len(iSlice))

	for i := range iSlice {
		retVals[i] = StructFieldValue(fieldName, iSlice[i])
	}

	return retVals
}

// SliceFieldKeyValues is similar to SliceFieldValues, but instead of
// returning a slice of field values, it returns a map containing two
// field values, a "key" and a "value". For example:
//
//	persons := []*person{
//	  {Name: "Alice", Age: 42},
//	  {Name: "Bob", Age: 27},
//	}
//
//	m := SliceFieldKeyValues("Name", "Age", persons)
//	// map[Alice:42 Bob:27]
//
// Note that this function uses reflection, and may panic. It is only
// to be used by test code.
//
// See also: StructFieldValue, SliceFieldValues.
func SliceFieldKeyValues(keyFieldName, valFieldName string, slice any) map[any]any {
	if slice == nil {
		return nil
	}

	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic(fmt.Sprintf("arg slice expected to be a slice, but was {%T}", slice))
	}

	iSlice := InterfaceSlice(slice)
	m := make(map[any]any, len(iSlice))

	for i := range iSlice {
		key := StructFieldValue(keyFieldName, iSlice[i])
		val := StructFieldValue(valFieldName, iSlice[i])

		m[key] = val
	}

	return m
}

// InterfaceSlice converts a typed slice (such as []string) to []interface{}.
// If slice is already of type []interface{}, it is returned unmodified.
// Otherwise a new []interface{} is constructed. If slice is nil, nil is
// returned. The function panics if slice is not a slice.
//
// Note that this function uses reflection, and may panic. It is only
// to be used by test code.
func InterfaceSlice(slice any) []any {
	if slice == nil {
		return nil
	}

	// If it's already an []interface{}, then just return
	if iSlice, ok := slice.([]any); ok {
		return iSlice
	}

	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic(fmt.Sprintf("arg slice expected to be a slice, but was {%T}", slice))
	}

	// Keep the distinction between nil and empty slice input
	if s.IsNil() {
		return nil
	}

	ret := make([]any, s.Len())

	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}

	return ret
}

// StringSlice accepts a slice of arbitrary type (e.g. []int64 or []interface{})
// and returns a slice of string.
func StringSlice(slice any) []string {
	if slice == nil {
		return nil
	}

	// If it's already []string, return directly
	if sSlice, ok := slice.([]string); ok {
		return sSlice
	}

	iSlice := InterfaceSlice(slice)
	sSlice := make([]string, len(iSlice))
	for i := range iSlice {
		sSlice[i] = fmt.Sprintf("%v", iSlice[i])
	}

	return sSlice
}

// Name is a convenience function for building a test name to
// pass to t.Run.
//
//	t.Run(testh.Name("my_test", 1), func(t *testing.T) {
//
// The most common usage is with test names that are file
// paths.
//
//	testh.Name("path/to/file") --> "path_to_file"
//
// Any element of arg that prints to empty string is skipped.
func Name(args ...any) string {
	var parts []string
	var s string
	for _, a := range args {
		s = fmt.Sprintf("%v", a)
		if s == "" {
			continue
		}

		s = strings.ReplaceAll(s, "/", "_")
		s = stringz.TrimLen(s, 40) // we don't want it to be too long
		parts = append(parts, s)
	}

	s = strings.Join(parts, "_")
	if s == "" {
		return "empty"
	}

	return s
}

// SkipShort invokes t.Skip if testing.Short and arg skip are both true.
func SkipShort(t *testing.T, skip bool) {
	if skip && testing.Short() {
		t.Skip("Skipping long-running test because -short is true.")
	}
}

// Val returns the fully dereferenced value of i. If i
// is nil, nil is returned. If i has type *(*string),
// Val(i) returns string.
// Useful for testing.
func Val(i any) any {
	if i == nil {
		return nil
	}

	v := reflect.ValueOf(i)
	for {
		if !v.IsValid() {
			return nil
		}

		switch v.Kind() { //nolint:exhaustive
		default:
			return v.Interface()
		case reflect.Ptr, reflect.Interface:
			if v.IsNil() {
				return nil
			}
			v = v.Elem()
			// Loop again
			continue
		}
	}
}

// AssertCompareFunc matches several of the testify/require funcs.
// It can be used to choose assertion comparison funcs in test cases.
type AssertCompareFunc func(require.TestingT, any, any, ...any)

// Verify that a sample of the require funcs match AssertCompareFunc.
var (
	_ AssertCompareFunc = require.Equal
	_ AssertCompareFunc = require.GreaterOrEqual
	_ AssertCompareFunc = require.Greater
)

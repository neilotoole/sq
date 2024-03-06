//go:build go1.15
// +build go1.15

package jcolorenc

import (
	"reflect"
	"unsafe"
)

func extendSlice(t reflect.Type, s *slice, n int) slice {
	arrayType := reflect.ArrayOf(n, t.Elem())
	arrayData := reflect.New(arrayType)
	reflect.Copy(arrayData.Elem(), reflect.NewAt(t, unsafe.Pointer(s)).Elem())
	return slice{
		data: unsafe.Pointer(arrayData.Pointer()),
		len:  s.len,
		cap:  n,
	}
}

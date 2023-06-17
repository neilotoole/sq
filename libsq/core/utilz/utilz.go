// Package utilz contains utility functions that don't have a home.
package utilz

// All returns a new slice containing elems.
func All[T any](elems ...T) []T {
	a := make([]T, len(elems))
	copy(a, elems)
	return a
}

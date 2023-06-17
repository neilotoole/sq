// Package loz contains functionality supplemental to samber/lo.
// Ideally these functions would be merged into that package.
package loz

// All returns a new slice containing elems.
func All[T any](elems ...T) []T {
	a := make([]T, len(elems))
	copy(a, elems)
	return a
}

// ToSliceType returns a new slice of type T, having performed
// type conversion on each element of in.
func ToSliceType[S, T any](in ...S) (out []T, ok bool) {
	out = make([]T, len(in))
	var a any
	for i := range in {
		a = in[i]
		out[i], ok = a.(T)
		if !ok {
			return nil, false
		}
	}

	return out, true
}

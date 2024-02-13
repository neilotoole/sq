// Package tailbuf contains a tail buffer [Buf] of fixed size that provides a
// window on the tail of the items written via [Buf.Write]. Start with
// [tailbuf.New] to create a [Buf].
package tailbuf

import "context"

// Buf is an append-only fixed-size circular buffer that provides a window on
// the tail of items written to the buffer. The zero value is not usable; invoke
// [tailbuf.New] to create a [Buf]. It is not safe for concurrent use.
//
// Note the terms "nominal buffer" and "tail window" (or just "window"). The
// nominal buffer is the complete list of items written to Buf via the Buf.Write
// or Buf.WriteAll methods. However, Buf drops the oldest items as it fills
// (which is the entire point of this package): the tail window is the subset of
// the nominal buffer that is currently available. Some of Buf's methods take
// arguments that are indices into the nominal buffer, for example [Buf.NominalSlice].
type Buf[T any] struct {
	// back is the cursor for the oldest item.
	back int
	// front is the cursor for the newest item.
	front int
	// count is the number of items written.
	count int
	// window is the circular buffer.
	window []T
}

// New returns a new Buf with the specified capacity. It panics if capacity is
// less than 1.
func New[T any](capacity int) *Buf[T] {
	if capacity < 1 {
		panic("capacity must be > 0")
	}
	return &Buf[T]{
		window: make([]T, capacity),
		back:   -1,
		front:  -1,
	}
}

// Write appends t to the buffer. If the buffer fills, the oldest item
// is overwritten. The buffer is returned for chaining.
func (b *Buf[T]) Write(t T) *Buf[T] {
	b.write(t)
	return b
}

// WriteAll appends items to the buffer. If the buffer fills, the oldest items
// are overwritten. The buffer is returned for chaining.
func (b *Buf[T]) WriteAll(a ...T) *Buf[T] {
	for i := range a {
		b.write(a[i])
	}
	return b
}

// write appends item [Buf]. If the buffer is full, the oldest item is
// overwritten.
func (b *Buf[T]) write(item T) {
	b.count++
	switch {
	case b.front == -1:
		b.back = 0
	case b.count > len(b.window):
		b.back = (b.back + 1) % len(b.window)
	default:
	}

	b.front = (b.front + 1) % len(b.window)
	b.window[b.front] = item
}

// Window returns the current window of items in the buffer. The window is
// returned as a slice, with the oldest item at index 0. The returned slice is
// a copy, so the caller is free to mutate the slice. If the buffer is empty,
// the returned slice is empty.
func (b *Buf[T]) Window() []T {
	if b.count < 1 {
		return make([]T, 0)
	}
	if b.count <= len(b.window) {
		return b.window[0:b.count]
	}
	if b.front >= b.back {
		return b.window[b.back : b.front+1]
	}

	return append(b.window[b.back:], b.window[:b.front+1]...)
}

// Count returns the number of items written to the buffer.
func (b *Buf[T]) Count() int {
	return b.count
}

func (b *Buf[T]) InBounds(i int) bool {
	if b.count == 0 {
		return false
	}
	start, end := b.Bounds()
	return i >= start && i <= end // TODO: should be < end?
}

// Bounds returns the start and end indices of the current window vs the nominal
// complete list of items written to the buffer. If the buffer is empty, start
// and end are both 0. The returned end value always equals [Buf.Count].
func (b *Buf[T]) Bounds() (start, end int) {
	if b.front == -1 {
		return 0, 0
	}

	end = b.count
	if b.count <= len(b.window) {
		start = 0

	} else {
		start = b.count - len(b.window)
	}

	return start, end
}

// NominalSlice returns a slice into the nominal buffer, using the standard
// [inclusive:exclusive] slicing mechanics. NominalSlice panics if start is negative or
// end is less than start.
//
// Boundary checking is relaxed. If the buffer is empty, the returned slice
// is empty. Otherwise, if the requested range is completely outside the bounds
// of the tail window, the returned slice is empty; if the range overlaps with
// the tail window, the returned slice contains the overlapping items. If strict
// boundary checking is important to you, use [Buf.InBounds] to check the start
// and end indices.
//
// NominalSlice is approximately functionality equivalent to reslicing the result of
// [Buf.Window], but it avoids wasteful copying (and has relaxed boundarcy
// checking).
//
//	buf := tailbuf.New[int](3).WriteAll(1, 2, 3)
//	a := buf.Window()[0:2]
//	b := buf.NominalSlice(0, 2)
//	assert.Equal(t, a, b)
func (b *Buf[T]) NominalSlice(start, end int) []T {
	offset := b.Offset()
	return b.TailSlice(start-offset, end-offset)
}

//func (b *Buf[T]) NominalSlice(start, end int) []T {
//	switch {
//	case start < 0:
//		panic("start must be >= 0")
//	case end < start:
//		panic("end must be >= start")
//	case end == start:
//		return make([]T, 0)
//	case b.count == 0:
//		return make([]T, 0)
//	case start > b.count:
//		return make([]T, 0)
//	case end < b.count-len(b.window):
//		return make([]T, 0)
//	case b.front > b.back:
//		offset := b.count - len(b.window)
//		s := b.window[start-offset : end-offset]
//		return s
//	case b.count == 1:
//		// Special case: the buffer has only one item.
//		if start == 0 && end > 1 {
//			return []T{b.window[0]}
//		}
//		return make([]T, 0)
//
//	default: // b.back > b.front
//		var offset int
//		if b.count > len(b.window) {
//			offset = b.count - len(b.window)
//		}
//		backo := start - offset
//		//fronto := end - offset
//		//fmt.Printf("found it! back:%d, front: %d, start: %d, end: %d, backo: %d, fronto: %d\n",
//		//	b.back, b.front, start, end, backo, fronto)
//
//		back := b.window[b.back+backo:]
//		front := b.window[:b.front]
//
//		x := append(back, front...)
//		return x
//
//		//panic()
//		//return append(b.window[backo:], b.window[:fronto+1]...)
//	}
//}

func (b *Buf[T]) TailSliceNew(start, end int) []T {
	switch {
	case start < 0:
		panic("start must be >= 0")
	case end < start:
		panic("end must be >= start")
	case end == start:
		return make([]T, 0)
	case b.count == 0:
		return make([]T, 0)
	case b.count == 1:
		// Special case: the buffer has only one item.
		if start == 0 && end > 1 {
			return []T{b.window[0]}
		}
		return make([]T, 0)
	case start >= b.count:
		return make([]T, 0)
	}
	//case end < b.count-len(b.window):
	//	return make([]T, 0)

	if b.front > b.back {
		if end > b.count {
			end = b.count
		}
		if end > len(b.window) {
			end = len(b.window)
		}
		s := b.window[start:end]
		return s
	}

	// b.front < b.back
	if end >= b.count {
		end = b.count - 1
	} else if end > len(b.window) {
		end = len(b.window)
	}

	back := b.window[b.back+start:]
	front := b.window[:b.front+end-len(b.window)+1]

	x := append(back, front...)
	return x

}

func (b *Buf[T]) TailSlice(start, end int) []T {
	switch {
	case start < 0:
		panic("start must be >= 0")
	case end < start:
		panic("end must be >= start")
	case end == start, b.count == 0, start >= b.count:
		return make([]T, 0)
	case b.count == 1:
		// Special case: the buffer has only one item.
		if start == 0 && end > 1 {
			return []T{b.window[0]}
		}
		return make([]T, 0)
	case b.front > b.back:
		if end > b.count {
			end = b.count
		} else if end > len(b.window) {
			end = len(b.window)
		}
		return b.window[start:end]
	default: // b.back > b.front
		if end >= b.count {
			end = b.count - 1
		} else if end > len(b.window) {
			end = len(b.window)
		}

		s := make([]T, 0, end-start)
		s = append(s, b.window[b.back+start:]...)
		return append(s, b.window[:b.front+end-len(b.window)+1]...)
	}
}

// Capacity returns the capacity of Buf, which is the size specified when the
// buffer was created.
func (b *Buf[T]) Capacity() int {
	return len(b.window)
}

// Offset returns the offset of the current window vs the nominal complete list
// of items written to the buffer. It is effectively the count of discarded
// items. If the buffer is empty, the returned offset is 0.
func (b *Buf[T]) Offset() int {
	if b.count <= len(b.window) {
		return 0
	}

	return b.count - len(b.window)
}

func (b *Buf[T]) AtOffset(offset int) T {
	if b.back == -1 {
		var t T
		return t
	}

	x := offset - b.count - len(b.window)

	i := (b.front + offset) % len(b.window)
	i = i - b.count

	return b.window[x]

}

func (b *Buf[T]) Front() T {
	if b.front == -1 {
		var t T
		return t
	}
	return b.window[b.front]
}

func (b *Buf[T]) Back() T {
	if b.back == -1 {
		var t T
		return t
	}
	return b.window[b.back]
}

// Apply applies fn to each item in the tail window, in oldest-to-newest order.
// If the buffer is empty, fn is not invoked. The buffer is returned for
// chaining. Example:
//
//	buf := tailbuf.New[string](3)
//	buf.WriteAll("a", "b  ", "   c  ")
//	buf.Apply(strings.ToUpper).Apply(strings.TrimSpace)
//	fmt.Println(buf.Window())
//	// Output: [A B C]
//
// Using Apply is cheaper than getting the window and applying the function
// manually, as it avoids the allocation of a new slice by Buf.Window.
//
// For more control or to handle errors, use [Buf.Do].
func (b *Buf[T]) Apply(fn func(item T) T) *Buf[T] {
	if b.count == 0 {
		return b
	}

	if b.front > b.back {
		for i := b.back; i <= b.front; i++ {
			b.window[i] = fn(b.window[i])
		}
		return b
	}

	for i := b.back; i < len(b.window); i++ {
		b.window[i] = fn(b.window[i])
	}

	for i := 0; i <= b.front; i++ {
		b.window[i] = fn(b.window[i])
	}
	return b
}

// Do applies fn to each item in the tail window, in oldest-to-newest order,
// replacing each item with the value returned by successful invocation of fn.
// If fn returns an error, the item is not replaced. Execution is halted if any
// invocation of fn returns an error, and that error is returned to the caller.
// Thus a partial application of fn may occur.
//
// If Buf is empty, fn is not invoked.
//
// The index arg to fn is the index of the item in the tail window. You can use
// the offset arg to compute the index of the item in the nominal buffer.
//
//	nominalIndex := index + offset
//
// REVISIT: Should index be the tailIndex instead?
//
// The context is not checked for cancellation between iterations. The context
// should be checked in fn if desired.
func (b *Buf[T]) Do(ctx context.Context, fn func(ctx context.Context, item T, index, offset int) (T, error)) error {
	if b.count == 0 {
		return nil
	}

	var v T
	var err error

	if b.front > b.back {
		for i := b.back; i <= b.front; i++ {
			v, err = fn(ctx, b.window[i], i, i-b.back)
			if err != nil {
				return err
			}
			b.window[i] = v
		}
		return nil
	}

	for i := b.back; i < len(b.window); i++ {
		v, err = fn(ctx, b.window[i], i, i-b.back)
		if err != nil {
			return err
		}
		b.window[i] = v
	}

	for i := 0; i <= b.front; i++ {
		v, err = fn(ctx, b.window[i], i, i-b.back)
		if err != nil {
			return err
		}
		b.window[i] = v
	}

	return nil
}

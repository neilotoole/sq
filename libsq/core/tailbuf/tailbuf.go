// Package tailbuf contains a tail buffer [Buf] of fixed size that provides a
// window on the tail of the items written via [Buf.Write]. Start with
// [tailbuf.New] to create a [Buf].
package tailbuf

import "context"

// Buf is an append-only fixed-size circular buffer that provides a window on
// the tail of items written to the buffer. The zero value is technically
// usable, but not very useful. Instead, invoke [tailbuf.New] to create a Buf.
// Buf is not safe for concurrent use.
//
// Note the terms "nominal buffer" and "tail window" (or just "window"). The
// nominal buffer is the complete list of items written to Buf via the Buf.Write
// or Buf.WriteAll methods. However, Buf drops the oldest items as it fills
// (which is the entire point of this package): the tail window is the subset of
// the nominal buffer that is currently available. Some of Buf's methods take
// arguments that are indices into the nominal buffer, for example [Buf.Slice].
type Buf[T any] struct {
	// window is the circular buffer.
	window []T
	// back is the cursor for the oldest item.
	back int
	// front is the cursor for the newest item.
	front int
	// count is the number of items written.
	count int
}

// New returns a new Buf with the specified capacity. It panics if capacity is
// less than 1.
func New[T any](capacity int) *Buf[T] {
	if capacity < 0 {
		panic("capacity must be >= 0") // FIXME: make zero value usable
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
	if len(b.window) == 0 {
		return b
	}

	b.write(t)
	return b
}

// WriteAll appends items to the buffer. If the buffer fills, the oldest items
// are overwritten. The buffer is returned for chaining.
func (b *Buf[T]) WriteAll(a ...T) *Buf[T] {
	if len(b.window) == 0 {
		return b
	}

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

// Tail returns a slice containing the current tail window of items in the
// buffer, with the oldest item at index 0. Depending on the state of Buf, the
// returned slice may be a slice of Buf's internal data, or a copy. Thus you
// should copy the returned slice before modifying it, or instead use TailSlice.
func (b *Buf[T]) Tail() []T {
	switch {
	case b.count < 1:
		return make([]T, 0) // REVISIT: why not return a nil slice?
	case b.count <= len(b.window):
		return b.window[0:b.count]
	case b.front >= b.back:
		return b.window[b.back : b.front+1]
	default:
		s := make([]T, 0, len(b.window))
		s = append(s, b.window[b.back:]...)
		return append(s, b.window[:b.front+1]...)
	}
}

// Count returns the total number of items written to the buffer.
func (b *Buf[T]) Count() int {
	return b.count
}

// InBounds returns true if the index i of the nominal buffer is within the
// bounds of the tail window. That is to say, InBounds returns true if the ith
// item written to the buffer is still in the tail window.
func (b *Buf[T]) InBounds(i int) bool {
	if b.count == 0 || i < 0 {
		return false
	}
	start, end := b.Bounds()
	return i >= start && i <= end // TODO: should be < end?
}

// Bounds returns the start and end indices of the tail window vs the nominal
// buffer. If the buffer is empty, start and end are both 0. The returned
// values are the same as [Buf.Offset] and [Buf.Count].
func (b *Buf[T]) Bounds() (start, end int) {
	return b.Offset(), b.Count()
}

// Slice returns a slice into the nominal buffer, using the standard
// [inclusive:exclusive] slicing mechanics.
//
// Boundary checking is relaxed. If the buffer is empty, the returned slice
// is empty. Otherwise, if the requested range is completely outside the bounds
// of the tail window, the returned slice is empty; if the range overlaps with
// the tail window, the returned slice contains the overlapping items. If strict
// boundary checking is important to you, use [Buf.InBounds] to check the start
// and end indices.
//
// Slice is approximately functionality equivalent to reslicing the result of
// [Buf.Tail], but it may avoid wasteful copying (and has relaxed boundary
// checking).
//
//	buf := tailbuf.New[int](3).WriteAll(1, 2, 3)
//	a := buf.Tail()[0:2]
//	b := buf.Slice(0, 2)
//	assert.Equal(t, a, b)
//
// If start < 0, zero is used. Slice panics if end is less than start.
func (b *Buf[T]) Slice(start, end int) []T {
	offset := b.Offset()
	start = start - offset
	if start < 0 {
		start = 0
	}
	end = end - offset
	if end <= start {
		return make([]T, 0)
	}

	return b.TailSlice(start, end)
}

// TailSlice returns a slice of the tail window, using the standard
// [inclusive:exclusive] slicing mechanics, but with permissive bounds checking.
// The slice is freshly allocated, so the caller is free to mutate it.
//
// A call to TailSlice is equivalent to reslicing the result of [Buf.Tail], but
// it may avoid unnecessary copying, depending on the state of Buf.
//
//	buf := tailbuf.New[int](3).WriteAll(1, 2, 3)
//	a := buf.Tail()[0:2]
//	b := buf.TailSlice(0, 2)
//	fmt.Println("a:", a, "b:", b)
//	// Output: a: [1 2] b: [1 2]
//
// If Buf is empty, the returned slice is empty. Otherwise, if the requested
// range is completely outside the bounds of the tail window, the returned slice
// is empty; if the range overlaps with the tail window, the returned slice
// contains the overlapping items. If strict boundary checking is important, use
// [Buf.InBounds] to check the start and end indices.
//
// Slice panics if start is negative or end is less than start.
//
// See also: [Buf.Slice], [Buf.Tail], [Buf.Bounds], [Buf.InBounds].
func (b *Buf[T]) TailSlice(start, end int) []T {
	switch {
	case start < 0:
		panic("start must be >= 0")
	case end < start:
		panic("end must be >= start")
	case len(b.window) == 0, end == start, b.count == 0, start >= b.count:
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
		s := make([]T, 0, end-start)
		return append(s, b.window[start:end]...)
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

// Capacity returns the capacity of Buf, which is the fixed size specified when
// the buffer was created.
func (b *Buf[T]) Capacity() int {
	return len(b.window)
}

// Reset resets the buffer to its initial state. The buffer is returned for
// chaining.
func (b *Buf[T]) Reset() *Buf[T] {
	b.back = -1
	b.front = -1
	b.count = 0
	return b
}

// Offset returns the offset of the current window vs the nominal complete list
// of items written to the buffer. It is effectively the count of items that
// have slipped out of the tail window. If the buffer is empty, the returned
// offset is 0.
func (b *Buf[T]) Offset() int {
	if b.count <= len(b.window) {
		return 0
	}

	return b.count - len(b.window)
}

// Front returns the newest item in the tail window. If Buf is empty, the zero
// value of T is returned.
func (b *Buf[T]) Front() T {
	if b.front == -1 {
		var t T
		return t
	}
	return b.window[b.front]
}

// Back returns the oldest item in the tail window. If Buf empty, the zero value
// of T is returned.
func (b *Buf[T]) Back() T {
	if b.back == -1 {
		var t T
		return t
	}
	return b.window[b.back]
}

// Apply applies fn to each item in the tail window, in oldest-to-newest order.
// If Buf is empty, fn is not invoked. The buffer is returned for chaining.
//
//	buf := tailbuf.New[string](3)
//	buf.WriteAll("a", "b  ", "   c  ")
//	buf.Apply(strings.TrimSpace).Apply(strings.ToUpper)
//	fmt.Println(buf.Tail())
//	// Output: [A B C]
//
// Using Apply is cheaper than getting the slice via [Buf.Tail] and applying fn
// manually, as it avoids the possible allocation of a new slice by Buf.Tail.
//
// For more control, or to handle errors, use [Buf.Do].
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

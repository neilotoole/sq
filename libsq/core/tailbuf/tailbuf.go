// Package tailbuf contains a tail buffer [Buf] of fixed size that provides a
// window on the tail of the items written via [Buf.Write] or [Buf.WriteAll].
// Start with [tailbuf.New] to create a [Buf].
package tailbuf

import "context"

// Buf is an append-only fixed-size circular buffer that provides a window on
// the tail of items written to the buffer. The zero value is technically
// usable, but not very useful. Instead, invoke [tailbuf.New] to create a Buf.
// Buf is not safe for concurrent use.
//
// Note the terms "nominal buffer" and "tail window" (or just "window"). The
// nominal buffer is the complete list of items written to Buf via the
// [Buf.Write] or [Buf.WriteAll] methods. However, Buf drops the oldest items as
// it fills (which is the entire point of this package): the tail window is the
// subset of the nominal buffer that is currently available. Some of Buf's
// methods take arguments that are indices into the nominal buffer, for example
// [SliceNominal].
type Buf[T any] struct {
	// zero is the zero value of T, used for zeroing elements of the in-use
	// window so that after operations like Buf.DropBack we don't accidentally
	// hold on to references. This is probably a premature optimization; needs
	// benchmarks.
	zero T

	// window is the circular buffer.
	window []T

	// len is the number of items currently in the buffer.
	len int

	// back is the cursor for the oldest item.
	back int

	// REVISIT: Do we need both back and front?
	// Could we just use back and len?

	// front is the cursor for the newest item.
	front int

	// written is the total number of items added via Buf.Write or Buf.WriteAll.
	written int
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

// Write appends t to the buffer. If the buffer fills, the oldest item is
// overwritten. The buffer is returned for chaining.
func (b *Buf[T]) Write(t T) *Buf[T] {
	if len(b.window) == 0 {
		// We won't actually store the item, but we still count it.
		b.written++
		return b
	}

	b.write(t)
	return b
}

// WriteAll appends items to the buffer. If the buffer fills, the oldest items
// are overwritten. The buffer is returned for chaining.
func (b *Buf[T]) WriteAll(a ...T) *Buf[T] {
	if len(b.window) == 0 {
		// We won't actually store the items, but we still count them.
		b.written += len(a)
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
	b.written++
	switch {
	case b.front == -1:
		b.back = 0
	case b.written > len(b.window):
		b.back = (b.back + 1) % len(b.window)
	default:
	}

	b.front = (b.front + 1) % len(b.window)
	b.window[b.front] = item
	if b.len < len(b.window) {
		b.len++
	}
}

// Tail returns a slice containing the items currently in the buffer, in
// oldest-to-newest order. If possible, the returned slice shares the buffer's
// internal window, but a fresh slice is allocated if necessary. Thus you
// should copy the returned slice before modifying it, or instead use
// [SliceTail].
func (b *Buf[T]) Tail() []T {
	switch {
	case b.len == 0:
		return b.window[:0]
	case b.len == 1:
		return b.window[b.front : b.front+1]
	case b.front > b.back:
		return b.window[b.back : b.front+1]
	default:
		s := make([]T, b.len)
		copy(s, b.window[b.back:])
		copy(s[len(b.window)-b.back:], b.window[:b.front+1])
		return s
	}
}

// tailNewSlice is like Buf.Tail but it always returns a fresh slice.
func (b *Buf[T]) tailNewSlice() []T {
	switch {
	case b.len == 0:
		return make([]T, 0)
	case b.len == 1:
		return []T{b.window[b.front]}
	case b.front > b.back:
		s := make([]T, b.front-b.back+1)
		copy(s, b.window[b.back:b.front+1])
		return s
	default:
		s := make([]T, b.len)
		copy(s, b.window[b.back:])
		copy(s[len(b.window)-b.back:], b.window[:b.front+1])
		return s
	}
}

// Written returns the total number of items written to the buffer.
func (b *Buf[T]) Written() int {
	return b.written
}

// InBounds returns true if the index i of the nominal buffer is within the
// bounds of the tail window. That is to say, InBounds returns true if the ith
// item written to the buffer is still in the tail window.
func (b *Buf[T]) InBounds(i int) bool {
	if b.written == 0 || i < 0 || len(b.window) == 0 {
		return false
	}
	start, end := b.Bounds()
	return i >= start && i <= end // TODO: should be < end?
}

// Bounds returns the start and end indices of the tail window vs the nominal
// buffer. If the buffer is empty, start and end are both 0. The returned
// values are the same as [Buf.Offset] and [Buf.Written].
func (b *Buf[T]) Bounds() (start, end int) {
	return b.Offset(), b.Written()
}

// Capacity returns the capacity of Buf, which is the fixed size specified when
// the buffer was created.
func (b *Buf[T]) Capacity() int {
	return len(b.window)
}

// Len returns the number of items currently in the buffer.
func (b *Buf[T]) Len() int {
	return b.len
}

// zeroTail zeroes out the items in the tail window.
func (b *Buf[T]) zeroTail() {
	if b.front > b.back {
		for i := b.back; i <= b.front; i++ {
			b.window[i] = b.zero
		}
	} else {
		for i := b.back; i < len(b.window); i++ {
			b.window[i] = b.zero
		}
		for i := 0; i <= b.front; i++ {
			b.window[i] = b.zero
		}
	}
}

// Reset resets the buffer to its initial state, including the value returned
// by [Buf.Written]. The buffer is returned for chaining. Any items in the
// buffer are zeroed out.
//
// See also: [Buf.Clear].
func (b *Buf[T]) Reset() *Buf[T] {
	b.Clear()
	b.written = 0
	return b
}

// Clear removes all items from the buffer, zeroing all values. This is similar
// to [Buf.Reset], but note that the value returned by [Buf.Written] is
// unchanged. The buffer is returned for chaining.
//
// See also: [Buf.Reset].
func (b *Buf[T]) Clear() *Buf[T] {
	b.zeroTail()

	b.back = -1
	b.front = -1
	b.len = 0

	return b
}

// Offset returns the offset of the current window vs the nominal complete list
// of items written to the buffer. It is effectively the count of items that
// have slipped out of the tail window. If the buffer is empty, the returned
// offset is 0.
func (b *Buf[T]) Offset() int {
	if b.written <= len(b.window) {
		return 0
	}

	return b.written - len(b.window)
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

// PopFront removes and returns the newest item in the tail window.
func (b *Buf[T]) PopFront() T {
	if b.front == -1 {
		var t T
		return t
	}

	item := b.window[b.front]
	b.window[b.front] = b.zero
	if b.front == b.back {
		b.back = -1
		b.front = -1
	} else {
		b.front = (b.front - 1 + len(b.window)) % len(b.window)
	}
	b.len--
	return item
}

// Back returns the oldest item in the tail window. If Buf is empty, the zero
// value of T is returned.
func (b *Buf[T]) Back() T {
	if b.back == -1 {
		var t T
		return t
	}
	return b.window[b.back]
}

// DropBack removes the oldest item in the tail window.
// See also: [Buf.PopBack].
func (b *Buf[T]) DropBack() {
	if b.back == -1 {
		return
	}

	b.window[b.back] = b.zero
	if b.front == b.back {
		b.back = -1
		b.front = -1
	} else {
		b.back = (b.back + 1) % len(b.window)
	}
	b.len--
}

// DropBackN removes the oldest n items from the tail, zeroing out the items.
func (b *Buf[T]) DropBackN(n int) {
	if b.len == 0 || n < 1 {
		return
	}

	if n >= b.len {
		b.Clear()
		return
	}

	b.len -= n

	if b.front > b.back {
		for i := 0; i < n; i++ {
			b.window[b.back] = b.zero
			b.back = (b.back + 1) % len(b.window)
		}
		return
	}

	for i := 0; i < n; i++ {
		b.window[b.back] = b.zero
		b.back = (b.back + 1) % len(b.window)
	}
}

// PopBack removes and returns the oldest item in the tail window. If the buffer
// is empty, the zero value of T is returned.
func (b *Buf[T]) PopBack() T {
	if b.back == -1 {
		var t T
		return t
	}

	item := b.window[b.back]
	b.window[b.back] = b.zero
	if b.front == b.back {
		b.back = -1
		b.front = -1
	} else {
		b.back = (b.back + 1) % len(b.window)
	}
	b.len--
	return item
}

// PopBackN removes and returns the oldest n items in the tail window. Any
// removed items are zeroed out from the buffer's internal window. On return,
// the slice (which is always freshly allocated) contains the removed items, in
// oldest-to-newest order, and Buf.Len is reduced by n. If n is greater than the
// number of items in the tail window, all items in the tail window are removed
// and returned.
func (b *Buf[T]) PopBackN(n int) []T {
	if b.len == 0 || n < 1 {
		return make([]T, 0)
	}

	if n >= b.len {
		s := b.tailNewSlice()
		b.Clear()
		return s
	}

	b.len -= n
	if b.front > b.back {
		s := make([]T, n)
		for i := 0; i < n; i++ {
			s[i] = b.window[b.back]
			b.window[b.back] = b.zero
			b.back = (b.back + 1) % len(b.window)
		}
		return s
	}

	s := make([]T, n)
	for i := 0; i < n; i++ {
		s[i] = b.window[b.back]
		b.window[b.back] = b.zero
		b.back = (b.back + 1) % len(b.window)
	}
	return s
}

// PopFrontN removes and returns the newest n items in the tail window. Any
// removed items are zeroed out from the buffer's internal window. On return,
// the slice (which is always freshly allocated) contains the removed items, in
// oldest-to-newest order, and Buf.Len is reduced by n. If n is greater than the
// number of items in the tail window, all items in the tail window are removed
// and returned.
func (b *Buf[T]) PopFrontN(n int) []T {
	if b.len == 0 || n < 1 {
		return make([]T, 0)
	}

	if n >= b.len {
		s := b.tailNewSlice()
		b.Clear()
		return s
	}

	b.len -= n

	if b.front > b.back {
		s := make([]T, n)
		for i := n - 1; i >= 0; i-- {
			s[i] = b.window[b.front]
			b.window[b.front] = b.zero
			b.front = (b.front - 1 + len(b.window)) % len(b.window)
		}
		return s
	}

	s := make([]T, n)
	for i := n - 1; i >= 0; i-- {
		s[i] = b.window[b.front]
		b.window[b.front] = b.zero
		b.front = (b.front - 1 + len(b.window)) % len(b.window)
	}
	return s
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
	if b.len == 0 {
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
	if b.len == 0 {
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

// SliceNominal returns a slice into the nominal buffer, using the standard
// [inclusive:exclusive] slicing mechanics.
//
// Boundary checking is relaxed. If the buffer is empty, the returned slice is
// empty. Otherwise, if the requested range is completely outside the bounds of
// the tail window, the returned slice is empty; if the range overlaps with the
// tail window, the returned slice contains the overlapping items. If strict
// boundary checking is important to you, use [Buf.InBounds] to check the start
// and end indices.
//
// SliceNominal is approximately functionality equivalent to reslicing the
// result of [Buf.Tail], but it may avoid wasteful copying (and has relaxed
// bounds checking).
//
//	buf := tailbuf.New[int](3).WriteAll(1, 2, 3)
//	a := buf.Tail()[0:2]
//	b := buf.SliceNominal(0, 2)
//	assert.Equal(t, a, b)
//
// If start < 0, zero is used. SliceNominal panics if end is less than start.
func SliceNominal[T any](b *Buf[T], start, end int) []T {
	offset := b.Offset()
	start -= offset
	if start < 0 {
		start = 0
	}
	end -= offset
	if end <= start {
		return make([]T, 0)
	}

	return SliceTail(b, start, end)
}

// SliceTail returns a slice of the tail window, using the standard
// [inclusive:exclusive] slicing mechanics, but with permissive bounds checking.
// The slice is freshly allocated, so the caller is free to mutate it.
//
// A call to SliceTail is equivalent to reslicing the result of [Buf.Tail], but
// it may avoid unnecessary copying, depending on the state of Buf.
//
//	buf := tailbuf.New[int](3).WriteAll(1, 2, 3)
//	a := buf.Tail()[0:2]
//	b := buf.SliceTail(0, 2)
//	fmt.Println("a:", a, "b:", b)
//	// Output: a: [1 2] b: [1 2]
//
// If Buf is empty, the returned slice is empty. Otherwise, if the requested
// range is completely outside the bounds of the tail window, the returned slice
// is empty; if the range overlaps with the tail window, the returned slice
// contains the overlapping items. If strict boundary checking is important, use
// [Buf.InBounds] to check the start and end indices.
//
// SliceTail panics if start is negative or end is less than start.
//
// See also: [SliceNominal], [Buf.Tail], [Buf.Bounds], [Buf.InBounds].
func SliceTail[T any](b *Buf[T], start, end int) []T {
	switch {
	case start < 0:
		panic("start must be >= 0")
	case end < start:
		panic("end must be >= start")
	case len(b.window) == 0, end == start, b.written == 0, start >= b.written:
		return make([]T, 0)
	case b.written == 1, b.front == b.back:
		// Special case: the buffer has only one item.
		if start == 0 && end >= 1 {
			return []T{b.window[0]}
		}
		return make([]T, 0)
	case b.front > b.back:
		if end > b.written {
			end = b.written
		}
		if end > len(b.window) {
			end = len(b.window)
		}
		s := make([]T, 0, end-start)
		return append(s, b.window[start:end]...)
	default: // b.back > b.front
		if end >= b.written {
			end = b.written - 1
		}
		if end > len(b.window) {
			end = len(b.window)
		}
		s := make([]T, 0, end-start)
		s = append(s, b.window[b.back+start:]...)
		frontIndex := b.front + end - len(b.window) + 1

		return append(s, b.window[:frontIndex]...)
	}
}

// Package tailbuf contains a tail buffer [Buf] of fixed size that provides a
// window on the tail of the items written via [Buf.Write]. Start with
// [tailbuf.New] to create a [Buf].
package tailbuf

import "fmt"

// Buf is an append-only circular buffer of fixed size that provides a window on
// the tail of the items written to the buffer. The zero value is not usable;
// invoke [tailbuf.New] to create a [Buf]. It is not safe for concurrent use.
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

// New returns a new Buf with the specified size. It panics if size is less
// than 1.
func New[T any](size int) *Buf[T] {
	if size < 1 {
		panic("size must be a positive integer")
	}
	return &Buf[T]{
		window: make([]T, size),
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

// Range returns the start and end indices of the current window vs the nominal
// complete list of items written to the buffer. If the buffer is empty, start
// and end are both 0. The returned end value always equals [Buf.Count].
func (b *Buf[T]) Range() (start, end int) {
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

func (b *Buf[T]) Slice(start, end int) []T {

	switch {
	case start < 0:
		panic("start must be >= 0")
	case end < start:
		panic("end must be >= start")
	case end == start:
		return make([]T, 0)
	case b.count == 0:
		return make([]T, 0)
	case start > b.count:
		return make([]T, 0)
	case end < b.count-len(b.window):
		return make([]T, 0)
	case b.front > b.back:
		offset := b.count - len(b.window)
		//backo := start - offset
		//fronto := end - offset
		//s := b.window[backo:fronto]
		//backo :=
		//fronto :=
		s := b.window[start-offset : end-offset]
		return s
	default: // b.back > b.front
		offset := b.count - len(b.window)
		backo := start - offset
		fronto := end - offset
		fmt.Printf("found it! back:%d, front: %d, start: %d, end: %d, backo: %d, fronto: %d\n",
			b.back, b.front, start, end, backo, fronto)

		back := b.window[b.back+backo:]
		front := b.window[:b.front]

		x := append(back, front...)
		return x

		//panic()
		//return append(b.window[backo:], b.window[:fronto+1]...)
	}
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
	if b.back == -1 {
		var t T
		return t
	}

	return b.window[b.front]
}

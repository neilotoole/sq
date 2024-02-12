package tailbuffer

import "github.com/emirpasic/gods/queues/circularbuffer"

func Huzzah() {
	circularbuffer.New(3)
}

type Buffer[T any] struct {
	window []T
	next   int
	back   int
	count  int
}

func New[T any](size int) *Buffer[T] {
	return &Buffer[T]{window: make([]T, size), next: -1, back: -1}
}

func (b *Buffer[T]) Append(item T) *Buffer[T] {
	b.count++
	if b.next == -1 {
		b.next = 0
	}
	b.back = (b.back + 1) % len(b.window)
	b.window[b.back] = item
	if b.back == b.next {
		b.next = (b.next + 1) % len(b.window)
	}
	return b
}

func (b *Buffer[T]) Window() []T {
	if b.count < 1 {
		return make([]T, 0)
	}
	if b.count <= len(b.window) {
		return b.window[0:b.count]
	}

	if b.back >= b.next {
		return b.window[b.next : b.back+1]
	}
	return append(b.window[b.next:], b.window[:b.back+1]...)
}

func (b *Buffer[T]) Range() (start, end int) {
	if b.next == -1 {
		return 0, 0
	}

	return b.back, b.next
}

func (b *Buffer[T]) AtOffset(offset int) T {
	if b.next == -1 {
		var t T
		return t
	}

	x := offset - b.count - len(b.window)

	i := (b.back + offset) % len(b.window)
	i = i - b.count

	return b.window[x]

}

func (b *Buffer[T]) Front() T {
	if b.next == -1 {
		var t T
		return t
	}

	return b.window[b.back]
}

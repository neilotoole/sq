package tailbuffer

import "github.com/emirpasic/gods/queues/circularbuffer"

func Huzzah() {
	circularbuffer.New(3)
}

type Buffer[T any] struct {
	window []T
	front  int
	back   int
	count  int
}

func New[T any](size int) *Buffer[T] {
	return &Buffer[T]{
		window: make([]T, size),
		back:   -1,
		front:  -1,
	}
}

func (b *Buffer[T]) Write(item T) *Buffer[T] {
	b.count++
	switch {
	case b.front == -1:
		b.back = 0
	case b.count < len(b.window):
	default:
		b.back = (b.back + 1) % len(b.window)
	}

	//if b.front == -1 {
	//	b.back = 0
	//} else if b.front == len(b.window) {
	//	b.back = (b.back + 1) % len(b.window)
	//}

	b.front = (b.front + 1) % len(b.window)
	b.window[b.front] = item
	//if b.front == b.back {
	//	b.back = (b.back + 1) % len(b.window)
	//}
	return b
}

func (b *Buffer[T]) Window() []T {
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

func (b *Buffer[T]) Range() (start, end int) {
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

func (b *Buffer[T]) AtOffset(offset int) T {
	if b.back == -1 {
		var t T
		return t
	}

	x := offset - b.count - len(b.window)

	i := (b.front + offset) % len(b.window)
	i = i - b.count

	return b.window[x]

}

func (b *Buffer[T]) Front() T {
	if b.back == -1 {
		var t T
		return t
	}

	return b.window[b.front]
}

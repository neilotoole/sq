package tailbuf

// InternalWindow exposes Buf's internal window for testing.
func InternalWindow[T any](b *Buf[T]) []T {
	return b.window
}

// TailNewSlice exposes Buf's internal tailNewSlice for testing.
func TailNewSlice[T any](b *Buf[T]) []T {
	return b.tailNewSlice()
}

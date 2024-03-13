package tailbuf_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/tailbuf"
)

func TestTail(t *testing.T) {
	buf := tailbuf.New[rune](3)
	gotLen := buf.Len()
	require.Equal(t, 0, gotLen)
	require.Equal(t, 0, buf.Written())
	require.Empty(t, buf.Tail())
	require.Empty(t, tailbuf.TailNewSlice(buf))

	buf.Write('a')
	require.Equal(t, 1, buf.Written())
	gotLen = buf.Len()
	require.Equal(t, 1, gotLen)
	gotTail := buf.Tail()
	require.Equal(t, []rune{'a'}, gotTail)
	require.Equal(t, gotTail, tailbuf.TailNewSlice(buf))

	buf.Write('b')
	require.Equal(t, 2, buf.Written())
	gotLen = buf.Len()
	require.Equal(t, 2, gotLen)
	gotTail = buf.Tail()
	require.Equal(t, []rune{'a', 'b'}, gotTail)
	require.Equal(t, gotTail, tailbuf.TailNewSlice(buf))

	buf.Write('c')
	require.Equal(t, 3, buf.Written())
	gotLen = buf.Len()
	require.Equal(t, 3, gotLen)
	gotTail = buf.Tail()
	require.Equal(t, []rune{'a', 'b', 'c'}, gotTail)
	require.Equal(t, gotTail, tailbuf.TailNewSlice(buf))

	buf.Write('d')
	require.Equal(t, 4, buf.Written())
	gotLen = buf.Len()
	require.Equal(t, 3, gotLen)
	gotTail = buf.Tail()
	require.Equal(t, []rune{'b', 'c', 'd'}, gotTail)
	require.Equal(t, gotTail, tailbuf.TailNewSlice(buf))

	buf.Write('e')
	require.Equal(t, 5, buf.Written())
	gotLen = buf.Len()
	require.Equal(t, 3, gotLen)
	gotTail = buf.Tail()
	require.Equal(t, []rune{'c', 'd', 'e'}, gotTail)
	require.Equal(t, gotTail, tailbuf.TailNewSlice(buf))

	buf.Write('f')
	require.Equal(t, 6, buf.Written())
	gotLen = buf.Len()
	require.Equal(t, 3, gotLen)
	gotTail = buf.Tail()
	require.Equal(t, []rune{'d', 'e', 'f'}, gotTail)
	require.Equal(t, gotTail, tailbuf.TailNewSlice(buf))

	buf.Write('g')
	require.Equal(t, 7, buf.Written())
	gotLen = buf.Len()
	require.Equal(t, 3, gotLen)
	gotTail = buf.Tail()
	require.Equal(t, []rune{'e', 'f', 'g'}, gotTail)
	require.Equal(t, gotTail, tailbuf.TailNewSlice(buf))

	buf.WriteAll('h', 'i', 'j')
	require.Equal(t, 10, buf.Written())
	gotLen = buf.Len()
	require.Equal(t, 3, gotLen)
	gotTail = buf.Tail()
	require.Equal(t, []rune{'h', 'i', 'j'}, gotTail)
	require.Equal(t, gotTail, tailbuf.TailNewSlice(buf))
}

func TestBuf(t *testing.T) {
	testCases := []struct {
		add                rune
		wantStart, wantEnd int
		wantWindow         []rune
	}{
		{add: 'a', wantStart: 0, wantEnd: 1, wantWindow: []rune{'a'}},
		{add: 'b', wantStart: 0, wantEnd: 2, wantWindow: []rune{'a', 'b'}},
		{add: 'c', wantStart: 0, wantEnd: 3, wantWindow: []rune{'a', 'b', 'c'}},
		{add: 'd', wantStart: 1, wantEnd: 4, wantWindow: []rune{'b', 'c', 'd'}},
		{add: 'e', wantStart: 2, wantEnd: 5, wantWindow: []rune{'c', 'd', 'e'}},
		{add: 'f', wantStart: 3, wantEnd: 6, wantWindow: []rune{'d', 'e', 'f'}},
		{add: 'g', wantStart: 4, wantEnd: 7, wantWindow: []rune{'e', 'f', 'g'}},
		{add: 'h', wantStart: 5, wantEnd: 8, wantWindow: []rune{'f', 'g', 'h'}},
	}

	buf := tailbuf.New[rune](3)

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%d_%s", i, string(tc.add)), func(t *testing.T) {
			buf.Write(tc.add)
			require.Equal(t, tc.wantEnd, buf.Written())
			require.Equal(t, tc.add, buf.Front())
			window := buf.Tail()
			require.Equal(t, tc.wantWindow, window)
			start, end := buf.Bounds()
			require.Equal(t, tc.wantStart, start)
			require.Equal(t, tc.wantEnd, end)
			s := tailbuf.SliceNominal(buf, start, end+1)
			require.Equal(t, window, s)
		})
	}
}

func TestBounds(t *testing.T) {
	buf := tailbuf.New[string](3)
	start, end := buf.Bounds()
	require.Equal(t, 0, start)
	require.Equal(t, 0, end)

	buf.WriteAll("a", "b", "c")
	start, end = buf.Bounds()
	require.Equal(t, 0, start)
	require.Equal(t, 3, end)

	buf.WriteAll("d", "e")
	start, end = buf.Bounds()
	require.Equal(t, 2, start)
	require.Equal(t, 5, end)
}

func TestSlice(t *testing.T) {
	buf := tailbuf.New[int](3)
	buf.WriteAll(0, 1, 2)

	start, end := buf.Bounds()
	require.Equal(t, 0, start)
	require.Equal(t, 3, end)
	s := tailbuf.SliceNominal(buf, start, end)
	require.Equal(t, []int{0, 1, 2}, s)

	s = tailbuf.SliceNominal(buf, 0, 0)
	require.Empty(t, s)

	s = tailbuf.SliceNominal(buf, 0, 1)
	require.Equal(t, []int{0}, s)
	s = tailbuf.SliceNominal(buf, 0, 2)
	require.Equal(t, []int{0, 1}, s)
	s = tailbuf.SliceNominal(buf, 0, 3)
	require.Equal(t, []int{0, 1, 2}, s)

	s = tailbuf.SliceNominal(buf, 1, 1)
	require.Empty(t, s)
	s = tailbuf.SliceNominal(buf, 1, 3)
	require.Equal(t, []int{1, 2}, s)

	buf.WriteAll(3, 4, 5)
	start, end = buf.Bounds()
	require.Equal(t, 3, start)
	require.Equal(t, 6, end)
	s = tailbuf.SliceNominal(buf, start, end)
	require.Equal(t, []int{3, 4, 5}, s)

	s = tailbuf.SliceNominal(buf, 3, 3)
	require.Empty(t, s)
	s = tailbuf.SliceNominal(buf, 3, 4)
	require.Equal(t, []int{3}, s)
	s = tailbuf.SliceNominal(buf, 3, 5)
	require.Equal(t, []int{3, 4}, s)

	buf.WriteAll(6, 7)
	s = tailbuf.SliceNominal(buf, 6, 7)
	require.Equal(t, []int{6}, s)
}

func TestApply(t *testing.T) {
	buf := tailbuf.New[string](3)
	buf.WriteAll("In", "Xanadu  ", "   did", "Kubla  ", "Khan")
	buf.Apply(strings.ToUpper).Apply(strings.TrimSpace)
	got := buf.Tail()
	require.Equal(t, []string{"DID", "KUBLA", "KHAN"}, got)
}

func TestTailSlice(t *testing.T) {
	buf := tailbuf.New[int](10).WriteAll(1, 2, 3, 4, 5)
	a := buf.Tail()[0:2]
	b := tailbuf.SliceTail(buf, 0, 2)
	require.Equal(t, []int{1, 2}, b)
	require.Equal(t, a, b)
}

func TestTail_Slice_Equivalence(t *testing.T) {
	buf := tailbuf.New[int](10).WriteAll(1, 2, 3, 4, 5)
	a := buf.Tail()[0:2]
	b := tailbuf.SliceNominal(buf, 0, 2)
	require.Equal(t, []int{1, 2}, b)
	require.Equal(t, a, b)
}

func TestWrittenGTCapacity(t *testing.T) {
	buf := tailbuf.New[string](1)
	buf.WriteAll("a", "b")
	require.Equal(t, 1, buf.Capacity())
	require.Equal(t, 2, buf.Written())
	tail := buf.Tail()
	require.Equal(t, []string{"b"}, tail)
	tailSlice := tailbuf.SliceTail(buf, 0, 1)
	require.Equal(t, []string{"b"}, tailSlice)
	nomSlice := tailbuf.SliceNominal(buf, 0, 2)
	require.Equal(t, []string{"b"}, nomSlice)
	nomSlice = tailbuf.SliceNominal(buf, 0, 1)
	require.Empty(t, nomSlice)
}

func TestZeroCapacity(t *testing.T) {
	buf := tailbuf.New[rune](0)
	require.Equal(t, 0, buf.Capacity())
	require.Equal(t, 0, buf.Written())
	require.Equal(t, 0, buf.Len())
	require.Empty(t, buf.Tail())

	buf.Write('a')

	require.Equal(t, 1, buf.Written())
	gotLen := buf.Len()
	require.Equal(t, 0, gotLen)
	require.Empty(t, buf.Tail())
	require.Empty(t, tailbuf.SliceNominal(buf, 0, 1))
}

func TestPopFront(t *testing.T) {
	buf := tailbuf.New[rune](3)
	buf.WriteAll('a', 'b', 'c')
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 3, buf.Len())
	require.Equal(t, 'c', buf.Front())
	require.Equal(t, 'a', buf.Back())
	require.Equal(t, []rune{'a', 'b', 'c'}, buf.Tail())

	got := buf.PopFront()
	require.Equal(t, 'c', got)
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 2, buf.Len())
	require.Equal(t, 'b', buf.Front())
	require.Equal(t, []rune{'a', 'b', 0}, tailbuf.InternalWindow(buf))
	require.Equal(t, []rune{'a', 'b'}, buf.Tail())

	got = buf.PopFront()
	require.Equal(t, 'b', got)
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 1, buf.Len())
	require.Equal(t, 'a', buf.Front())
	require.Equal(t, []rune{'a', 0, 0}, tailbuf.InternalWindow(buf))
	require.Equal(t, []rune{'a'}, buf.Tail())

	got = buf.PopFront()
	require.Equal(t, 'a', got)
	require.Equal(t, 3, buf.Written())
	require.Empty(t, buf.Front())
	requireZeroInternalWindow(t, buf)
	require.Equal(t, 0, buf.Len())
	require.Equal(t, []rune{}, buf.Tail())

	got = buf.PopFront()
	require.Zero(t, got)
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 0, buf.Len())
	require.Empty(t, buf.Front())
	requireZeroInternalWindow(t, buf)
	require.Equal(t, []rune{}, buf.Tail())
}

func TestPopBack(t *testing.T) {
	buf := tailbuf.New[rune](3)
	buf.WriteAll('a', 'b', 'c')
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 3, buf.Len())
	require.Equal(t, 'c', buf.Front())
	require.Equal(t, 'a', buf.Back())
	require.Equal(t, []rune{'a', 'b', 'c'}, tailbuf.InternalWindow(buf))
	require.Equal(t, []rune{'a', 'b', 'c'}, buf.Tail())

	got := buf.PopBack()
	require.Equal(t, 'a', got)
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 2, buf.Len())
	require.Equal(t, 'b', buf.Back())
	require.Equal(t, []rune{0, 'b', 'c'}, tailbuf.InternalWindow(buf))
	require.Equal(t, []rune{'b', 'c'}, buf.Tail())

	got = buf.PopBack()
	require.Equal(t, 'b', got)
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 1, buf.Len())
	require.Equal(t, 'c', buf.Back())
	require.Equal(t, []rune{0, 0, 'c'}, tailbuf.InternalWindow(buf))

	got = buf.PopBack()
	require.Equal(t, 'c', got)
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 0, buf.Len())
	require.Empty(t, buf.Back())
	requireZeroInternalWindow(t, buf)

	got = buf.PopBack()
	require.Zero(t, got)
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 0, buf.Len())
	require.Empty(t, buf.Back())
	requireZeroInternalWindow(t, buf)
}

func TestDropBack(t *testing.T) {
	buf := tailbuf.New[rune](3)
	buf.WriteAll('a', 'b', 'c')
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 3, buf.Len())
	require.Equal(t, 'a', buf.Back())
	require.Equal(t, []rune{'a', 'b', 'c'}, buf.Tail())

	buf.DropBack()
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 2, buf.Len())
	require.Equal(t, 'b', buf.Back())
	require.Equal(t, []rune{0, 'b', 'c'}, tailbuf.InternalWindow(buf))
	require.Equal(t, []rune{'b', 'c'}, buf.Tail())

	buf.DropBack()
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 1, buf.Len())
	require.Equal(t, 'c', buf.Back())
	require.Equal(t, []rune{0, 0, 'c'}, tailbuf.InternalWindow(buf))
	require.Equal(t, []rune{'c'}, buf.Tail())

	buf.DropBack()
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 0, buf.Len())
	require.Empty(t, buf.Back())
	requireZeroInternalWindow(t, buf)
	require.Empty(t, buf.Tail())

	buf.DropBack()
	require.Equal(t, 3, buf.Written())
	require.Equal(t, 0, buf.Len())
	require.Empty(t, buf.Back())
	requireZeroInternalWindow(t, buf)
	require.Empty(t, buf.Tail())
}

func TestPopBackN(t *testing.T) {
	all := []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j'}
	buf := tailbuf.New[rune](10)
	buf.WriteAll(all...)
	require.Equal(t, 10, buf.Len())
	require.Equal(t, 10, buf.Written())
	require.Equal(t, all, buf.Tail())

	got := buf.PopBackN(0)
	require.Empty(t, got)
	require.Equal(t, 10, buf.Len())
	require.Equal(t, 10, buf.Written())
	require.Equal(t, all, buf.Tail())

	got = buf.PopBackN(1)
	require.Equal(t, []rune{'a'}, got)
	require.Equal(t, 9, buf.Len())
	require.Equal(t, 10, buf.Written())
	window := tailbuf.InternalWindow(buf)
	require.Equal(t, []rune{0, 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j'}, window)
	gotTail := buf.Tail()
	require.Equal(t, all[1:], gotTail)

	got = buf.PopBackN(3)
	require.Equal(t, []rune{'b', 'c', 'd'}, got)
	require.Equal(t, 6, buf.Len())
	require.Equal(t, 10, buf.Written())
	gotTail = buf.Tail()
	require.Equal(t, all[4:], gotTail)

	got = buf.PopBackN(10)
	require.Equal(t, []rune{'e', 'f', 'g', 'h', 'i', 'j'}, got)
	require.Equal(t, 0, buf.Len())
	require.Equal(t, 10, buf.Written())
	require.Empty(t, buf.Tail())
}

func TestPopFrontN(t *testing.T) {
	all := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	buf := tailbuf.New[string](10)
	buf.WriteAll(all...)
	require.Equal(t, 10, buf.Len())
	require.Equal(t, 10, buf.Written())
	require.Equal(t, all, buf.Tail())

	got := buf.PopFrontN(0)
	require.Empty(t, got)
	require.Equal(t, 10, buf.Len())
	require.Equal(t, 10, buf.Written())
	require.Equal(t, all, buf.Tail())

	got = buf.PopFrontN(1)
	require.Equal(t, []string{"j"}, got)
	require.Equal(t, 9, buf.Len())
	require.Equal(t, 10, buf.Written())
	window := tailbuf.InternalWindow(buf)
	require.Equal(t, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", ""}, window)
	gotTail := buf.Tail()
	require.Equal(t, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}, gotTail)

	got = buf.PopFrontN(2)
	require.Equal(t, []string{"h", "i"}, got)
	require.Equal(t, 7, buf.Len())
	require.Equal(t, 10, buf.Written())
	gotTail = buf.Tail()
	require.Equal(t, []string{"a", "b", "c", "d", "e", "f", "g"}, gotTail)

	got = buf.PopFrontN(10)
	require.Equal(t, []string{"a", "b", "c", "d", "e", "f", "g"}, got)
	require.Equal(t, 0, buf.Len())
	require.Equal(t, 10, buf.Written())
	gotTail = buf.Tail()
	require.Empty(t, gotTail)
}

func TestLen(t *testing.T) {
	all := []string{"a", "b", "c"}
	buf := tailbuf.New[string](3)
	require.Equal(t, 0, buf.Len())
	buf.Write("a")
	require.Equal(t, 1, buf.Len())
	buf.Write("b")
	require.Equal(t, 2, buf.Len())
	buf.Write("c")
	require.Equal(t, 3, buf.Len())
	buf.Clear()
	require.Equal(t, 0, buf.Len())
	buf.WriteAll(all...)
	require.Equal(t, 3, buf.Len())
}

func TestDropBackN(t *testing.T) {
	all := []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j'}
	buf := tailbuf.New[rune](10)
	buf.WriteAll(all...)
	require.Equal(t, 10, buf.Len())
	require.Equal(t, 10, buf.Written())
	require.Equal(t, all, buf.Tail())

	buf.DropBackN(0)
	require.Equal(t, 10, buf.Len())
	require.Equal(t, 10, buf.Written())
	require.Equal(t, all, buf.Tail())

	buf.DropBackN(1)
	require.Equal(t, 9, buf.Len())
	require.Equal(t, 10, buf.Written())
	window := tailbuf.InternalWindow(buf)
	require.Equal(t, []rune{0, 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j'}, window)
	gotTail := buf.Tail()
	require.Equal(t, all[1:], gotTail)

	buf.DropBackN(3)
	require.Equal(t, 6, buf.Len())
	require.Equal(t, 10, buf.Written())
	gotTail = buf.Tail()
	require.Equal(t, all[4:], gotTail)

	buf.DropBackN(10)
	require.Equal(t, 0, buf.Len())
	require.Equal(t, 10, buf.Written())
	require.Empty(t, buf.Tail())
}

func TestPopBack_PopBackN_Equivalence(t *testing.T) {
	all := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	buf1 := tailbuf.New[string](10)
	buf2 := tailbuf.New[string](10)

	tailbuf.RequireEqualInternalState(t, buf1, buf2)

	buf1.WriteAll(all...)
	buf2.WriteAll(all...)

	tailbuf.RequireEqualInternalState(t, buf1, buf2)
	tail1 := buf1.Tail()
	tail2 := buf2.Tail()

	require.Equal(t, tail1, tail2)

	buf1.PopBackN(5)
	for i := 0; i < 5; i++ {
		buf2.PopBack()
	}

	tailbuf.RequireEqualInternalState(t, buf1, buf2)
	require.Equal(t, tail1, tail2)

	require.Equal(t, buf1.Tail(), buf2.Tail())
}

func requireZeroInternalWindow[T any](tb testing.TB, buf *tailbuf.Buf[T]) {
	tb.Helper()
	window := tailbuf.InternalWindow(buf)
	for i := range window {
		require.Zero(tb, window[i])
	}
}

package tailbuf_test

import (
	"strings"
	"testing"

	"github.com/neilotoole/sq/libsq/core/tailbuf"
	"github.com/neilotoole/sq/testh/tu"
	"github.com/stretchr/testify/require"
)

func TestBuf(t *testing.T) {
	testCases := []struct {
		add                string
		wantStart, wantEnd int
		wantWindow         []string
	}{
		{add: "a", wantStart: 0, wantEnd: 1, wantWindow: []string{"a"}},
		{add: "b", wantStart: 0, wantEnd: 2, wantWindow: []string{"a", "b"}},
		{add: "c", wantStart: 0, wantEnd: 3, wantWindow: []string{"a", "b", "c"}},
		{add: "d", wantStart: 1, wantEnd: 4, wantWindow: []string{"b", "c", "d"}},
		{add: "e", wantStart: 2, wantEnd: 5, wantWindow: []string{"c", "d", "e"}},
		{add: "f", wantStart: 3, wantEnd: 6, wantWindow: []string{"d", "e", "f"}},
		{add: "g", wantStart: 4, wantEnd: 7, wantWindow: []string{"e", "f", "g"}},
		{add: "h", wantStart: 5, wantEnd: 8, wantWindow: []string{"f", "g", "h"}},
	}

	buf := tailbuf.New[string](3)

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.add), func(t *testing.T) {
			buf.Write(tc.add)
			require.Equal(t, tc.wantEnd, buf.Count())
			require.Equal(t, tc.add, buf.Front())
			window := buf.Tail()
			require.Equal(t, tc.wantWindow, window)
			start, end := buf.Bounds()
			require.Equal(t, tc.wantStart, start)
			require.Equal(t, tc.wantEnd, end)
			s := buf.Slice(start, end+1)
			require.Equal(t, window, s)
		})
	}
}

func TestBuf_Bounds(t *testing.T) {
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

func TestBuf_Slice(t *testing.T) {
	buf := tailbuf.New[int](3)
	buf.WriteAll(0, 1, 2)

	start, end := buf.Bounds()
	require.Equal(t, 0, start)
	require.Equal(t, 3, end)
	s := buf.Slice(start, end)
	require.Equal(t, []int{0, 1, 2}, s)

	s = buf.Slice(0, 0)
	require.Empty(t, s)

	s = buf.Slice(0, 1)
	require.Equal(t, []int{0}, s)
	s = buf.Slice(0, 2)
	require.Equal(t, []int{0, 1}, s)
	s = buf.Slice(0, 3)
	require.Equal(t, []int{0, 1, 2}, s)

	s = buf.Slice(1, 1)
	require.Empty(t, s)
	s = buf.Slice(1, 3)
	require.Equal(t, []int{1, 2}, s)

	buf.WriteAll(3, 4, 5)
	start, end = buf.Bounds()
	require.Equal(t, 3, start)
	require.Equal(t, 6, end)
	s = buf.Slice(start, end)
	require.Equal(t, []int{3, 4, 5}, s)

	s = buf.Slice(3, 3)
	require.Empty(t, s)
	s = buf.Slice(3, 4)
	require.Equal(t, []int{3}, s)
	s = buf.Slice(3, 5)
	require.Equal(t, []int{3, 4}, s)

	buf.WriteAll(6, 7)
	s = buf.Slice(6, 7)
	require.Equal(t, []int{6}, s)
}

func TestBuf_Apply(t *testing.T) {
	buf := tailbuf.New[string](3)
	buf.WriteAll("In", "Xanadu  ", "   did", "Kubla  ", "Khan")
	buf.Apply(strings.ToUpper).Apply(strings.TrimSpace)
	got := buf.Tail()
	require.Equal(t, []string{"DID", "KUBLA", "KHAN"}, got)
}

func TestBuf_TailSlice(t *testing.T) {
	buf := tailbuf.New[int](10).WriteAll(1, 2, 3, 4, 5)
	a := buf.Tail()[0:2]
	b := buf.TailSlice(0, 2)
	require.Equal(t, []int{1, 2}, b)
	require.Equal(t, a, b)
}

func TestBuf_Tail_Slice_Equivalence(t *testing.T) {
	buf := tailbuf.New[int](10).WriteAll(1, 2, 3, 4, 5)
	a := buf.Tail()[0:2]
	b := buf.Slice(0, 2)
	require.Equal(t, []int{1, 2}, b)
	require.Equal(t, a, b)
}

func TestBuf_ZeroCapacity(t *testing.T) {
	buf := tailbuf.New[int](0)
	require.Equal(t, 0, buf.Capacity())
	buf.Write(1)

	require.Equal(t, 0, buf.Count())
	require.Empty(t, buf.Tail())
	require.Empty(t, buf.Slice(0, 1))
}

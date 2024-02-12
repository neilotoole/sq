package tailbuf_test

import (
	"github.com/neilotoole/sq/libsq/core/tailbuf"
	"github.com/neilotoole/sq/testh/tu"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBuf(t *testing.T) {
	testCases := []struct {
		add        string
		start, end int
		window     []string
	}{
		{add: "a", start: 0, end: 1, window: []string{"a"}},
		{add: "b", start: 0, end: 2, window: []string{"a", "b"}},
		{add: "c", start: 0, end: 3, window: []string{"a", "b", "c"}},
		{add: "d", start: 1, end: 4, window: []string{"b", "c", "d"}},
		{add: "e", start: 2, end: 5, window: []string{"c", "d", "e"}},
		{add: "f", start: 3, end: 6, window: []string{"d", "e", "f"}},
		{add: "g", start: 4, end: 7, window: []string{"e", "f", "g"}},
		{add: "h", start: 5, end: 8, window: []string{"f", "g", "h"}},
	}

	buf := tailbuf.New[string](3)

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.add), func(t *testing.T) {
			buf.Write(tc.add)
			require.Equal(t, tc.end, buf.Count())
			require.Equal(t, tc.add, buf.Front())
			window := buf.Window()
			require.Equal(t, tc.window, window)
			start, end := buf.Range()
			require.Equal(t, tc.start, start)
			require.Equal(t, tc.end, end)
			s := buf.Slice(start, end)
			require.Equal(t, window, s)
		})
	}
}

func TestBuf_Slice(t *testing.T) {
	buf := tailbuf.New[int](3)
	buf.WriteAll(0, 1, 2)

	start, end := buf.Range()
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
	start, end = buf.Range()
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

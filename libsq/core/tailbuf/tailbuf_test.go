package tailbuf_test

import (
	"fmt"
	"github.com/neilotoole/sq/libsq/core/tailbuf"
	"github.com/neilotoole/sq/testh/tu"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
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
			window := buf.Window()
			require.Equal(t, tc.wantWindow, window)
			start, end := buf.Bounds()
			require.Equal(t, tc.wantStart, start)
			require.Equal(t, tc.wantEnd, end)
			s := buf.NominalSlice(start, end+1)
			require.Equal(t, window, s)
		})
	}
}

func TestX4e(t *testing.T) {
	buf := tailbuf.New[string](3)
	buf.WriteAll("a", "b", "c", "d", "e")
	window := buf.Window()
	t.Log("window:", window)
	start, end := buf.Bounds()
	t.Logf("start: %d, end: %d", start, end)
	//s := buf.NominalSlice(start, end+1)
	s := buf.TailSlice(0, 4)
	require.Equal(t, window, s)
}

func TestX3(t *testing.T) {
	buf := tailbuf.New[string](3)
	buf.WriteAll("a", "b")
	window := buf.Window()
	t.Log("window:", window)
	start, end := buf.Bounds()
	t.Logf("start: %d, end: %d", start, end)
	s := buf.NominalSlice(start, end+1)
	require.Equal(t, window, s)
}

func TestX2(t *testing.T) {
	buf := tailbuf.New[string](3)
	buf.WriteAll("a", "b", "c", "d", "e", "f")
	window := buf.Window()
	t.Log("window:", window)
	start, end := buf.Bounds()
	t.Logf("start: %d, end: %d", start, end)
	s := buf.NominalSlice(start, end+1)
	require.Equal(t, window, s)
}

func TestX(t *testing.T) {
	buf := tailbuf.New[string](3)
	buf.WriteAll("a", "b", "c", "d")
	window := buf.Window()
	t.Log("window:", window)
	start, end := buf.Bounds()
	t.Logf("start: %d, end: %d", start, end)
	s := buf.NominalSlice(start, end+1)
	require.Equal(t, window, s)
}

func TestBuf_Slice2(t *testing.T) {
	buf := tailbuf.New[string](3)
	buf.Write("a")
	window := buf.Window()
	require.Equal(t, []string{"a"}, window)
	start, end := buf.Bounds()
	require.Equal(t, 0, start)
	require.Equal(t, 1, end)
	s := buf.NominalSlice(0, 0)
	require.Empty(t, s)
	s = buf.NominalSlice(0, 1)
	require.Empty(t, s)
	s = buf.NominalSlice(0, 2)
	require.Equal(t, []string{"a"}, s)

}

func TestBuf_NominalSlice(t *testing.T) {
	buf := tailbuf.New[int](3)
	buf.WriteAll(0, 1, 2)

	start, end := buf.Bounds()
	require.Equal(t, 0, start)
	require.Equal(t, 3, end)
	s := buf.NominalSlice(start, end)
	require.Equal(t, []int{0, 1, 2}, s)

	s = buf.NominalSlice(0, 0)
	require.Empty(t, s)

	s = buf.NominalSlice(0, 1)
	require.Equal(t, []int{0}, s)
	s = buf.NominalSlice(0, 2)
	require.Equal(t, []int{0, 1}, s)
	s = buf.NominalSlice(0, 3)
	require.Equal(t, []int{0, 1, 2}, s)

	s = buf.NominalSlice(1, 1)
	require.Empty(t, s)
	s = buf.NominalSlice(1, 3)
	require.Equal(t, []int{1, 2}, s)

	buf.WriteAll(3, 4, 5)
	start, end = buf.Bounds()
	require.Equal(t, 3, start)
	require.Equal(t, 6, end)
	s = buf.NominalSlice(start, end)
	require.Equal(t, []int{3, 4, 5}, s)

	s = buf.NominalSlice(3, 3)
	require.Empty(t, s)
	s = buf.NominalSlice(3, 4)
	require.Equal(t, []int{3}, s)
	s = buf.NominalSlice(3, 5)
	require.Equal(t, []int{3, 4}, s)

	buf.WriteAll(6, 7)
	s = buf.NominalSlice(6, 7)
	require.Equal(t, []int{6}, s)
}

func TestX4(t *testing.T) {
	buf := tailbuf.New[int](3)
	buf.WriteAll(0, 1, 2, 3, 4, 5, 6, 7)
	s := buf.NominalSlice(6, 7)
	require.Equal(t, []int{6}, s)
}

func TestBuf_Apply(t *testing.T) {
	buf := tailbuf.New[string](3)
	buf.WriteAll("In", "Xanadu  ", "   did", "Kubla  ", "Khan")
	buf.Apply(strings.ToUpper).Apply(strings.TrimSpace)
	got := buf.Window()
	require.Equal(t, []string{"DID", "KUBLA", "KHAN"}, got)
}

func TestBuf_Apply2(t *testing.T) {
	buf := tailbuf.New[string](3)
	buf.WriteAll("a", "b  ", "   c  ")
	buf.Apply(strings.ToUpper).Apply(strings.TrimSpace)
	fmt.Println(buf.Window())
	//t.Log(got)
	//require.Equal(t, []string{"A", "B", "C"}, got)
}

func TestTailSlice(t *testing.T) {
	buf := tailbuf.New[int](10).WriteAll(1, 2, 3, 4, 5)
	a := buf.Window()[0:2]
	b := buf.TailSlice(0, 2)
	require.Equal(t, []int{1, 2}, b)
	require.Equal(t, a, b)
}

func Test_Window_Slice_Equivalence(t *testing.T) {
	buf := tailbuf.New[int](10).WriteAll(1, 2, 3, 4, 5)
	a := buf.Window()[0:2]
	b := buf.NominalSlice(0, 2)
	require.Equal(t, []int{1, 2}, b)
	require.Equal(t, a, b)
}

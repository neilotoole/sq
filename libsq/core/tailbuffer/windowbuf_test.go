package tailbuffer_test

import (
	"github.com/neilotoole/sq/libsq/core/tailbuffer"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBuffer(t *testing.T) {
	buf := tailbuffer.New[int](3)
	buf.Write(0).Write(1).Write(2)
	require.Equal(t, []int{0, 1, 2}, buf.Window())
	require.Equal(t, 2, buf.Front())

	a := buf.Window()
	t.Log(a)

	buf.Write(3)
	buf.Write(4)
	buf.Write(5)
	a = buf.Window()
	t.Log(a)
}

func TestRange(t *testing.T) {
	buf := tailbuffer.New[string](3)
	window := buf.Window()
	require.Equal(t, []string{}, window)
	start, end := buf.Range()
	t.Log(start, end)
	require.Equal(t, 0, start)
	require.Equal(t, 0, end)

	// Add an item
	buf.Write("a")
	window = buf.Window()
	require.Equal(t, []string{"a"}, window)
	start, end = buf.Range()
	//r := window[start:end]
	//t.Log(start, end, r)
	require.Equal(t, 0, start)
	require.Equal(t, 1, end)

	// Add another item
	buf.Write("b")
	window = buf.Window()
	require.Equal(t, []string{"a", "b"}, window)
	start, end = buf.Range()
	//r = window[start:end]

	//t.Log(start, end, r)
	require.Equal(t, 0, start)
	require.Equal(t, 2, end)

	buf.Write("c")
	window = buf.Window()
	require.Equal(t, []string{"a", "b", "c"}, window)
	start, end = buf.Range()
	//r = window[start:end]
	//t.Log(start, end, r)
	require.Equal(t, 0, start)
	require.Equal(t, 3, end)

}

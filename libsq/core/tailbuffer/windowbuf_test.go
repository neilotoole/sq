package tailbuffer_test

import (
	"github.com/neilotoole/sq/libsq/core/tailbuffer"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBuffer(t *testing.T) {
	buf := tailbuffer.New[int](3)
	buf.Append(0).Append(1).Append(2)
	require.Equal(t, []int{0, 1, 2}, buf.Window())
	require.Equal(t, 2, buf.Front())

	a := buf.Window()
	t.Log(a)

	buf.Append(3)
	buf.Append(4)
	buf.Append(5)
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
	buf.Append("a")
	window = buf.Window()
	require.Equal(t, []string{"a"}, window)
	start, end = buf.Range()
	t.Log(start, end)
	require.Equal(t, 0, start)
	require.Equal(t, 1, end)

	// Add two more items
	buf.Append("b")
	buf.Append("c")
	window = buf.Window()
	require.Equal(t, []string{"a", "b", "c"}, window)
	start, end = buf.Range()
	t.Log(start, end)
	require.Equal(t, 0, start)
	require.Equal(t, 2, end)

}

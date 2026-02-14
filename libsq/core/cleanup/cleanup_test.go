package cleanup_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func TestCleanup(t *testing.T) {
	want := []int{4, 3, 2, 1, 0}
	var got []int

	clnup := cleanup.New()

	for i := range 5 {
		i := i
		clnup.AddE(func() error {
			got = append(got, i)
			return nil
		})
	}

	err := clnup.Run()
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestCleanup_Error(t *testing.T) {
	clnup := cleanup.New()

	clnup.AddE(func() error {
		return nil
	})

	clnup.AddE(func() error {
		return errz.New("err1")
	})

	clnup.AddE(func() error {
		return errz.New("err2")
	})

	err := clnup.Run()
	require.Error(t, err)

	require.Equal(t, "err2; err1", err.Error())
	errs := errz.Errors(err)
	require.Equal(t, 2, len(errs))
	require.Equal(t, "err2", errs[0].Error())
	require.Equal(t, "err1", errs[1].Error())
}

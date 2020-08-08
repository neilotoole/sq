package fetcher_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/lg/testlg"

	"github.com/neilotoole/sq/libsq/source/fetcher"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestFetchFile(t *testing.T) {
	file, _, cleanup, err := fetcher.FetchFile(testlg.New(t), sakila.URLSubsetXLSX)
	if cleanup != nil {
		defer func() {
			require.Nil(t, cleanup())
		}()
	}
	require.Nil(t, err)
	require.NotNil(t, file)
	require.Nil(t, file.Close())
}

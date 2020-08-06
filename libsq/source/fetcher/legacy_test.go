package fetcher_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/lg/testlg"

	"github.com/neilotoole/sq/libsq/source/fetcher"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestFetchFile(t *testing.T) {
	file, mediatype, cleanup, err := fetcher.FetchFile(testlg.New(t), sakila.URLSubsetXLSX)
	if cleanup != nil {
		defer require.Nil(t, cleanup())
	}
	require.Nil(t, err)
	require.NotNil(t, file)
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", mediatype)
	require.Nil(t, file.Close())
}

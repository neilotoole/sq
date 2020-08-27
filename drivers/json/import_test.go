package json_test

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/json"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/testsrc"
)

func TestImportJSONL(t *testing.T) {
	openFn := func() (io.ReadCloser, error) {
		return os.Open("testdata/jsonl_actor_nested.jsonl")
	}

	th, src, dbase, _ := testh.NewWith(t, testsrc.EmptyDB)
	err := json.ImportJSONL(th.Context, th.Log, src, openFn, dbase)
	require.NoError(t, err)

	sink, err := th.QuerySQL(src, "SELECT * FROM data")
	require.NoError(t, err)
	require.Equal(t, 4, len(sink.Recs))
}

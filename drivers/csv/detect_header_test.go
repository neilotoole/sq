package csv

import (
	"bytes"
	"encoding/csv"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/tu"
)

func Test_detectHeaderRow(t *testing.T) {
	testCases := []struct {
		fp    string
		comma rune // zero value uses comma
		want  bool
	}{
		{fp: "testdata/sakila-csv/actor.csv", want: true},
		{fp: "testdata/sakila-csv-noheader/actor.csv", want: false},
	}

	for i, tc := range testCases {
		tc := tc

		t.Run(tu.Name(i, tc.fp), func(t *testing.T) {
			recs := readAllRecs(t, tc.comma, tc.fp)

			gotHasHeader, err := detectHeaderRow(recs)
			require.NoError(t, err)
			require.Equal(t, tc.want, gotHasHeader)
		})
	}
}

func readAllRecs(t *testing.T, comma rune, fp string) [][]string {
	b, err := os.ReadFile(fp)
	require.NoError(t, err)

	cr := csv.NewReader(&crFilterReader{r: bytes.NewReader(b)})
	if comma != rune(0) {
		cr.Comma = comma
	}

	recs, err := cr.ReadAll()
	require.NoError(t, err)
	return recs
}

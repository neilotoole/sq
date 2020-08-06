package xlsx

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tealeg/xlsx/v2"
)

func Test_getRowsMaxCellCount(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"test_header.xlsx",
		"test_noheader.xlsx",
		"problem_with_recognizing_date_colA.xlsx",
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			xlFile, err := xlsx.OpenFile(filepath.Join("testdata", tc))
			require.NoError(t, err)
			require.NotNil(t, xlFile)

			for _, sheet := range xlFile.Sheets {
				cellCount := getRowsMaxCellCount(sheet)
				t.Logf("sheet %s: cell count: %d", sheet.Name, cellCount)
				require.True(t, cellCount > 0)
			}
		})
	}
}

package xlsx_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

func TestDetectXLSX(t *testing.T) {
	testCases := []struct {
		name      string
		file      string
		wantType  drivertype.Type
		wantScore float32
	}{
		{
			name:      "actor_header",
			file:      "testdata/actor_header.xlsx",
			wantType:  drivertype.XLSX,
			wantScore: 1.0,
		},
		{
			name:      "actor_no_header",
			file:      "testdata/actor_no_header.xlsx",
			wantType:  drivertype.XLSX,
			wantScore: 1.0,
		},
		{
			name:      "sakila",
			file:      "testdata/sakila.xlsx",
			wantType:  drivertype.XLSX,
			wantScore: 1.0,
		},
		{
			name:      "sakila_subset",
			file:      "testdata/sakila_subset.xlsx",
			wantType:  drivertype.XLSX,
			wantScore: 1.0,
		},
		{
			name:      "datetime",
			file:      "testdata/datetime.xlsx",
			wantType:  drivertype.XLSX,
			wantScore: 1.0,
		},
		{
			name:      "some_sheets_empty",
			file:      "testdata/some_sheets_empty.xlsx",
			wantType:  drivertype.XLSX,
			wantScore: 1.0,
		},
		{
			name:      "strict_openxml",
			file:      "testdata/file_formats/sakila.strict_openxml.xlsx",
			wantType:  drivertype.XLSX,
			wantScore: 1.0,
		},
		{
			name:      "invalid_xlsx",
			file:      "testdata/test_invalid.xlsx",
			wantType:  drivertype.None,
			wantScore: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fpath := tc.file
			if !filepath.IsAbs(fpath) {
				// Handle relative paths for test execution from different dirs
				if _, err := os.Stat(fpath); os.IsNotExist(err) {
					// Try from parent directory
					fpath = filepath.Join("..", "..", "drivers", "xlsx", tc.file)
				}
			}

			newRdrFn := func(_ context.Context) (io.ReadCloser, error) {
				return os.Open(fpath)
			}

			gotType, gotScore, err := xlsx.DetectXLSX(context.Background(), newRdrFn)
			require.NoError(t, err)
			require.Equal(t, tc.wantType, gotType, "type mismatch for %s", tc.file)
			require.Equal(t, tc.wantScore, gotScore, "score mismatch for %s", tc.file)
		})
	}
}

func TestDetectXLSX_NonXLSX(t *testing.T) {
	testCases := []struct {
		name    string
		content []byte
	}{
		{
			name:    "empty",
			content: []byte{},
		},
		{
			name:    "too_short",
			content: []byte{0x50, 0x4B},
		},
		{
			name:    "csv_content",
			content: []byte("a,b,c\n1,2,3\n4,5,6"),
		},
		{
			name:    "json_content",
			content: []byte(`{"foo": "bar"}`),
		},
		{
			name: "regular_zip",
			// A minimal valid ZIP file (empty archive)
			content: []byte{
				0x50, 0x4B, 0x05, 0x06, // End of central directory signature
				0x00, 0x00, // Number of this disk
				0x00, 0x00, // Disk where central directory starts
				0x00, 0x00, // Number of central directory records on this disk
				0x00, 0x00, // Total number of central directory records
				0x00, 0x00, 0x00, 0x00, // Size of central directory
				0x00, 0x00, 0x00, 0x00, // Offset of start of central directory
				0x00, 0x00, // Comment length
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			newRdrFn := func(_ context.Context) (io.ReadCloser, error) {
				return io.NopCloser(
					&bytesReader{data: tc.content},
				), nil
			}

			gotType, gotScore, err := xlsx.DetectXLSX(context.Background(), newRdrFn)
			require.NoError(t, err)
			require.Equal(t, drivertype.None, gotType, "expected None type for %s", tc.name)
			require.Equal(t, float32(0), gotScore, "expected 0 score for %s", tc.name)
		})
	}
}

// bytesReader is a simple io.Reader wrapper for byte slices.
type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

package xlsx_test

import (
	"bytes"
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
			name: "regular_zip_empty",
			// A minimal valid ZIP file (empty archive) - starts with end-of-central-dir
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
		{
			name: "zip_with_other_content",
			// A ZIP with a local file header but no xl/ entry (like a text file)
			content: func() []byte {
				// ZIP local file header for "readme.txt"
				header := []byte{
					0x50, 0x4B, 0x03, 0x04, // Local file header signature
					0x14, 0x00, // Version needed (2.0)
					0x00, 0x00, // General purpose bit flag
					0x00, 0x00, // Compression method (stored)
					0x00, 0x00, // Last mod file time
					0x00, 0x00, // Last mod file date
					0x00, 0x00, 0x00, 0x00, // CRC-32
					0x05, 0x00, 0x00, 0x00, // Compressed size (5)
					0x05, 0x00, 0x00, 0x00, // Uncompressed size (5)
					0x0A, 0x00, // Filename length (10)
					0x00, 0x00, // Extra field length (0)
				}
				filename := []byte("readme.txt")
				content := []byte("hello")
				return append(append(header, filename...), content...)
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			newRdrFn := func(_ context.Context) (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(tc.content)), nil
			}

			gotType, gotScore, err := xlsx.DetectXLSX(context.Background(), newRdrFn)
			require.NoError(t, err)
			require.Equal(t, drivertype.None, gotType, "expected None type for %s", tc.name)
			require.Equal(t, float32(0), gotScore, "expected 0 score for %s", tc.name)
		})
	}
}

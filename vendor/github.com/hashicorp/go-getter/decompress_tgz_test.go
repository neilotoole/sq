package getter

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestTarGzipDecompressor(t *testing.T) {

	multiplePaths := []string{"dir/", "dir/test2", "test1"}
	if runtime.GOOS == "windows" {
		multiplePaths = []string{"dir/", "dir\\test2", "test1"}
	}

	cases := []TestDecompressCase{
		{
			"empty.tar.gz",
			false,
			true,
			nil,
			"",
		},

		{
			"single.tar.gz",
			false,
			false,
			nil,
			"d3b07384d113edec49eaa6238ad5ff00",
		},

		{
			"single.tar.gz",
			true,
			false,
			[]string{"file"},
			"",
		},

		{
			"multiple.tar.gz",
			true,
			false,
			[]string{"file1", "file2"},
			"",
		},

		{
			"multiple.tar.gz",
			false,
			true,
			nil,
			"",
		},

		{
			"multiple_dir.tar.gz",
			true,
			false,
			multiplePaths,
			"",
		},
	}

	for i, tc := range cases {
		cases[i].Input = filepath.Join("./test-fixtures", "decompress-tgz", tc.Input)
	}

	TestDecompressor(t, new(TarGzipDecompressor), cases)
}

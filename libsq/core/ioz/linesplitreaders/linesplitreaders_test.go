package linesplitreaders_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/neilotoole/sq/libsq/core/ioz/linesplitreaders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testCases = []struct {
	name         string
	in           string
	wantLines    []string
	wantRdrCount int
}{
	{
		name:         "empty",
		in:           "",
		wantLines:    nil,
		wantRdrCount: 1,
	},
	{
		name:         "oneline-lf",
		in:           "a\n",
		wantLines:    []string{"a"},
		wantRdrCount: 2,
	},
	{
		name:         "empty-1-lf",
		in:           "\n",
		wantLines:    nil,
		wantRdrCount: 2,
	},
	{
		name:         "empty-1-crlf",
		in:           "\r\n",
		wantLines:    nil,
		wantRdrCount: 2,
	},
	{
		name:         "empty-2-crlf",
		in:           "\r\n\r\n",
		wantLines:    nil,
		wantRdrCount: 3,
	},
	{
		name:         "empty-2-lf",
		in:           "\n\n",
		wantLines:    nil,
		wantRdrCount: 3,
	},
	{
		name:         "oneline-crlf",
		in:           "line1\r\n",
		wantLines:    []string{"line1"},
		wantRdrCount: 2,
	},
	{
		name:         "oneline-no-lf",
		in:           "line1",
		wantLines:    []string{"line1"},
		wantRdrCount: 1,
	},
	{
		name:         "content-2-lf",
		in:           "line1\nline2\n",
		wantLines:    []string{"line1", "line2"},
		wantRdrCount: 3,
	},
	{
		name:         "content-4-no-trailing-lf",
		in:           "line1\nline2\nline3\nline4",
		wantLines:    []string{"line1", "line2", "line3", "line4"},
		wantRdrCount: 4,
	},
	{
		name:         "single-char-4-lf",
		in:           "a\nb\nc\nd",
		wantLines:    []string{"a", "b", "c", "d"},
		wantRdrCount: 4,
	},
	{
		name:         "single-char-4-cr",
		in:           "a\rb\rc\rd",
		wantLines:    []string{"a", "b", "c", "d"},
		wantRdrCount: 4,
	},
	{
		name:         "multi-lines-with-extra-lf",
		in:           "\nline2\nline3\nline4\n\nline5",
		wantLines:    []string{"line2", "line3", "line4", "line5"},
		wantRdrCount: 6,
	},
}

func Test_ReadAll(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var rdrCount int

			sc := linesplitreaders.New(strings.NewReader(tc.in))
			var lines []string
			for sc.Next() {
				r := sc.Reader()
				require.NotNil(t, r)
				rdrCount++

				line, err := io.ReadAll(r)
				assert.NoError(t, err)

				if len(line) > 0 {
					lines = append(lines, string(line))
				}
			}
			require.Equal(t, tc.wantLines, lines)
			require.Equal(t, tc.wantRdrCount, rdrCount)
		})
	}
}

func Test_Read_Loop(t *testing.T) {
	testCases := []struct {
		name         string
		in           string
		wantLines    []string
		wantRdrCount int
	}{
		{
			name:         "empty",
			in:           "",
			wantLines:    nil,
			wantRdrCount: 1,
		},
		{
			name:         "a-lf",
			in:           "a\n",
			wantLines:    []string{"a"},
			wantRdrCount: 1,
		},
		{
			name:         "empty-1-lf",
			in:           "\n",
			wantLines:    nil,
			wantRdrCount: 1,
		},
		//{
		//	name:         "empty-1-crlf",
		//	in:           "\r\n",
		//	wantLines:    nil,
		//	wantRdrCount: 2,
		//},
		//{
		//	name:         "empty-2-crlf",
		//	in:           "\r\n\r\n",
		//	wantLines:    nil,
		//	wantRdrCount: 3,
		//},
		{
			name:         "empty-2-lf",
			in:           "\n\n",
			wantLines:    nil,
			wantRdrCount: 2,
		},
		//{
		//	name:         "oneline-crlf",
		//	in:           "line1\r\n",
		//	wantLines:    []string{"line1"},
		//	wantRdrCount: 2,
		//},
		{
			name:         "oneline-no-lf",
			in:           "line1",
			wantLines:    []string{"line1"},
			wantRdrCount: 1,
		},
		{
			name:         "content-2-lf",
			in:           "line1\nline2\n",
			wantLines:    []string{"line1", "line2"},
			wantRdrCount: 2,
		},
		{
			name:         "ab-2-lf",
			in:           "ab\ncd\n",
			wantLines:    []string{"ab", "cd"},
			wantRdrCount: 2,
		},
		{
			name:         "content-4-no-trailing-lf",
			in:           "line1\nline2\nline3\nline4",
			wantLines:    []string{"line1", "line2", "line3", "line4"},
			wantRdrCount: 4,
		},
		{
			name:         "single-char-4-lf",
			in:           "a\nb\nc\nd",
			wantLines:    []string{"a", "b", "c", "d"},
			wantRdrCount: 4,
		},
		//{
		//	name:         "single-char-4-cr",
		//	in:           "a\rb\rc\rd",
		//	wantLines:    []string{"a", "b", "c", "d"},
		//	wantRdrCount: 4,
		//},
		{
			name:         "multi-lines-with-extra-lf",
			in:           "\nline2\nline3\nline4\n\nline5",
			wantLines:    []string{"line2", "line3", "line4", "line5"},
			wantRdrCount: 6,
		},
	}

	const bufMin, bufMax = 1, 16

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for bufSize := bufMin; bufSize <= bufMax; bufSize++ { // FIXME: test bufsize zero
				t.Run(fmt.Sprintf("buf-%d", bufSize), func(t *testing.T) {
					var rdrCount = 0
					splitter := linesplitreaders.New(strings.NewReader(tc.in))
					var lines []string

					for splitter.Next() {
						r := splitter.Reader()
						require.NotNil(t, r)
						rdrCount++

						var line []byte

						p := make([]byte, bufSize)
						var n int
						var err error

						for {
							n, err = r.Read(p)

							if n > 0 {
								line = append(line, p[:n]...)
							}

							lineStr := string(line)
							_ = lineStr

							if err != nil {
								assert.True(t, errors.Is(err, io.EOF))
								break
							}
						}

						if len(line) > 0 {
							lines = append(lines, string(line))
							line = nil
						}
					}

					require.Equal(t, tc.wantLines, lines)
					require.Equal(t, tc.wantRdrCount, rdrCount)
				})
			}
		})
	}
}

func TestCRLF_one_byte_buf10(t *testing.T) {
	const input = "\r\n"
	const bufSize = 10

	splitter := linesplitreaders.New(strings.NewReader(input))

	require.True(t, splitter.Next())
	buf := make([]byte, bufSize)
	r := splitter.Reader()
	require.NotNil(t, r)
	n, err := r.Read(buf)
	require.Equal(t, 0, n)
	require.NoError(t, err)

	n, err = r.Read(buf)
	require.Equal(t, 0, n)
	require.True(t, errors.Is(err, io.EOF))

	require.False(t, splitter.Next())
	r = splitter.Reader()
	require.Nil(t, r)
}

func TestCRLF_one_byte_buf2(t *testing.T) {
	const input = "\r\n"
	const bufSize = 5

	splitter := linesplitreaders.New(strings.NewReader(input))

	require.True(t, splitter.Next())
	buf := make([]byte, bufSize)
	r := splitter.Reader()
	require.NotNil(t, r)
	n, err := r.Read(buf)
	require.Equal(t, 0, n)
	require.NoError(t, err)

	n, err = r.Read(buf)
	require.Equal(t, 0, n)
	require.True(t, errors.Is(err, io.EOF))

	require.False(t, splitter.Next())
	r = splitter.Reader()
	require.Nil(t, r)
}

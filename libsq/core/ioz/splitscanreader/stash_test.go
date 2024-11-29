package splitscanreader_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/neilotoole/sq/libsq/core/ioz/splitscanreader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScanner_ReaderLoop_FixedBufSize(t *testing.T) {
	bufSize := 1

	testCases := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "empty",
			in:   "",
			want: nil,
		},
		{
			name: "empty-1-lf",
			in:   "\n",
			want: nil,
		},
		{
			name: "empty-2-lf",
			in:   "\n",
			want: nil,
		},
		{
			name: "oneline-lf",
			in:   "a\n",
			want: []string{"a"},
		},
		{
			name: "oneline-crlf",
			in:   "a\r\n",
			want: []string{"a"},
		},
		//{
		//	name: "oneline-crlf",
		//	in:   "line1\r\n"),
		//	want: []string{"line1"},
		//},
		{
			name: "oneline-no-newline",
			in:   "line1",
			want: []string{"line1"},
		},
		{
			name: "newline-2",
			in:   "line1\nline2\n",
			want: []string{"line1", "line2"},
		},
		{
			name: "newline-4",
			in:   "line1\nline2\nline3\nline4",
			want: []string{"line1", "line2", "line3", "line4"},
		},
		{
			name: "single-char-newline-4",
			in:   "a\nb\nc\nd",
			want: []string{"a", "b", "c", "d"},
		},
		//{
		//	name: "newlines-2",
		//	in:   strings.NewReader("\nline2\nline3\nline4\n\nline5"),
		//	want: [][]string{{"line2"}, {"line3"}, {"line4"}, {""}, {"line5"}},
		//},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// t.Parallel()

			sc := splitscanreader.NewScanner(strings.NewReader(tc.in))
			var lines []string
			for sc.Scan() {
				r := sc.Reader()

				var line []byte

				p := make([]byte, bufSize)
				var n int
				var err error

				for {
					n, err = r.Read(p)

					if n > 0 {
						line = append(line, p[:n]...)
					}

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
			require.Equal(t, tc.want, lines)
		})
	}
}

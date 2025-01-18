package linesplitreaders_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/neilotoole/sq/libsq/core/ioz"

	"github.com/neilotoole/sq/libsq/core/ioz/linesplitreaders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var splitTestCases = []struct {
	name         string
	in           string
	wantLines    []string
	wantRdrCount int
}{
	{
		name:         "empty",
		in:           "",
		wantLines:    []string{""},
		wantRdrCount: 1,
	},
	{
		name:         "a-lf",
		in:           "a\n",
		wantLines:    []string{"a", ""},
		wantRdrCount: 2,
	},
	{
		name:         "empty-1-lf",
		in:           "\n",
		wantLines:    []string{"", ""},
		wantRdrCount: 2,
	},
	{
		name:         "empty-1-cr",
		in:           "\r",
		wantLines:    []string{"\r"},
		wantRdrCount: 1,
	},
	{
		name:         "empty-1-crlf",
		in:           "\r\n",
		wantLines:    []string{"", ""},
		wantRdrCount: 2,
	},
	{
		name:         "empty-2-crlf",
		in:           "\r\n\r\n",
		wantLines:    []string{"", "", ""},
		wantRdrCount: 3,
	},
	{
		name:         "a-crlf",
		in:           "a\r\n",
		wantLines:    []string{"a", ""},
		wantRdrCount: 2,
	},
	{
		name:         "a-crlf-b-crlf",
		in:           "a\r\nb\r\n",
		wantLines:    []string{"a", "b", ""},
		wantRdrCount: 3,
	},
	{
		name:         "empty-2-lf",
		in:           "\n\n",
		wantLines:    []string{"", "", ""},
		wantRdrCount: 3,
	},
	{
		name:         "oneline-crlf",
		in:           "line1\r\n",
		wantLines:    []string{"line1", ""},
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
		wantLines:    []string{"line1", "line2", ""},
		wantRdrCount: 3,
	},
	{
		name:         "ab-2-lf",
		in:           "ab\ncd\n",
		wantLines:    []string{"ab", "cd", ""},
		wantRdrCount: 3,
	},
	{
		name:         "ab-crlf-cd-crlf",
		in:           "ab\r\ncd\r\n",
		wantLines:    []string{"ab", "cd", ""},
		wantRdrCount: 3,
	},
	{
		name:         "content-4-no-trailing-lf",
		in:           "line1\nline2\nline3\nline4",
		wantLines:    []string{"line1", "line2", "line3", "line4"},
		wantRdrCount: 4,
	},
	{
		name:         "content-4-no-trailing-crlf",
		in:           "line1\r\nline2\r\nline3\r\nline4",
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
		name:         "single-char-4-crlf",
		in:           "a\r\nb\r\nc\r\nd",
		wantLines:    []string{"a", "b", "c", "d"},
		wantRdrCount: 4,
	},
	{
		name:         "single-char-4-cr",
		in:           "a\rb\rc\rd",
		wantLines:    []string{"a\rb\rc\rd"},
		wantRdrCount: 1,
	},
	{
		name:         "multi-lines-with-extra-lf",
		in:           "\nline2\nline3\nline4\n\nline5",
		wantLines:    []string{"", "line2", "line3", "line4", "", "line5"},
		wantRdrCount: 6,
	},
	{
		name:         "multi-lines-with-extra-crlf",
		in:           "\r\nline2\r\nline3\r\nline4\r\n\r\nline5",
		wantLines:    []string{"", "line2", "line3", "line4", "", "line5"},
		wantRdrCount: 6,
	},
	{
		name:         "single-char-lines-with-extra-lf",
		in:           "\nb\nc\nd\n\nf",
		wantLines:    []string{"", "b", "c", "d", "", "f"},
		wantRdrCount: 6,
	},
	{
		name:         "single-char-lines-with-extra-crlf",
		in:           "\r\nb\r\nc\r\nd\r\n\r\nf",
		wantLines:    []string{"", "b", "c", "d", "", "f"},
		wantRdrCount: 6,
	},
	{
		name:         "single-char-lines-with-extra-lf-2",
		in:           "\nb\nc\nd\n\nf\n",
		wantLines:    []string{"", "b", "c", "d", "", "f", ""},
		wantRdrCount: 7,
	},
	{
		name:         "a-c-lines-with-extra-lf",
		in:           "a\n\nc",
		wantLines:    []string{"a", "", "c"},
		wantRdrCount: 3,
	},
	{
		name:         "a-c-lines-with-extra-crlf",
		in:           "a\r\n\r\nc",
		wantLines:    []string{"a", "", "c"},
		wantRdrCount: 3,
	},

	{
		name:         "lf-lf-c",
		in:           "\n\nc",
		wantLines:    []string{"", "", "c"},
		wantRdrCount: 3,
	},
	{
		name:         "single-char-2-cr",
		in:           "a\rb\r",
		wantLines:    []string{"a\rb\r"},
		wantRdrCount: 1,
	},
	{
		name:         "crlf-crlf-c",
		in:           "\r\n\r\nc",
		wantLines:    []string{"", "", "c"},
		wantRdrCount: 3,
	},
	{
		name:         "mixed-endings-3",
		in:           "\r\r\n\r\r\r\n",
		wantLines:    []string{"\r", "\r\r", ""},
		wantRdrCount: 3,
	},
	{
		name:         "mixed-endings-4",
		in:           "\r\r\n\n\r\r\r\n",
		wantLines:    []string{"\r", "", "\r\r", ""},
		wantRdrCount: 4,
	},
}

func TestSplitter_via_ioReadAll(t *testing.T) {
	t.Parallel()

	for _, tc := range splitTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var rdrCount int

			sc := linesplitreaders.New(strings.NewReader(tc.in))
			var lines []string
			for sc.HasMore() {
				r := sc.Reader()
				require.NotNil(t, r)
				rdrCount++

				line, err := io.ReadAll(r)
				assert.NoError(t, err)

				lines = append(lines, string(line))
			}
			require.Equal(t, tc.wantLines, lines)
			require.Equal(t, tc.wantRdrCount, rdrCount)
		})
	}
}

// TestSplitter_via_Read tests via the io.Reader returned from Splitter.Reader.
func TestSplitter_via_Read(t *testing.T) {
	t.Parallel()

	// Try different buffer sizes.
	const bufMin, bufMax = 1, 1000

	for _, tc := range splitTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for bufSize := bufMin; bufSize <= bufMax; bufSize++ {
				t.Run(fmt.Sprintf("buf-%d", bufSize), func(t *testing.T) {
					t.Parallel()

					rdrCount := 0
					splitter := linesplitreaders.New(strings.NewReader(tc.in))
					var lines []string

					for splitter.HasMore() {
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

							if err != nil {
								assert.True(t, errors.Is(err, io.EOF))
								break
							}
						}

						// We've hit EOF, so we should have a line
						lines = append(lines, string(line))
						line = nil
					}

					require.Equal(t, tc.wantLines, lines)
					require.Equal(t, tc.wantRdrCount, rdrCount)
				})
			}
		})
	}
}

func TestSplitter_Reader_PanicsWhenExistingReaderNotConsumed(t *testing.T) {
	for _, tc := range []string{"", "a\n", "a\nb\n", "a\r\n"} {
		t.Run(tc, func(t *testing.T) {
			splitter := linesplitreaders.New(strings.NewReader(tc))
			require.True(t, splitter.HasMore())
			require.True(t, splitter.HasMore())
			_ = splitter.Reader()
			require.Panics(t, func() {
				splitter.Reader()
			})
			require.True(t, splitter.HasMore())
			require.Panics(t, func() {
				splitter.Reader()
			})
			require.True(t, splitter.HasMore())
		})
	}
}

func TestSplitter_Reader_Read_ReturnsEOFSubsequently(t *testing.T) {
	for _, tc := range []string{"", "a\n", "a\n", "a\r\n"} {
		t.Run(tc, func(t *testing.T) {
			splitter := linesplitreaders.New(strings.NewReader(tc))
			rdr := splitter.Reader()
			_, err := io.ReadAll(rdr)
			require.NoError(t, err)
			for i := 0; i < 10; i++ {
				data := make([]byte, 10)
				var n int
				n, err = rdr.Read(data)
				require.Equal(t, 0, n)
				require.True(t, errors.Is(err, io.EOF))
			}
		})
	}
}

func TestSplitter_Reader_Read_ReturnsSameErrorSubsequently(t *testing.T) {
	wantErr := errors.New("test error")

	for _, tc := range []string{"", "a\n", "a\n", "a\r\n"} {
		t.Run(tc, func(t *testing.T) {
			splitter := linesplitreaders.New(&errReader{err: wantErr})
			rdr := splitter.Reader()
			_, err := io.ReadAll(rdr)
			require.Error(t, err)
			require.True(t, errors.Is(err, wantErr))
			_, err = io.ReadAll(rdr)
			require.Error(t, err)
			require.True(t, errors.Is(err, wantErr))

			require.False(t, splitter.HasMore())
		})
	}
}

func TestReadAllError(t *testing.T) {
	input := "a\nb\nc\nd\n\r"
	want := []string{"a", "b", "c", "d", "\r"}
	wantErr := errors.New("want error")

	src := ioz.NewErrorAfterBytesReader([]byte(input), wantErr)

	lines, err := linesplitreaders.ReadAll(src)
	require.Equal(t, want, lines)
	require.True(t, errors.Is(err, wantErr))
}

func TestReadAll(t *testing.T) {
	sentinelErr := errors.New("sentinel error")

	testCases := []struct {
		src       io.Reader
		wantLines []string
		wantErr   error
	}{
		{
			src:       strings.NewReader(""),
			wantLines: []string{""},
			wantErr:   nil,
		},
		{
			src:       strings.NewReader("hello"),
			wantLines: []string{"hello"},
			wantErr:   nil,
		},
		{
			src:       strings.NewReader("a\nb\nc\nd"),
			wantLines: []string{"a", "b", "c", "d"},
			wantErr:   nil,
		},
		{
			src:       ioz.NewErrorAfterBytesReader([]byte("a\nb\nc\nd"), sentinelErr),
			wantLines: []string{"a", "b", "c", "d"},
			wantErr:   sentinelErr,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			lines, err := linesplitreaders.ReadAll(tc.src)
			if tc.wantErr != nil {
				require.True(t, errors.Is(err, tc.wantErr))
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.wantLines, lines)
		})
	}
}

var _ io.Reader = (*errReader)(nil)

// errReader is an [io.Reader] that always returns an error.
type errReader struct {
	err error
}

// Read implements [io.Reader]: it always returns [errReader.Err].
func (e errReader) Read([]byte) (n int, err error) {
	return 0, e.err
}

package csv

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/tu"
)

func Test_isCSV(t *testing.T) {
	t.Parallel()

	const (
		comma = ','
		tab   = '\t'
	)

	testCases := []struct {
		delim rune
		want  float32
		input string
	}{
		{delim: comma, input: "", want: scoreNo},
		{delim: comma, input: "a", want: scoreMaybe},
		{delim: comma, input: "a,b", want: scoreMaybe},
		{delim: comma, input: "a,b\n", want: scoreMaybe},
		{delim: comma, input: "a,b\na,b", want: scoreProbably},
		{delim: comma, input: "a,b,c\na,b,c\na,b,c", want: scoreYes},
		{delim: comma, input: "a,b\na,b,c", want: scoreNo}, // Fields per record not equal
		{delim: tab, input: "", want: scoreNo},
		{delim: tab, input: "a", want: scoreMaybe},
		{delim: tab, input: "a\tb", want: scoreMaybe},
		{delim: tab, input: "a\tb\n", want: scoreMaybe},
		{delim: tab, input: "a\tb\na\tb", want: scoreProbably},
		{delim: tab, input: "a\tb\tc\na\tb\tc\na\tb\tc", want: scoreYes},
		{delim: tab, input: "a\tb\na\tb\tc", want: scoreNo}, // Fields per record not equal
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d %s", i, tc.input), func(t *testing.T) {
			t.Parallel()

			cr := csv.NewReader(&crFilterReader{r: strings.NewReader(tc.input)})
			cr.Comma = tc.delim

			got := isCSV(context.Background(), cr)
			require.Equal(t, tc.want, got)
		})
	}
}

func Test_detectColKinds(t *testing.T) {
	testCases := []struct {
		name      string
		recs      [][]string
		wantKinds []kind.Kind
		wantErr   bool
	}{
		{
			name:    "empty",
			recs:    [][]string{},
			wantErr: true,
		},
		{
			name: "basic",
			recs: [][]string{
				{"1", "true", "hello", "0.0"},
				{"2", "false", "world", "1"},
				{"3", "true", "", "7.7"},
				{"", "", "", ""},
			},
			wantKinds: []kind.Kind{kind.Int, kind.Bool, kind.Text, kind.Decimal},
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.name), func(t *testing.T) {
			gotKinds, _, gotErr := detectColKinds(tc.recs)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, stringz.Strings(tc.wantKinds), stringz.Strings(gotKinds))
		})
	}
}

func TestCRFilterReader(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"\r", "\n"},
		{"\r\n", "\r\n"},
		{"\r\r\n", "\n\r\n"},
		{"a\rb\rc", "a\nb\nc"},
		{" \r ", " \n "},
		{" \r\n\n", " \r\n\n"},
		{"\r \n", "\n \n"},
		{"abc\r", "abc\n"},
		{"abc\r\n\r", "abc\r\n\n"},
	}

	for _, tc := range testCases {
		filter := &crFilterReader{r: bytes.NewReader([]byte(tc.in))}
		actual, err := io.ReadAll(filter)
		require.Nil(t, err)
		require.Equal(t, tc.want, string(actual))
	}
}

func TestSkipBOM(t *testing.T) {
	t.Parallel()

	const bom = "\xef\xbb\xbf"

	testCases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"bom_only", bom, ""},
		{"partial_bom", "\xef\xbb", "\xef\xbb"},
		{"bom_data", bom + "a,b\n", "a,b\n"},
		{"no_bom", "a,b\n", "a,b\n"},
		{"double_bom", bom + bom + "a", bom + "a"},
		{"mid_file_bom", "a," + bom + "b", "a," + bom + "b"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := io.ReadAll(skipBOM(strings.NewReader(tc.in)))
			require.NoError(t, err)
			require.Equal(t, tc.want, string(got))
		})
	}
}

// TestDetectType_BOM verifies that type detection skips a leading UTF-8 BOM.
// A BOM before a quoted first field is a parse error for encoding/csv, so
// without the skip, detection fails for such a file when there's no file
// extension to go on, e.g. when reading from stdin.
func TestDetectType_BOM(t *testing.T) {
	t.Parallel()

	const bom = "\xef\xbb\xbf"

	testCases := []struct {
		name  string
		typ   drivertype.Type
		input string
	}{
		{"csv", drivertype.CSV, bom + "id,name,age\n1,alice,30\n2,bob,40\n"},
		{"csv_quoted_header", drivertype.CSV, bom + "\"id\",\"name\",\"age\"\n1,alice,30\n2,bob,40\n"},
		{"tsv", drivertype.TSV, bom + "id\tname\tage\n1\talice\t30\n2\tbob\t40\n"},
		{"tsv_quoted_header", drivertype.TSV, bom + "\"id\"\t\"name\"\t\"age\"\n1\talice\t30\n2\tbob\t40\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newRdrFn := func(context.Context) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(tc.input)), nil
			}

			gotType, gotScore, err := detectType(context.Background(), tc.typ, newRdrFn)
			require.NoError(t, err)
			require.Equal(t, tc.typ, gotType)
			require.Equal(t, scoreYes, gotScore)
		})
	}
}

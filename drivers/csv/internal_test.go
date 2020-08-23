package csv

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlz"
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
		tc := tc

		t.Run(fmt.Sprintf("%d %s", i, tc.input), func(t *testing.T) {
			t.Parallel()

			cr := csv.NewReader(&crFilterReader{r: strings.NewReader(tc.input)})
			cr.Comma = tc.delim

			got := isCSV(context.Background(), cr)
			require.Equal(t, tc.want, got)
		})
	}
}

func Test_predictColKinds(t *testing.T) {
	const maxExamine = 100

	testCases := []struct {
		wantKinds      []sqlz.Kind
		readAheadInput [][]string
		readerInput    string
	}{
		{
			readAheadInput: [][]string{},
			readerInput:    "",
			wantKinds:      []sqlz.Kind{},
		},
		{
			readAheadInput: [][]string{
				{"1", "true", "hello", "0.0"},
				{"2", "false", "world", "1"},
				{"3", "true", "", "7.7"},
				{"", "", "", ""},
			},
			wantKinds: []sqlz.Kind{sqlz.KindInt, sqlz.KindBool, sqlz.KindText, sqlz.KindDecimal},
		},
		{
			readAheadInput: [][]string{},
			readerInput:    "1,true,hello,0.0\n2,false,world,1\n3,true,,7.7\n,,,",
			wantKinds:      []sqlz.Kind{sqlz.KindInt, sqlz.KindBool, sqlz.KindText, sqlz.KindDecimal},
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			gotKinds, err := predictColKinds(
				len(tc.wantKinds),
				csv.NewReader(strings.NewReader(tc.readerInput)),
				&tc.readAheadInput,
				maxExamine)

			require.NoError(t, err)
			require.Equal(t, tc.wantKinds, gotKinds)
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
		actual, err := ioutil.ReadAll(filter)
		require.Nil(t, err)
		require.Equal(t, tc.want, string(actual))
	}
}

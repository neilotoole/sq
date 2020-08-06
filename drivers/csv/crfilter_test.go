package csv

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_crFilterReader(t *testing.T) {
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

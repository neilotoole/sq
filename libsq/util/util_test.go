package util

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCRFilterReader(t *testing.T) {
	tests := []struct {
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

	for _, test := range tests {
		filter := NewCRFilterReader(bytes.NewReader([]byte(test.in)))
		actual, err := ioutil.ReadAll(filter)
		require.Nil(t, err)
		require.Equal(t, test.want, string(actual))
	}
}

package linesplitreaders

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
)

func TestErrorBufferReader_Read(t *testing.T) {
	testCases := []struct {
		input   []byte
		wantErr error
	}{
		{input: []byte("hello"), wantErr: errors.New("huzzah")},
		{input: []byte(""), wantErr: errors.New("huzzah")},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {

			r := newErrorBufferReader(tc.input, tc.wantErr)
			got, err := io.ReadAll(r)
			require.Equal(t, tc.input, got)
			require.Equal(t, tc.wantErr, err)
		})
	}
}

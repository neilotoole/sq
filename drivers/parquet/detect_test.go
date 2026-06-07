package parquet_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/parquet"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

func newRdrFnFromBytes(b []byte) files.NewReaderFunc {
	return func(_ context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(b)), nil
	}
}

func TestDetectParquet(t *testing.T) {
	const par1 = "PAR1"
	body := bytes.Repeat([]byte{0xAB}, 64)

	testCases := []struct {
		name    string
		payload []byte
		wantTyp drivertype.Type
		wantMin float32
		wantMax float32
	}{
		{
			name:    "head_and_tail",
			payload: append(append([]byte(par1), body...), []byte(par1)...),
			wantTyp: drivertype.Parquet,
			wantMin: 1.0,
			wantMax: 1.0,
		},
		{
			name:    "head_only_truncated",
			payload: append([]byte(par1), body...),
			wantTyp: drivertype.Parquet,
			wantMin: 0.7,
			wantMax: 0.7,
		},
		{
			name:    "not_parquet_csv_like",
			payload: []byte("a,b,c\n1,2,3\n"),
			wantTyp: drivertype.None,
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "empty",
			payload: []byte{},
			wantTyp: drivertype.None,
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "shorter_than_four_bytes",
			payload: []byte("PA"),
			wantTyp: drivertype.None,
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "xlsx_zip_prefix",
			payload: append([]byte{'P', 'K', 0x03, 0x04}, body...),
			wantTyp: drivertype.None,
			wantMin: 0,
			wantMax: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, score, err := parquet.DetectParquet(context.Background(), newRdrFnFromBytes(tc.payload))
			require.NoError(t, err)
			require.Equal(t, tc.wantTyp, got)
			require.GreaterOrEqual(t, score, tc.wantMin)
			require.LessOrEqual(t, score, tc.wantMax)
		})
	}
}

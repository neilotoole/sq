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
		name      string
		payload   []byte
		wantTyp   drivertype.Type
		wantScore float32
	}{
		{
			name:      "head_and_tail",
			payload:   append(append([]byte(par1), body...), []byte(par1)...),
			wantTyp:   drivertype.Parquet,
			wantScore: 1.0,
		},
		{
			name:      "head_only_truncated",
			payload:   append([]byte(par1), body...),
			wantTyp:   drivertype.Parquet,
			wantScore: 0.7,
		},
		{
			name:      "not_parquet_csv_like",
			payload:   []byte("a,b,c\n1,2,3\n"),
			wantTyp:   drivertype.None,
			wantScore: 0,
		},
		{
			name:      "empty",
			payload:   []byte{},
			wantTyp:   drivertype.None,
			wantScore: 0,
		},
		{
			name:      "shorter_than_four_bytes",
			payload:   []byte("PA"),
			wantTyp:   drivertype.None,
			wantScore: 0,
		},
		{
			name:      "xlsx_zip_prefix",
			payload:   append([]byte{'P', 'K', 0x03, 0x04}, body...),
			wantTyp:   drivertype.None,
			wantScore: 0,
		},
		{
			// A 4-byte "PAR1" payload satisfies both head and tail checks: seeking
			// to -4 from end lands at byte 0, reading back the same four bytes.
			name:      "exactly_four_bytes_par1",
			payload:   []byte("PAR1"),
			wantTyp:   drivertype.Parquet,
			wantScore: 1.0,
		},
		{
			// Non-seekable reader carrying a payload larger than the
			// readLastFour drain cap: detector returns head-only (0.7) rather
			// than draining the full body to confirm the tail. This is the
			// hot path for huge remote Parquet over a streaming HTTP body.
			name:      "head_only_when_payload_exceeds_drain_cap",
			payload:   append(append([]byte("PAR1"), bytes.Repeat([]byte{0xAB}, 1<<20+1024)...), []byte("PAR1")...),
			wantTyp:   drivertype.Parquet,
			wantScore: 0.7,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, score, err := parquet.DetectParquet(context.Background(), newRdrFnFromBytes(tc.payload))
			require.NoError(t, err)
			require.Equal(t, tc.wantTyp, got)
			require.Equal(t, tc.wantScore, score)
		})
	}
}

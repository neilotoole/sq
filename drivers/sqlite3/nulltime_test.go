package sqlite3

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNullTimeScan(t *testing.T) {
	// 1700000000s == 1700000000000ms == 2023-11-14 22:13:20 UTC.
	epochInstant := time.Date(2023, 11, 14, 22, 13, 20, 0, time.UTC)
	railsInstant := time.Date(2024, 1, 15, 12, 34, 56, 123456000, time.UTC)
	tm := time.Date(2024, 1, 15, 12, 34, 56, 0, time.UTC)

	testCases := []struct {
		name    string
		src     any
		valid   bool
		isTime  bool
		want    time.Time
		wantStr string
	}{
		{name: "nil", src: nil, valid: false},
		{name: "time", src: tm, valid: true, isTime: true, want: tm},
		{name: "rails_micro", src: "2024-01-15 12:34:56.123456", valid: true, isTime: true, want: railsInstant},
		{name: "rfc3339_z", src: "2024-01-15T12:34:56Z", valid: true, isTime: true, want: tm},
		{name: "date_only", src: "2024-01-15", valid: true, isTime: true, want: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{name: "junk", src: "not-a-date", valid: true, isTime: false, wantStr: "not-a-date"},
		{name: "epoch_seconds", src: int64(1700000000), valid: true, isTime: true, want: epochInstant},
		{name: "epoch_millis", src: int64(1700000000000), valid: true, isTime: true, want: epochInstant},
		{name: "bytes_micro", src: []byte("2024-01-15 12:34:56.123456"), valid: true, isTime: true, want: railsInstant},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var nt nullTime
			require.NoError(t, nt.Scan(tc.src))
			require.Equal(t, tc.valid, nt.Valid)
			require.Equal(t, tc.isTime, nt.IsTime)
			if tc.isTime {
				require.Truef(t, tc.want.Equal(nt.Time), "want %s, got %s", tc.want, nt.Time)
			}
			require.Equal(t, tc.wantStr, nt.String)
		})
	}
}

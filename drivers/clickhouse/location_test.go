package clickhouse_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/clickhouse"
)

// TestLocationWithDefaultPort tests that the default port is correctly applied
// based on the connection URL's secure parameter.
func TestLocationWithDefaultPort(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		wantLoc   string
		wantAdded bool
		wantErr   bool
	}{
		{
			name:      "no_port_non_secure",
			input:     "clickhouse://user:pass@localhost/mydb",
			wantLoc:   "clickhouse://user:pass@localhost:9000/mydb",
			wantAdded: true,
		},
		{
			name:      "no_port_secure",
			input:     "clickhouse://user:pass@localhost/mydb?secure=true",
			wantLoc:   "clickhouse://user:pass@localhost:9440/mydb?secure=true",
			wantAdded: true,
		},
		{
			name:      "explicit_port",
			input:     "clickhouse://user:pass@localhost:19000/mydb",
			wantLoc:   "clickhouse://user:pass@localhost:19000/mydb",
			wantAdded: false,
		},
		{
			name:      "explicit_default_port",
			input:     "clickhouse://user:pass@localhost:9000/mydb",
			wantLoc:   "clickhouse://user:pass@localhost:9000/mydb",
			wantAdded: false,
		},
		{
			name:      "no_port_with_database_param",
			input:     "clickhouse://user:pass@localhost?database=mydb",
			wantLoc:   "clickhouse://user:pass@localhost:9000?database=mydb",
			wantAdded: true,
		},
		{
			name:      "no_port_secure_false",
			input:     "clickhouse://user:pass@localhost/mydb?secure=false",
			wantLoc:   "clickhouse://user:pass@localhost:9000/mydb?secure=false",
			wantAdded: true,
		},
		{
			name:      "no_port_multiple_params",
			input:     "clickhouse://user:pass@localhost/mydb?compress=true&secure=true",
			wantLoc:   "clickhouse://user:pass@localhost:9440/mydb?compress=true&secure=true",
			wantAdded: true,
		},
		{
			name:    "invalid_url",
			input:   "://invalid",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, added, err := clickhouse.LocationWithDefaultPort(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantAdded, added)
			require.Equal(t, tc.wantLoc, got)
		})
	}
}

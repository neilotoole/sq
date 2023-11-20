package sqlparser_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3/internal/sqlparser"
	"github.com/neilotoole/sq/testh/tutil"
)

func TestExtractTableNameFromCreateTableStmt(t *testing.T) {
	testCases := []struct {
		in         string
		unescape   bool
		wantSchema string
		wantTable  string
		wantErr    bool
	}{
		{
			in:        `CREATE TABLE actor ( actor_id INTEGER NOT NULL)`,
			unescape:  true,
			wantTable: "actor",
		},
		{
			in:         `CREATE TABLE "sakila"."actor" ( actor_id INTEGER NOT NULL)`,
			unescape:   true,
			wantSchema: "sakila",
			wantTable:  "actor",
		},
		{
			in:       `CREATE TABLE "sakila"."actor"."not_legal" ( actor_id INTEGER NOT NULL)`,
			unescape: true,
			wantErr:  true,
		},
		{
			in:         `CREATE TABLE [sakila]."actor" ( actor_id INTEGER NOT NULL)`,
			unescape:   true,
			wantSchema: "sakila",
			wantTable:  "actor",
		},
		{
			in:         `CREATE TABLE [sakila]."actor" ( actor_id INTEGER NOT NULL)`,
			unescape:   false,
			wantSchema: "[sakila]",
			wantTable:  `"actor"`,
		},
		{
			in:         `CREATE TABLE "sak ila"."actor" ( actor_id INTEGER NOT NULL)`,
			unescape:   true,
			wantSchema: "sak ila",
			wantTable:  "actor",
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc.in), func(t *testing.T) {
			schema, table, err := sqlparser.ExtractTableIdentFromCreateTableStmt(tc.in, tc.unescape)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantSchema, schema)
			require.Equal(t, tc.wantTable, table)
		})
	}
}

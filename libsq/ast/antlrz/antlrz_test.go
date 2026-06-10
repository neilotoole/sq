package antlrz_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3/sqlparser"
)

// TestTokenExtractor_Offset_UTF8 guards against an off-by-N byte error in
// TokenExtractor.Offset: ANTLR's Token.GetColumn reports a rune-based column
// position, but the returned offset must be a Go byte index so callers can
// safely slice the input string. When a multi-byte UTF-8 identifier appears
// earlier on the same line, the rune count diverges from the byte count.
//
// This test exercises the path through the sqlite3 sqlparser, since that is
// the closest public surface that uses TokenExtractor.Offset for downstream
// string slicing.
func TestTokenExtractor_Offset_UTF8(t *testing.T) {
	testCases := []struct {
		name string
		stmt string
		col  string
	}{
		{
			name: "ascii_only",
			stmt: `CREATE TABLE actor (actor_id INTEGER NOT NULL)`,
			col:  "actor_id",
		},
		{
			name: "non_ascii_table_name",
			stmt: `CREATE TABLE "名前" (actor_id INTEGER NOT NULL)`,
			col:  "actor_id",
		},
		{
			name: "non_ascii_schema_and_table",
			stmt: `CREATE TABLE "名前"."actor" (actor_id INTEGER NOT NULL)`,
			col:  "actor_id",
		},
		{
			name: "non_ascii_column_name",
			stmt: `CREATE TABLE t ("名前" TEXT NOT NULL, actor_id INTEGER)`,
			col:  "actor_id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ident, err := sqlparser.ExtractTableIdentFromCreateTableStmt(tc.stmt)
			require.NoError(t, err)
			require.Equal(t, ident.RawTable,
				tc.stmt[ident.TableOffset:ident.TableOffset+len(ident.RawTable)],
				"TableOffset must be a byte offset; round-trip into input must yield RawTable")
			if ident.SchemaOffset >= 0 {
				require.Equal(t, ident.RawSchema,
					tc.stmt[ident.SchemaOffset:ident.SchemaOffset+len(ident.RawSchema)],
					"SchemaOffset must be a byte offset")
			}

			colDefs, err := sqlparser.ExtractCreateTableStmtColDefs(tc.stmt)
			require.NoError(t, err)
			require.NotEmpty(t, colDefs)
			for _, cd := range colDefs {
				require.Equal(t, cd.RawName,
					tc.stmt[cd.RawNameOffset:cd.RawNameOffset+len(cd.RawName)],
					"RawNameOffset must be a byte offset; round-trip into input must yield RawName")
				require.Equal(t, cd.RawType,
					tc.stmt[cd.RawTypeOffset:cd.RawTypeOffset+len(cd.RawType)],
					"RawTypeOffset must be a byte offset; round-trip into input must yield RawType")
			}
		})
	}
}

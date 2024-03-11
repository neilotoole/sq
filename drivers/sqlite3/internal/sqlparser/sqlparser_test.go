package sqlparser_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3/internal/sqlparser"
	"github.com/neilotoole/sq/testh/tu"
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
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
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

func TestAlterCreateTableColumnType(t *testing.T) {
	const input = `CREATE TABLE "og_table" (
"name" TEXT NOT NULL,
"age" INTEGER( 10 ) NOT NULL,
"weight" INTEGER NOT NULL
)`

	gotColName, gotColType, err := sqlparser.ExtractColNameAndTypeFromCreateStmt(input, "age")
	require.NoError(t, err)
	require.Equal(t, `"age"`, gotColName)
	require.Equal(t, "INTEGER", gotColType)

	//	const want = `CREATE TABLE "og_table" (
	//"name" TEXT NOT NULL,
	//"age" TEXT NOT NULL,
	//"weight" INTEGER NOT NULL
	//)`
	//
	//	require.Equal(t, want, got)
}

func TestExtractCreateStmtColDefs(t *testing.T) {
	const input = `CREATE TABLE "og_table" (
"name" TEXT NOT NULL,
"age" INTEGER( 10 ) NOT NULL,
weight INTEGER NOT NULL
)`

	colDefs, err := sqlparser.ExtractCreateStmtColDefs(input)
	require.NoError(t, err)
	require.Len(t, colDefs, 3)
	require.Equal(t, `"name" TEXT NOT NULL`, colDefs[0].Raw)
	require.Equal(t, `"name"`, colDefs[0].RawName)
	require.Equal(t, `name`, colDefs[0].Name)
	require.Equal(t, "TEXT", colDefs[0].Type)
	require.Equal(t, "TEXT", colDefs[0].RawType)
	snippet := input[colDefs[0].InputOffset : colDefs[0].InputOffset+len(colDefs[0].Raw)]
	require.Equal(t, colDefs[0].Raw, snippet)

	require.Equal(t, `"age" INTEGER( 10 ) NOT NULL`, colDefs[1].Raw)
	require.Equal(t, `"age"`, colDefs[1].RawName)
	require.Equal(t, `age`, colDefs[1].Name)
	require.Equal(t, "INTEGER(10)", colDefs[1].Type)
	require.Equal(t, "INTEGER( 10 )", colDefs[1].RawType)
	snippet = input[colDefs[1].InputOffset : colDefs[1].InputOffset+len(colDefs[1].Raw)]
	require.Equal(t, colDefs[1].Raw, snippet)

	require.Equal(t, `weight INTEGER NOT NULL`, colDefs[2].Raw)
	require.Equal(t, `weight`, colDefs[2].RawName)
	require.Equal(t, `weight`, colDefs[2].Name)
	require.Equal(t, "INTEGER", colDefs[2].Type)
	require.Equal(t, "INTEGER", colDefs[2].RawType)
	snippet = input[colDefs[2].InputOffset : colDefs[2].InputOffset+len(colDefs[2].Raw)]
	require.Equal(t, colDefs[2].Raw, snippet)
}

func TestExtractColNamesAndTypesFromCreateStmt(t *testing.T) {
	const input = `CREATE TABLE "og_table" (
"name" TEXT NOT NULL,
"age" INTEGER( 10 ) NOT NULL,
weight INTEGER NOT NULL
)`

	names, types, err := sqlparser.ExtractColNamesAndTypesFromCreateStmt(input)
	require.NoError(t, err)
	require.Equal(t, []string{`"name"`, `"age"`, `weight`}, names)
	require.Equal(t, []string{"TEXT", "INTEGER(10)", "INTEGER"}, types)
}

func TestCanonicalizeCreateStmtColNames(t *testing.T) {
	const input = `CREATE TABLE "og_table" (
"name" TEXT NOT NULL,
"age" INTEGER( 10 ) NOT NULL,
weight INTEGER NOT NULL
)`
	const want = `CREATE TABLE "og_table" (
"name" TEXT NOT NULL,
"age" INTEGER( 10 ) NOT NULL,
"weight" INTEGER NOT NULL
)`

	got, err := sqlparser.CanonicalizeCreateStmtColNames(input)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

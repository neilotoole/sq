package sqlparser_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/sqlite3/sqlparser"
	"github.com/neilotoole/sq/testh/tu"
)

func TestExtractTableIdentFromCreateTableStmt(t *testing.T) {
	testCases := []struct {
		in            string
		wantSchema    string
		wantTable     string
		wantRawSchema string
		wantRawTable  string
		wantErr       bool
	}{
		{
			in:           `CREATE TABLE actor ( actor_id INTEGER NOT NULL)`,
			wantTable:    "actor",
			wantRawTable: "actor",
		},
		{
			in:            `CREATE TABLE "sakila"."actor" ( actor_id INTEGER NOT NULL)`,
			wantSchema:    "sakila",
			wantTable:     "actor",
			wantRawSchema: `"sakila"`,
			wantRawTable:  `"actor"`,
		},
		{
			in:      `CREATE TABLE "sakila"."actor"."not_legal" ( actor_id INTEGER NOT NULL)`,
			wantErr: true,
		},
		{
			in:            `CREATE TABLE [sakila]."actor" ( actor_id INTEGER NOT NULL)`,
			wantSchema:    "sakila",
			wantTable:     "actor",
			wantRawSchema: "[sakila]",
			wantRawTable:  `"actor"`,
		},
		{
			in:            `CREATE TABLE "sak ila"."actor" ( actor_id INTEGER NOT NULL)`,
			wantSchema:    "sak ila",
			wantTable:     "actor",
			wantRawSchema: `"sak ila"`,
			wantRawTable:  `"actor"`,
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			ident, err := sqlparser.ExtractTableIdentFromCreateTableStmt(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, ident)
			require.Equal(t, tc.wantSchema, ident.Schema)
			require.Equal(t, tc.wantTable, ident.Table)
			require.Equal(t, tc.wantRawSchema, ident.RawSchema)
			require.Equal(t, tc.wantRawTable, ident.RawTable)

			// Offsets must round-trip: input[offset:offset+len(raw)] == raw.
			require.Equal(t, ident.RawTable,
				tc.in[ident.TableOffset:ident.TableOffset+len(ident.RawTable)],
				"TableOffset must point at RawTable in the input")
			if ident.RawSchema == "" {
				require.Equal(t, -1, ident.SchemaOffset,
					"SchemaOffset must be -1 when no schema is present")
			} else {
				require.Equal(t, ident.RawSchema,
					tc.in[ident.SchemaOffset:ident.SchemaOffset+len(ident.RawSchema)],
					"SchemaOffset must point at RawSchema in the input")
			}
		})
	}
}

func TestExtractCreateTableStmtColDefs(t *testing.T) {
	const input = `CREATE TABLE "og_table" (
"name" TEXT NOT NULL,
"age" INTEGER( 10 ) NOT NULL,
weight INTEGER NOT NULL
)`

	colDefs, err := sqlparser.ExtractCreateTableStmtColDefs(input)
	require.NoError(t, err)
	require.Len(t, colDefs, 3)
	require.Equal(t, `"name" TEXT NOT NULL`, colDefs[0].Raw)
	require.Equal(t, `"name"`, colDefs[0].RawName)
	require.Equal(t, `name`, colDefs[0].Name)
	require.Equal(t, "TEXT", colDefs[0].Type)
	require.Equal(t, "TEXT", colDefs[0].RawType)
	snippet := input[colDefs[0].InputOffset : colDefs[0].InputOffset+len(colDefs[0].Raw)]
	require.Equal(t, colDefs[0].Raw, snippet)
	require.Equal(t, colDefs[0].RawName,
		input[colDefs[0].RawNameOffset:colDefs[0].RawNameOffset+len(colDefs[0].RawName)])
	require.Equal(t, colDefs[0].RawType,
		input[colDefs[0].RawTypeOffset:colDefs[0].RawTypeOffset+len(colDefs[0].RawType)])

	require.Equal(t, `"age" INTEGER( 10 ) NOT NULL`, colDefs[1].Raw)
	require.Equal(t, `"age"`, colDefs[1].RawName)
	require.Equal(t, `age`, colDefs[1].Name)
	require.Equal(t, "INTEGER(10)", colDefs[1].Type)
	require.Equal(t, "INTEGER( 10 )", colDefs[1].RawType)
	snippet = input[colDefs[1].InputOffset : colDefs[1].InputOffset+len(colDefs[1].Raw)]
	require.Equal(t, colDefs[1].Raw, snippet)
	require.Equal(t, colDefs[1].RawName,
		input[colDefs[1].RawNameOffset:colDefs[1].RawNameOffset+len(colDefs[1].RawName)])
	require.Equal(t, colDefs[1].RawType,
		input[colDefs[1].RawTypeOffset:colDefs[1].RawTypeOffset+len(colDefs[1].RawType)])

	require.Equal(t, `weight INTEGER NOT NULL`, colDefs[2].Raw)
	require.Equal(t, `weight`, colDefs[2].RawName)
	require.Equal(t, `weight`, colDefs[2].Name)
	require.Equal(t, "INTEGER", colDefs[2].Type)
	require.Equal(t, "INTEGER", colDefs[2].RawType)
	snippet = input[colDefs[2].InputOffset : colDefs[2].InputOffset+len(colDefs[2].Raw)]
	require.Equal(t, colDefs[2].Raw, snippet)
	require.Equal(t, colDefs[2].RawName,
		input[colDefs[2].RawNameOffset:colDefs[2].RawNameOffset+len(colDefs[2].RawName)])
	require.Equal(t, colDefs[2].RawType,
		input[colDefs[2].RawTypeOffset:colDefs[2].RawTypeOffset+len(colDefs[2].RawType)])
}

// TestExtractCreateTableStmtColDefs_QuotedIdentifiers verifies that ColDef.Name
// is stripped of all four SQLite legal identifier-quoting styles: double-quote,
// single-quote, backtick, and square brackets. See issue #752.
func TestExtractCreateTableStmtColDefs_QuotedIdentifiers(t *testing.T) {
	testCases := []struct {
		name        string
		stmt        string
		wantRawName string
	}{
		{
			name:        "double_quote",
			stmt:        `CREATE TABLE t ("age" INTEGER NOT NULL)`,
			wantRawName: `"age"`,
		},
		{
			name:        "single_quote",
			stmt:        `CREATE TABLE t ('age' INTEGER NOT NULL)`,
			wantRawName: `'age'`,
		},
		{
			name:        "backtick",
			stmt:        "CREATE TABLE t (`age` INTEGER NOT NULL)",
			wantRawName: "`age`",
		},
		{
			name:        "square_brackets",
			stmt:        `CREATE TABLE t ([age] INTEGER NOT NULL)`,
			wantRawName: `[age]`,
		},
		{
			name:        "unquoted",
			stmt:        `CREATE TABLE t (age INTEGER NOT NULL)`,
			wantRawName: `age`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			colDefs, err := sqlparser.ExtractCreateTableStmtColDefs(tc.stmt)
			require.NoError(t, err)
			require.Len(t, colDefs, 1)
			require.Equal(t, tc.wantRawName, colDefs[0].RawName)
			require.Equal(t, "age", colDefs[0].Name)
		})
	}
}

// TestExtractCreateTableStmtColDefs_Offsets_NameVsType is the offset
// counterpart to gh750: when a column name shares a prefix with its type
// token (or equals it outright), substring-based rewrites can clobber the
// name. RawTypeOffset must point past the name to the actual type token.
func TestExtractCreateTableStmtColDefs_Offsets_NameVsType(t *testing.T) {
	testCases := []struct {
		name string
		stmt string
		col  string
		typ  string
	}{
		{
			name: "name_prefixes_type",
			stmt: `CREATE TABLE t (text_payload TEXT NOT NULL)`,
			col:  "text_payload",
			typ:  "TEXT",
		},
		{
			name: "name_equals_type",
			stmt: `CREATE TABLE t (INTEGER INTEGER NOT NULL)`,
			col:  "INTEGER",
			typ:  "INTEGER",
		},
		{
			name: "name_contains_type",
			stmt: `CREATE TABLE t (my_text_col TEXT NOT NULL)`,
			col:  "my_text_col",
			typ:  "TEXT",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			colDefs, err := sqlparser.ExtractCreateTableStmtColDefs(tc.stmt)
			require.NoError(t, err)
			require.Len(t, colDefs, 1)
			cd := colDefs[0]
			require.Equal(t, tc.col, cd.Name)
			require.Equal(t, tc.typ, cd.RawType)

			// Offsets must round-trip: input[offset:offset+len(raw)] == raw.
			require.Equal(t, cd.RawName,
				tc.stmt[cd.RawNameOffset:cd.RawNameOffset+len(cd.RawName)],
				"RawNameOffset must point at the name token, not a substring match in the type")
			require.Equal(t, cd.RawType,
				tc.stmt[cd.RawTypeOffset:cd.RawTypeOffset+len(cd.RawType)],
				"RawTypeOffset must point at the type token, not the name's prefix")

			// And specifically: the type offset must come AFTER the name offset.
			require.Greater(t, cd.RawTypeOffset, cd.RawNameOffset,
				"RawTypeOffset must come after RawNameOffset, otherwise the type-replace would clobber the name")
		})
	}
}

func TestApplyEdits(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		edits   []sqlparser.Edit
		want    string
		wantErr bool
	}{
		{
			name:  "empty",
			input: "hello world",
			edits: nil,
			want:  "hello world",
		},
		{
			name:  "single_middle",
			input: "hello world",
			edits: []sqlparser.Edit{{Start: 6, End: 11, Replacement: "there"}},
			want:  "hello there",
		},
		{
			name:  "two_in_order",
			input: "abcdef",
			edits: []sqlparser.Edit{
				{Start: 0, End: 1, Replacement: "X"},
				{Start: 4, End: 5, Replacement: "Y"},
			},
			want: "XbcdYf",
		},
		{
			name:  "two_out_of_order_input",
			input: "abcdef",
			edits: []sqlparser.Edit{
				{Start: 4, End: 5, Replacement: "Y"},
				{Start: 0, End: 1, Replacement: "X"},
			},
			want: "XbcdYf",
		},
		{
			name:  "adjacent_edits",
			input: "abcdef",
			edits: []sqlparser.Edit{
				{Start: 1, End: 3, Replacement: "XX"},
				{Start: 3, End: 5, Replacement: "YY"},
			},
			want: "aXXYYf",
		},
		{
			name:  "insertion",
			input: "abcdef",
			edits: []sqlparser.Edit{{Start: 3, End: 3, Replacement: "_INS_"}},
			want:  "abc_INS_def",
		},
		{
			name:  "deletion",
			input: "abcdef",
			edits: []sqlparser.Edit{{Start: 2, End: 4, Replacement: ""}},
			want:  "abef",
		},
		{
			name:  "longer_replacement",
			input: "name TYPE x",
			edits: []sqlparser.Edit{{Start: 5, End: 9, Replacement: "VERYLONGTYPE"}},
			want:  "name VERYLONGTYPE x",
		},
		{
			name:  "overlapping_error",
			input: "abcdef",
			edits: []sqlparser.Edit{
				{Start: 0, End: 3, Replacement: "X"},
				{Start: 2, End: 4, Replacement: "Y"},
			},
			wantErr: true,
		},
		{
			name:    "end_before_start_error",
			input:   "abcdef",
			edits:   []sqlparser.Edit{{Start: 4, End: 2, Replacement: "X"}},
			wantErr: true,
		},
		{
			name:    "out_of_range_error",
			input:   "abc",
			edits:   []sqlparser.Edit{{Start: 0, End: 10, Replacement: "X"}},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sqlparser.ApplyEdits(tc.input, tc.edits)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestApplyEdits_DDLRewriteIntegration sanity-checks the end-to-end story
// that motivates gh750: parser offsets feed ApplyEdits, and the resulting
// DDL is correct even when the column name shares a prefix with its type
// (the classic substring-collision failure mode).
func TestApplyEdits_DDLRewriteIntegration(t *testing.T) {
	const input = `CREATE TABLE t (text_payload TEXT NOT NULL)`
	colDefs, err := sqlparser.ExtractCreateTableStmtColDefs(input)
	require.NoError(t, err)
	require.Len(t, colDefs, 1)
	cd := colDefs[0]

	got, err := sqlparser.ApplyEdits(input, []sqlparser.Edit{
		{Start: cd.RawTypeOffset, End: cd.RawTypeOffset + len(cd.RawType), Replacement: "INTEGER"},
	})
	require.NoError(t, err)
	require.Equal(t, `CREATE TABLE t (text_payload INTEGER NOT NULL)`, got)
	require.True(t, strings.Contains(got, "text_payload"),
		"naive strings.Replace(input, RawType, ...) would have produced 'INTEGER_payload INTEGER'; offsets prevent that")
}

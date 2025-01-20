package sqlparser_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlparser"
	"github.com/neilotoole/sq/testh/proj"
)

func TestSplitInput(t *testing.T) {
	const sel, other = sqlparser.StmtSelect, sqlparser.StmtOther

	// convenience func to return a slice of n sqlparser.StmtType.
	nTypes := func(n int, typ sqlparser.StmtType) []sqlparser.StmtType {
		types := make([]sqlparser.StmtType, n)
		for i := range types {
			types[i] = typ
		}
		return types
	}

	// testCases key is a group of tests, e.g. "postgres" or "sqlserver"
	testCases := map[string][]struct {
		name         string
		delim        string
		moreDelims   []string
		input        string
		wantCount    int
		wantContains []string
		wantTypes    []sqlparser.StmtType
	}{
		"simple": {
			{
				name:         "select_1",
				delim:        ";",
				input:        "select * from my_table; ",
				wantCount:    1,
				wantContains: []string{"select * from my_table"},
				wantTypes:    []sqlparser.StmtType{sel},
			},
			{
				name:         "select_1_no_delim",
				delim:        ";",
				input:        "select * from my_table",
				wantCount:    1,
				wantContains: []string{"select * from my_table"},
				wantTypes:    []sqlparser.StmtType{sel},
			},
			{
				name:         "select_1_many_delim",
				delim:        ";",
				input:        "select * from my_table;\n;;",
				wantCount:    1,
				wantContains: []string{"select * from my_table"},
				wantTypes:    []sqlparser.StmtType{sel},
			},
			{
				name:         "select_2",
				delim:        ";",
				input:        "select * from my_table;\ndrop table my_table;",
				wantCount:    2,
				wantContains: []string{"select * from my_table", "drop table my_table"},
				wantTypes:    []sqlparser.StmtType{sel, other},
			},
			{
				name:         "select_2_no_trailing_delim",
				delim:        ";",
				input:        "select * from my_table;\ndrop table my_table",
				wantCount:    2,
				wantContains: []string{"select * from my_table", "drop table my_table"},
				wantTypes:    []sqlparser.StmtType{sel, other},
			},
		},
		"sqlite3": {
			{
				name:      "sqlite3_type_test.sql",
				delim:     ";",
				input:     string(proj.ReadFile("libsq/core/sqlparser/testdata/sqlite3_type_test.sql")),
				wantCount: 4,
				wantTypes: nTypes(4, other),
			},
			{
				name:      "sqlite3_address.sql",
				delim:     ";",
				input:     string(proj.ReadFile("libsq/core/sqlparser/testdata/sqlite3_address.sql")),
				wantCount: 4,
				wantTypes: nTypes(4, other),
			},
			{
				name:      "sqlite3_person.sql",
				delim:     ";",
				input:     string(proj.ReadFile("libsq/core/sqlparser/testdata/sqlite3_person.sql")),
				wantCount: 10,
				wantTypes: nTypes(10, other),
			},
		},
		"postgres": {
			{
				name:      "sqtype_public_type_test.sql",
				delim:     ";",
				input:     string(proj.ReadFile("libsq/core/sqlparser/testdata/postgres_public_type_test.sql")),
				wantCount: 5,
				wantTypes: nTypes(5, other),
			},
			{
				name:      "sqtest_public_address.sql",
				delim:     ";",
				input:     string(proj.ReadFile("libsq/core/sqlparser/testdata/postgres_public_address.sql")),
				wantCount: 5,
				wantTypes: nTypes(5, other),
			},
			{
				name:      "sqtest_public_person.sql",
				delim:     ";",
				input:     string(proj.ReadFile("libsq/core/sqlparser/testdata/postgres_public_person.sql")),
				wantCount: 11,
				wantTypes: nTypes(11, other),
			},
		},
		"sqlserver": {
			{
				name:         "select_1_semi_go",
				delim:        ";",
				moreDelims:   []string{"go"},
				input:        "select * from my_table;\ngo",
				wantCount:    1,
				wantContains: []string{"select * from my_table"},
				wantTypes:    []sqlparser.StmtType{sel},
			},
			{
				name:         "select_2",
				delim:        ";",
				moreDelims:   []string{"go"},
				input:        "select * from my_table;\ndrop table my_table;",
				wantCount:    2,
				wantContains: []string{"select * from my_table", "drop table my_table"},
				wantTypes:    []sqlparser.StmtType{sel, other},
			},
			{
				name:       "sqtype_dbo_type_test.sql",
				delim:      ";",
				moreDelims: []string{"go"},
				input:      string(proj.ReadFile("libsq/core/sqlparser/testdata/sqlserver_dbo_type_test.sql")),
				wantCount:  5,
				wantTypes:  nTypes(5, other),
			},
			{
				name:       "sqtest_dbo_address.sql",
				delim:      ";",
				moreDelims: []string{"go"},
				input:      string(proj.ReadFile("libsq/core/sqlparser/testdata/sqlserver_dbo_address.sql")),
				wantCount:  4,
				wantTypes:  nTypes(5, other),
			},
			{
				name:       "sqtest_dbo_person.sql",
				delim:      ";",
				moreDelims: []string{"go"},
				input:      string(proj.ReadFile("libsq/core/sqlparser/testdata/sqlserver_dbo_person.sql")),
				wantCount:  10,
				wantTypes:  nTypes(10, other),
			},
		},
		"mysql": {
			{
				name:      "mysql_type_test.sql",
				delim:     ";",
				input:     string(proj.ReadFile("libsq/core/sqlparser/testdata/mysql_type_test.sql")),
				wantCount: 5,
				wantTypes: nTypes(5, other),
			},
			{
				name:      "mysql_address.sql",
				delim:     ";",
				input:     string(proj.ReadFile("libsq/core/sqlparser/testdata/mysql_address.sql")),
				wantCount: 4,
				wantTypes: nTypes(5, other),
			},
			{
				name:      "mysql_person.sql",
				delim:     ";",
				input:     string(proj.ReadFile("libsq/core/sqlparser/testdata/mysql_person.sql")),
				wantCount: 9,
				wantTypes: nTypes(9, other),
			},
		},
	}

	for groupName, testGroup := range testCases {
		testGroup := testGroup
		t.Run(groupName, func(t *testing.T) {
			for _, tc := range testGroup {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					stmts, stmtTypes, err := sqlparser.SplitSQL(
						context.Background(),
						strings.NewReader(tc.input),
						tc.delim,
						tc.moreDelims...,
					)
					require.NoError(t, err)
					require.Equal(t, tc.wantCount, len(stmts))
					require.Equal(t, len(stmts), len(stmtTypes))

					for i, stmtType := range stmtTypes {
						require.Equal(t, tc.wantTypes[i], stmtType)
					}

					for i, wantContains := range tc.wantContains {
						require.Contains(t, stmts[i], wantContains)
					}

					// Sanity check to verify that we've stripped trailing delims
					allDelims := append([]string{tc.delim}, tc.moreDelims...)
					for _, sep := range allDelims {
						for _, stmt := range stmts {
							require.False(t, strings.HasSuffix(stmt, sep))
						}
					}
				})
			}
		})
	}
}

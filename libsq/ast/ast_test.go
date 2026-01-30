package ast_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/testh/tu"
)

// TestParseCatalogSchema_ErrorMessages verifies that error messages from
// ParseCatalogSchema are informative and include context about what went wrong.
func TestParseCatalogSchema_ErrorMessages(t *testing.T) {
	testCases := []struct {
		in          string
		wantContain string // Error message should contain this substring
	}{
		{in: "", wantContain: "empty"},
		{in: ".", wantContain: `"."`},                                        // Should mention the input
		{in: ".dbo", wantContain: `".dbo"`},                                  // Should mention the input
		{in: "catalog.schema.table", wantContain: "no valid selector found"}, // Too many components
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			_, _, err := ast.ParseCatalogSchema(tc.in)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContain,
				"error message should contain %q, got: %s", tc.wantContain, err.Error())
		})
	}
}

// TestParseCatalogSchema tests the ParseCatalogSchema function which parses
// strings of the form "catalog.schema" or just "schema". It validates parsing
// of standard identifiers, quoted identifiers, and numeric/numeric-prefixed
// identifiers (issue #470).
// See: https://github.com/neilotoole/sq/issues/470
func TestParseCatalogSchema(t *testing.T) {
	testCases := []struct {
		in                      string
		wantCatalog, wantSchema string
		wantErr                 bool
	}{
		{in: "", wantErr: true},
		{in: ".", wantErr: true},
		{in: "dbo", wantCatalog: "", wantSchema: "dbo"},
		{in: "sakila.", wantCatalog: "sakila", wantSchema: ""},
		{in: ".dbo", wantErr: true},
		{in: "sakila.dbo", wantCatalog: "sakila", wantSchema: "dbo"},
		{in: `"my catalog"."my schema"`, wantCatalog: "my catalog", wantSchema: "my schema"},
		{in: `"my catalog""."my schema"`, wantErr: true},
		{in: `"my catalog"."my schema"."my table"`, wantErr: true},
		{in: `catalog.schema.table`, wantErr: true},
		// Test numeric schema names (issue #470)
		{in: "123", wantCatalog: "", wantSchema: "123"},
		{in: "456.789", wantCatalog: "456", wantSchema: "789"},
		{in: "sakila.123", wantCatalog: "sakila", wantSchema: "123"},
		{in: "123.dbo", wantCatalog: "123", wantSchema: "dbo"},
		{in: "123abc", wantCatalog: "", wantSchema: "123abc"},
		{in: "123abc.def", wantCatalog: "123abc", wantSchema: "def"},
		{in: "456.123abc", wantCatalog: "456", wantSchema: "123abc"},
		{in: "123abc.456def", wantCatalog: "123abc", wantSchema: "456def"},
		// Edge cases for numeric identifiers (issue #470)
		{in: "0", wantCatalog: "", wantSchema: "0"},             // zero as schema
		{in: "0.0", wantCatalog: "0", wantSchema: "0"},          // zero as both
		{in: "0.123", wantCatalog: "0", wantSchema: "123"},      // zero as catalog
		{in: "123e10", wantCatalog: "", wantSchema: "123e10"},   // looks like exp notation, but is IDNUM
		{in: "1e", wantCatalog: "", wantSchema: "1e"},           // single digit + letter
		{in: "123_", wantCatalog: "", wantSchema: "123_"},       // trailing underscore
		{in: "_123", wantCatalog: "", wantSchema: "_123"},       // leading underscore (ID, not IDNUM)
		{in: "007bond", wantCatalog: "", wantSchema: "007bond"}, // leading zeros
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			gotCatalog, gotSchema, gotErr := ast.ParseCatalogSchema(tc.in)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.wantCatalog, gotCatalog)
			require.Equal(t, tc.wantSchema, gotSchema)
		})
	}
}

package ast_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/testh/tu"
)

func TestParseCatalogSchema(t *testing.T) {
	testCases := []struct {
		in                      string
		wantCatalog, wantSchema string
		wantErr                 bool
	}{
		{in: "", wantErr: true},
		{in: "dbo", wantCatalog: "", wantSchema: "dbo"},
		{in: "sakila.dbo", wantCatalog: "sakila", wantSchema: "dbo"},
		{in: `"my catalog"."my schema"`, wantCatalog: "my catalog", wantSchema: "my schema"},
		{in: `"my catalog""."my schema"`, wantErr: true},
		{in: `"my catalog"."my schema"."my table"`, wantErr: true},
		{in: `catalog.schema.table`, wantErr: true},
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

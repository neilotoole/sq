package mysql

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/lg"
)

// TestRenderFuncAvg_versionCast builds a minimal avg(actor_id) *ast.FuncNode
// by parsing SLQ and extracting the node, rather than constructing one
// directly: ast.FuncNode (libsq/ast/func.go) has no exported constructor —
// its fnName field is unexported and Text() reads from an ANTLR parse-tree
// context that only the parser populates. Parsing ".actor | avg(.actor_id)"
// and pulling the *ast.FuncNode via ast.FindFirstNode gives a real node that
// RenderFuncDefault can render as the inner "avg(`actor_id`)".
func TestRenderFuncAvg_versionCast(t *testing.T) {
	d := &driveri{}
	r := d.Renderer()
	dl := d.Dialect()

	a, err := ast.Parse(lg.Discard(), ".actor | avg(.actor_id)")
	require.NoError(t, err)
	fn := ast.FindFirstNode[*ast.FuncNode](a)
	require.NotNil(t, fn)
	require.Equal(t, "avg", fn.FuncName())

	newCtx := func(semver string) *render.Context {
		return &render.Context{Renderer: r, Dialect: dl, DBSemver: semver}
	}

	testCases := []struct {
		semver   string
		wantFrag string
	}{
		{semver: "v8.0.17", wantFrag: "CAST("}, // >= 8.0.17 → CAST ... AS DOUBLE
		{semver: "v8.0.36", wantFrag: "CAST("},
		{semver: "v8.0.16", wantFrag: "+ 0e0"}, // < 8.0.17 → + 0e0
		{semver: "v5.6.51", wantFrag: "+ 0e0"},
		{semver: "", wantFrag: "CAST("}, // unknown → modern default
	}

	for _, tc := range testCases {
		t.Run(tc.semver, func(t *testing.T) {
			got, err := renderFuncAvg(newCtx(tc.semver), fn)
			require.NoError(t, err)
			require.Contains(t, got, tc.wantFrag, "semver %s → %q", tc.semver, got)
			if tc.wantFrag == "+ 0e0" {
				require.NotContains(t, got, "DOUBLE")
			}
		})
	}
}

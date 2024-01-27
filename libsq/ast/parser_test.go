package ast

import (
	"testing"

	antlr "github.com/antlr4-go/antlr/v4"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/testh/tu"
)

// getSLQParser returns a parser for the given SQL input.
func getSLQParser(input string) *slq.SLQParser {
	is := antlr.NewInputStream(input)
	lex := slq.NewSLQLexer(is)
	ts := antlr.NewCommonTokenStream(lex, 0)
	p := slq.NewSLQParser(ts)
	return p
}

// buildInitialAST returns a new AST created by parseTreeVisitor. The AST has not
// yet been processed.
func buildInitialAST(t *testing.T, input string) (*AST, error) {
	t.Helper()
	log := lgt.New(t)

	p := getSLQParser(input)
	q, _ := p.Query().(*slq.QueryContext)
	v := &parseTreeVisitor{log: log}
	err := q.Accept(v)
	if err != nil {
		return nil, err.(error)
	}

	return v.ast, nil
}

// mustParse builds a full AST from the input SLQ, or fails on any error.
func mustParse(t *testing.T, input string) *AST {
	t.Helper()
	log := lgt.New(t)

	ast, err := Parse(log, input)
	require.NoError(t, err)
	return ast
}

func TestSimpleQuery(t *testing.T) {
	const q1 = `@mydb1 | .user | .uid, .username`
	log := lgt.New(t)

	ptree, err := parseSLQ(log, q1)
	require.Nil(t, err)
	require.NotNil(t, ptree)

	ast, err := buildAST(log, ptree)
	require.Nil(t, err)
	require.NotNil(t, ast)
}

// TestParseBuild performs some basic testing of the parser.
// These tests are largely duplicates of other tests, and
// probably should be consolidated.
func TestParseBuild(t *testing.T) {
	testCases := []struct {
		name string
		in   string
	}{
		{"rr1", `@mydb1 | .user | .uid, .username | .[]`},
		{"rr2", `@mydb1 | .user | .uid, .username | .[2]`},
		{"rr3", `@mydb1 | .user | .uid, .username | .[1:3]`},
		{"rr4", `@mydb1 | .user | .uid, .username | .[0:3]`},
		{"rr5", `@mydb1 | .user | .uid, .username | .[:3]`},
		{"rr6", `@mydb1 | .user | .uid, .username | .[2:]`},
		{"join with row range", `@my1 |.user | join(.address, .uid) |  .[0:4] | .user.uid, .username, .country`},
		{"join1", `@mydb1 | .user | join(.address, .user.uid == .address.uid) | .uid, .username, .country`},
		{"select1", `@mydb1 | .user | .uid, .username`},
		{"tbl datasource", `@mydb1.user | .uid, .username`},
		{"count1", `@mydb1.user | count`},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.name), func(t *testing.T) {
			t.Logf(tc.in)
			log := lgt.New(t)

			ptree, err := parseSLQ(log, tc.in)
			require.Nil(t, err)
			require.NotNil(t, ptree)

			ast, err := buildAST(log, ptree)
			require.Nil(t, err)
			require.NotNil(t, ast)
		})
	}
}

func TestInspector_FindWhereClauses(t *testing.T) {
	log := lgt.New(t)

	// Verify that "where(.uid > 4)" becomes a WHERE clause.
	const input = "@my1 | .actor | where(.uid > 4) | .uid, .username"

	ptree, err := parseSLQ(log, input)
	require.Nil(t, err)
	require.NotNil(t, ptree)

	nRoot, err := buildAST(log, ptree)
	require.Nil(t, err)

	insp := NewInspector(nRoot)
	whereNodes, err := insp.FindWhereClauses()
	require.NoError(t, err)
	require.Len(t, whereNodes, 1)
}

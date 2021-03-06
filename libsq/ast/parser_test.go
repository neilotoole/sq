package ast

import (
	"testing"

	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/lg/testlg"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

const (
	fixtRowRange1    = `@mydb1 | .user | .uid, .username | .[]`
	fixtRowRange2    = `@mydb1 | .user | .uid, .username | .[2]`
	fixtRowRange3    = `@mydb1 | .user | .uid, .username | .[1:3]`
	fixtRowRange4    = `@mydb1 | .user | .uid, .username | .[0:3]`
	fixtRowRange5    = `@mydb1 | .user | .uid, .username | .[:3]`
	fixtRowRange6    = `@mydb1 | .user | .uid, .username | .[2:]`
	fixtJoinRowRange = `@my1 |.user, .address | join(.uid) |  .[0:4] | .user.uid, .username, .country`
	fixtJoinQuery1   = `@mydb1 | .user, .address | join(.user.uid == .address.uid) | .uid, .username, .country`
	fixtSelect1      = `@mydb1 | .user | .uid, .username`
)

var slqInputs = map[string]string{
	"rr1":                 `@mydb1 | .user | .uid, .username | .[]`,
	"rr2":                 `@mydb1 | .user | .uid, .username | .[2]`,
	"rr3":                 `@mydb1 | .user | .uid, .username | .[1:3]`,
	"rr4":                 `@mydb1 | .user | .uid, .username | .[0:3]`,
	"rr5":                 `@mydb1 | .user | .uid, .username | .[:3]`,
	"rr6":                 `@mydb1 | .user | .uid, .username | .[2:]`,
	"join with row range": `@my1 |.user, .address | join(.uid) |  .[0:4] | .user.uid, .username, .country`,
	"join1":               `@mydb1 | .user, .address | join(.user.uid == .address.uid) | .uid, .username, .country`,
	"select1":             `@mydb1 | .user | .uid, .username`,
	"tbl datasource":      `@mydb1.user | .uid, .username`,
	"count1":              `@mydb1.user | count(*)`,
}

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
	log := testlg.New(t).Strict(true)

	p := getSLQParser(input)
	q := p.Query().(*slq.QueryContext)
	v := &parseTreeVisitor{log: log}
	err := q.Accept(v)
	if err != nil {
		return nil, err.(error)
	}

	return v.AST, nil
}

// mustBuildAST builds a full AST from the input SLQ, or fails on any error.
func mustBuildAST(t *testing.T, input string) *AST {
	log := testlg.New(t).Strict(true)

	ptree, err := parseSLQ(log, input)
	require.Nil(t, err)
	require.NotNil(t, ptree)

	ast, err := buildAST(log, ptree)
	require.Nil(t, err)
	require.NotNil(t, ast)
	return ast
}

func TestParseBuild(t *testing.T) {
	log := testlg.New(t).Strict(true)

	for test, input := range slqInputs {
		ptree, err := parseSLQ(log, input)
		require.Nil(t, err, test)
		require.NotNil(t, ptree, test)

		ast, err := buildAST(log, ptree)
		require.Nil(t, err, test)
		require.NotNil(t, ast, test)
	}
}

func TestInspector_FindWhereClauses(t *testing.T) {
	log := testlg.New(t)

	// Verify that ".uid > 4" becomes a WHERE clause.
	const input = "@my1 | .tbluser | .uid > 4 | .uid, .username"

	ptree, err := parseSLQ(log, input)
	require.Nil(t, err)
	require.NotNil(t, ptree)

	nRoot, err := buildAST(log, ptree)
	require.Nil(t, err)

	insp := NewInspector(log, nRoot)
	whereNodes, err := insp.FindWhereClauses()
	require.NoError(t, err)
	require.Len(t, whereNodes, 1)
}

package ql

import (
	"github.com/neilotoole/gotils/testing"
	"github.com/stretchr/testify/require"
)

func TestBuild2(t *testing.T) {

	p := getParser(fixtJoinRowRange)
	query := p.Query()
	ast, err := BuildAST(query)
	require.Nil(t, err)
	require.NotNil(t, ast)
	stmt, err := BuildModel(ast)
	require.Nil(t, err)
	require.NotNil(t, stmt)

}

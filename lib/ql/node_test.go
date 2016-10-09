package ql

import (
	"testing"

	"github.com/neilotoole/go-lg/lg"
	"github.com/stretchr/testify/assert"
)

func TestChildIndex(t *testing.T) {

	// `@mydb1 | .user, .address | join(.uid == .uid) | .uid, .username, .country`
	lg.Debugf("trying test child index")
	p := getParser(fixtJoinQuery1)
	query := p.Query()
	ast, err := BuildAST(query)
	assert.Nil(t, err)
	assert.NotNil(t, ast)
	assert.Equal(t, 4, len(ast.Segments()))

	for i, seg := range ast.Segments() {

		index := ChildIndex(ast, seg)
		assert.Equal(t, i, index)
	}

}

package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSegment(t *testing.T) {
	const q1 = `@mydb1 | .user  | join(.address, .user.uid == .address.uid) | .uid, .username, .country`
	ast := mustParse(t, q1)

	segs := ast.Segments()
	assert.Equal(t, 4, len(segs))

	assert.Nil(t, ast.Segments()[0].Prev(), "first segment should not have a parent")
	assert.Equal(t, ast.Segments()[0], ast.Segments()[1].Prev())
	assert.Equal(t, ast.Segments()[1], ast.Segments()[2].Prev())

	ok, err := ast.Segments()[0].uniformChildren()
	assert.Nil(t, err)
	assert.True(t, ok)

	typ, err := ast.Segments()[0].ChildType()
	assert.Nil(t, err)
	assert.NotNil(t, typ)
	assert.Equal(t, typeHandleNode.String(), typ.String())

	typ, err = ast.Segments()[1].ChildType()
	assert.Nil(t, err)
	assert.NotNil(t, typ)
	assert.Equal(t, typeTblSelectorNode.String(), typ.String())

	typ, err = ast.Segments()[2].ChildType()
	assert.Nil(t, err)
	assert.NotNil(t, typ)
	assert.Equal(t, typeJoinNode.String(), typ.String())

	typ, err = ast.Segments()[3].ChildType()
	assert.Nil(t, err)
	assert.NotNil(t, typ)
	assert.Equal(t, typeColSelectorNode.String(), typ.String())
}

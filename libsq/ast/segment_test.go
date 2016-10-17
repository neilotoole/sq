package ast

import (
	"testing"

	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/stretchr/testify/assert"
)

func TestSegment(t *testing.T) {

	// `@mydb1 | .user, .address | join(.uid == .uid) | .uid, .username, .country`
	p := getParser(FixtJoinQuery1)
	query := p.Query()
	ast, err := NewBuilder(drvr.NewSourceSet()).Build(query)
	assert.Nil(t, err)
	assert.NotNil(t, ast)

	segs := ast.Segments()

	assert.Equal(t, 4, len(segs))

	assert.Nil(t, ast.Segments()[0].Prev(), "first segment should not have a parent")
	assert.Equal(t, ast.Segments()[0], ast.Segments()[1].Prev())
	assert.Equal(t, ast.Segments()[1], ast.Segments()[2].Prev())

	ok, err := ast.Segments()[0].HasCompatibleChildren()
	assert.Nil(t, err)
	assert.True(t, ok)

	typ, err := ast.Segments()[0].ChildType()
	assert.Nil(t, err)
	assert.NotNil(t, typ)
	assert.Equal(t, TypeDatasource.String(), typ.String())
	//assert.NotNil(t, typ.(*Datasource))

	typ, err = ast.Segments()[1].ChildType()
	assert.Nil(t, err)
	assert.NotNil(t, typ)
	assert.Equal(t, TypeTableSelector.String(), typ.String())

	typ, err = ast.Segments()[2].ChildType()
	assert.Nil(t, err)
	assert.NotNil(t, typ)
	assert.Equal(t, TypeFnJoin.String(), typ.String())

	typ, err = ast.Segments()[3].ChildType()
	assert.Nil(t, err)
	assert.NotNil(t, typ)
	assert.Equal(t, TypeColSelector.String(), typ.String())

}

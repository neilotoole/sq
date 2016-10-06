package ql

import (
	"github.com/neilotoole/gotils/testing"
	"github.com/stretchr/testify/assert"
)

//func TestIR_Build(t *testing.T) {
//
//	p := getParser(fixtureSelect1)
//	query := p.Query()
//
//	//fmt.Printf("query tree: %T(%s)\n", query, query.GetText())
//
//	ir, err := Build(query)
//	assert.Nil(t, err)
//	assert.NotNil(t, ir)
//
//	assert.Equal(t, 2, len(ir.Segments), "should be two segments")
//
//	//atn := p.GetATN()
//	//assert.NotNil(t, atn)
//	//fmt.Printf("IR: %s\n", ir)
//
//}

func TestToTreeString(t *testing.T) {
	p := getParser(fixtSelect1)
	query := p.Query()
	ast, err := BuildAST(query)
	assert.Nil(t, err)
	assert.NotNil(t, ast)

	//fmt.Println(">>>>>>")
	//fmt.Println(ToTreeString(ir))
	//fmt.Println("<<<<<<")

}

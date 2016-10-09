package ql

import (
	"github.com/neilotoole/sq/lib/ql/parser"
	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

const fixtRowRange1 = `@mydb1 | .user | .uid, .username | .[]`
const fixtRowRange2 = `@mydb1 | .user | .uid, .username | .[2]`
const fixtRowRange3 = `@mydb1 | .user | .uid, .username | .[1:3]`
const fixtRowRange4 = `@mydb1 | .user | .uid, .username | .[0:3]`
const fixtRowRange5 = `@mydb1 | .user | .uid, .username | .[:3]`
const fixtRowRange6 = `@mydb1 | .user | .uid, .username | .[2:]`

const fixtJoinRowRange = `@my1 |.user, .address | join(.uid) |  .[0:4] | .user.uid, .username, .country`

const fixtJoinQuery1 = `@mydb1 | .user, .address | join(.user.uid == .address.uid) | .uid, .username, .country`
const fixtSelect1 = `@mydb1 | .user | .uid, .username`

// getParser returns a parser for the given sq query.
func getParser(query string) *parser.SQParser {
	is := antlr.NewInputStream(query)
	lex := parser.NewSQLexer(is)
	ts := antlr.NewCommonTokenStream(lex, 0)
	p := parser.NewSQParser(ts)
	return p
}

func getAST(query string) (*AST, error) {
	p := getParser(query)
	q := p.Query().(*parser.QueryContext)
	//q, ok := query.(*parser.QueryContext)
	//if !ok {
	//	return nil, errorf("unable to convert %T to *parser.QueryContext", query)
	//}

	v := &ParseTreeVisitor{}
	q.Accept(v)
	if v.Err != nil {
		return nil, v.Err
	}

	return v.ast, nil
}

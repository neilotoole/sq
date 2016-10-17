package ast

import (
	"github.com/neilotoole/sq/libsq/slq"
	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

const FixtRowRange1 = `@mydb1 | .user | .uid, .username | .[]`
const FixtRowRange2 = `@mydb1 | .user | .uid, .username | .[2]`
const FixtRowRange3 = `@mydb1 | .user | .uid, .username | .[1:3]`
const FixtRowRange4 = `@mydb1 | .user | .uid, .username | .[0:3]`
const FixtRowRange5 = `@mydb1 | .user | .uid, .username | .[:3]`
const FixtRowRange6 = `@mydb1 | .user | .uid, .username | .[2:]`

const FixtJoinRowRange = `@my1 |.user, .address | join(.uid) |  .[0:4] | .user.uid, .username, .country`

const FixtJoinQuery1 = `@mydb1 | .user, .address | join(.user.uid == .address.uid) | .uid, .username, .country`
const FixtSelect1 = `@mydb1 | .user | .uid, .username`

// getParser returns a parser for the given sq query.
func getParser(query string) *slq.SLQParser {
	is := antlr.NewInputStream(query)
	lex := slq.NewSLQLexer(is)
	ts := antlr.NewCommonTokenStream(lex, 0)
	p := slq.NewSLQParser(ts)
	return p
}

func getAST(query string) (*AST, error) {
	p := getParser(query)
	q := p.Query().(*slq.QueryContext)
	v := &ParseTreeVisitor{}
	q.Accept(v)
	if v.Err != nil {
		return nil, v.Err
	}

	return v.ast, nil
}

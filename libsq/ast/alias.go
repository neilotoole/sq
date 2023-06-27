package ast

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

// VisitAlias implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitAlias(ctx *slq.AliasContext) any {
	if ctx == nil || ctx.ID() == nil && ctx.GetText() == "" {
		return nil
	}

	var alias string
	if ctx.ID() != nil {
		alias = ctx.ID().GetText()
	} else if ctx.STRING() != nil {
		alias = stringz.StripDoubleQuote(ctx.STRING().GetText())
	}

	switch node := v.cur.(type) {
	case *SelectorNode:
		node.alias = alias
	case *TblSelectorNode:
		node.alias = alias
	case *ExprElementNode:
		node.alias = alias
	case *FuncNode:
		if alias != "" {
			node.alias = alias
			return nil
		}

		// NOTE: The grammar has a dodgy hack to deal with no-arg funcs
		// with an alias that is a reserved word.
		//
		// For example, let's start with this snippet. Note that "count" is
		// a function, equivalent to count().
		//
		//   .actor | count
		//
		// Then add an alias that is a reserved word, such as a function name.
		// In this example, we will use an alias of "count" as well.
		//
		//   .actor | count:count
		//
		// Well, the grammar doesn't know how to handle this. Most likely the
		// grammar could be refactored to deal with this more gracefully. The
		// hack is to look at the full text of the context (e.g. ":count"),
		// instead of just ID, and look for the alias after the colon.

		text := ctx.GetText()
		node.alias = strings.TrimPrefix(text, ":")

	default:
		return errorf("alias not allowed for type %T: %v", node, ctx.GetText())
	}

	return nil
}

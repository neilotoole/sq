package ast

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// extractAliasValue returns the alias value from ctx, handling every token
// shape permitted by the grammar's alias rule:
//
//	alias: ALIAS_RESERVED | ':' (ARG | ID | STRING)
//
// The shapes are:
//
//   - ID: e.g. ":given_name" yields "given_name".
//   - STRING: e.g. a double-quoted ":<...>" yields the unquoted text.
//   - ALIAS_RESERVED: e.g. ":count" yields "count". ALIAS_RESERVED is a
//     pre-baked ":keyword" token (e.g. ":count") that the grammar uses to
//     work around the lexer otherwise tokenizing the keyword; the leading
//     colon is trimmed to recover the name.
//   - ARG: e.g. ":$x" is rejected. An ARG (e.g. "$x") is a parameter
//     reference, not a name, and sq does not substitute it in the alias
//     position. Rather than silently produce a literal "$x" column, it is
//     reported as an error.
//
// It returns ("", nil) when ctx is nil or carries no alias token.
func extractAliasValue(ctx slq.IAliasContext) (string, error) {
	if ctx == nil {
		return "", nil
	}

	switch {
	case ctx.ID() != nil:
		return ctx.ID().GetText(), nil
	case ctx.STRING() != nil:
		return stringz.StripDoubleQuote(ctx.STRING().GetText()), nil
	case ctx.ALIAS_RESERVED() != nil:
		return strings.TrimPrefix(ctx.ALIAS_RESERVED().GetText(), ":"), nil
	case ctx.ARG() != nil:
		return "", errorf("alias may not be an argument reference: %s", ctx.ARG().GetText())
	default:
		return "", nil
	}
}

// VisitAlias implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitAlias(ctx *slq.AliasContext) any {
	if ctx == nil {
		return nil
	}

	alias, err := extractAliasValue(ctx)
	if err != nil {
		return err
	}

	switch node := v.cur.(type) {
	case *SelectorNode:
		node.alias = alias
	case *TblSelectorNode:
		node.alias = alias
	case *ExprElementNode:
		node.alias = alias
	case *FuncNode:
		node.alias = alias
	default:
		return errorf("alias not allowed for type %T: %v", node, ctx.GetText())
	}

	return nil
}

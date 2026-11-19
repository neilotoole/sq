package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The two rule function endings ANTLR 4.13.0 emits: most rules end with
// p.ExitRule(), left-recursive rules with p.UnrollRecursionContexts.
const (
	generatedExitRule = `func (p *SLQParser) Query() (localctx IQueryContext) {
	if p.HasError() {
		goto errorExit
	}

errorExit:
	if p.HasError() {
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}
`

	generatedUnroll = `func (p *SLQParser) expr(_p int) (localctx IExprContext) {
	if p.HasError() {
		goto errorExit
	}

errorExit:
	if p.HasError() {
		p.SetError(nil)
	}
	p.UnrollRecursionContexts(_parentctx)
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}
`

	rewrittenExitRule = `func (p *SLQParser) Query() (localctx IQueryContext) {
	if p.HasError() {
		goto errorExit
	}

errorExit:
	if p.HasError() {
		p.SetError(nil)
	}
	p.ExitRule()
	if false {
		goto errorExit // Trick to prevent compiler error if the label is not used
	}
	return localctx
}
`

	rewrittenUnroll = `func (p *SLQParser) expr(_p int) (localctx IExprContext) {
	if p.HasError() {
		goto errorExit
	}

errorExit:
	if p.HasError() {
		p.SetError(nil)
	}
	p.UnrollRecursionContexts(_parentctx)
	if false {
		goto errorExit // Trick to prevent compiler error if the label is not used
	}
	return localctx
}
`
)

func TestRewrite(t *testing.T) {
	got, err := rewrite([]byte(generatedExitRule + "\n" + generatedUnroll))
	require.NoError(t, err)
	require.Equal(t, rewrittenExitRule+"\n"+rewrittenUnroll, string(got))
}

func TestRewrite_errors(t *testing.T) {
	testCases := []struct {
		name string
		src  string
	}{
		{
			// A future ANTLR template drops or changes the trick in one rule.
			name: "rule_without_trick",
			src:  generatedExitRule + "\n" + rewrittenUnroll,
		},
		{
			// Already rewritten, or the template changed for every rule.
			name: "no_tricks",
			src:  rewrittenExitRule,
		},
		{
			// Not a parser file, e.g. the lexer.
			name: "no_rule_functions",
			src:  "package slq\n\nfunc NewSLQLexer() {}\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rewrite([]byte(tc.src))
			require.Error(t, err)
		})
	}
}

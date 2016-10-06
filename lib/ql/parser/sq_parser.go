// Generated from ../grammar/SQ.g4 by ANTLR 4.5.1.

package parser // SQ
import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

// Stopgap to suppress unused import error. We aren't certain
// to have these imports used in the generated code below

var _ = fmt.Printf
var _ = reflect.Copy
var _ = strconv.Itoa

var parserATN = []uint16{
	3, 1072, 54993, 33286, 44333, 17431, 44785, 36224, 43741, 3, 30, 104, 4,
	2, 9, 2, 4, 3, 9, 3, 4, 4, 9, 4, 4, 5, 9, 5, 4, 6, 9, 6, 4, 7, 9, 7, 4,
	8, 9, 8, 4, 9, 9, 9, 4, 10, 9, 10, 4, 11, 9, 11, 4, 12, 9, 12, 4, 13, 9,
	13, 4, 14, 9, 14, 4, 15, 9, 15, 3, 2, 3, 2, 3, 2, 7, 2, 34, 10, 2, 12,
	2, 14, 2, 37, 11, 2, 3, 3, 3, 3, 3, 3, 7, 3, 42, 10, 3, 12, 3, 14, 3, 45,
	11, 3, 3, 4, 3, 4, 3, 4, 3, 4, 3, 4, 5, 4, 52, 10, 4, 3, 5, 3, 5, 3, 6,
	3, 6, 3, 7, 3, 7, 3, 7, 7, 7, 61, 10, 7, 12, 7, 14, 7, 64, 11, 7, 5, 7,
	66, 10, 7, 3, 8, 3, 8, 3, 9, 3, 9, 3, 9, 3, 9, 3, 9, 3, 10, 3, 10, 3, 10,
	3, 10, 3, 11, 3, 11, 5, 11, 81, 10, 11, 3, 12, 3, 12, 3, 13, 3, 13, 3,
	13, 3, 14, 3, 14, 3, 15, 3, 15, 3, 15, 3, 15, 3, 15, 3, 15, 3, 15, 3, 15,
	3, 15, 3, 15, 5, 15, 100, 10, 15, 3, 15, 3, 15, 3, 15, 2, 2, 16, 2, 4,
	6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 2, 5, 3, 2, 20, 25, 4, 2,
	7, 7, 26, 26, 3, 2, 3, 5, 102, 2, 30, 3, 2, 2, 2, 4, 38, 3, 2, 2, 2, 6,
	51, 3, 2, 2, 2, 8, 53, 3, 2, 2, 2, 10, 55, 3, 2, 2, 2, 12, 65, 3, 2, 2,
	2, 14, 67, 3, 2, 2, 2, 16, 69, 3, 2, 2, 2, 18, 74, 3, 2, 2, 2, 20, 80,
	3, 2, 2, 2, 22, 82, 3, 2, 2, 2, 24, 84, 3, 2, 2, 2, 26, 87, 3, 2, 2, 2,
	28, 89, 3, 2, 2, 2, 30, 35, 5, 4, 3, 2, 31, 32, 7, 14, 2, 2, 32, 34, 5,
	4, 3, 2, 33, 31, 3, 2, 2, 2, 34, 37, 3, 2, 2, 2, 35, 33, 3, 2, 2, 2, 35,
	36, 3, 2, 2, 2, 36, 3, 3, 2, 2, 2, 37, 35, 3, 2, 2, 2, 38, 43, 5, 6, 4,
	2, 39, 40, 7, 13, 2, 2, 40, 42, 5, 6, 4, 2, 41, 39, 3, 2, 2, 2, 42, 45,
	3, 2, 2, 2, 43, 41, 3, 2, 2, 2, 43, 44, 3, 2, 2, 2, 44, 5, 3, 2, 2, 2,
	45, 43, 3, 2, 2, 2, 46, 52, 5, 24, 13, 2, 47, 52, 5, 26, 14, 2, 48, 52,
	5, 22, 12, 2, 49, 52, 5, 10, 6, 2, 50, 52, 5, 28, 15, 2, 51, 46, 3, 2,
	2, 2, 51, 47, 3, 2, 2, 2, 51, 48, 3, 2, 2, 2, 51, 49, 3, 2, 2, 2, 51, 50,
	3, 2, 2, 2, 52, 7, 3, 2, 2, 2, 53, 54, 9, 2, 2, 2, 54, 9, 3, 2, 2, 2, 55,
	56, 5, 16, 9, 2, 56, 11, 3, 2, 2, 2, 57, 62, 5, 14, 8, 2, 58, 59, 7, 13,
	2, 2, 59, 61, 5, 14, 8, 2, 60, 58, 3, 2, 2, 2, 61, 64, 3, 2, 2, 2, 62,
	60, 3, 2, 2, 2, 62, 63, 3, 2, 2, 2, 63, 66, 3, 2, 2, 2, 64, 62, 3, 2, 2,
	2, 65, 57, 3, 2, 2, 2, 65, 66, 3, 2, 2, 2, 66, 13, 3, 2, 2, 2, 67, 68,
	9, 3, 2, 2, 68, 15, 3, 2, 2, 2, 69, 70, 9, 4, 2, 2, 70, 71, 7, 9, 2, 2,
	71, 72, 5, 20, 11, 2, 72, 73, 7, 10, 2, 2, 73, 17, 3, 2, 2, 2, 74, 75,
	7, 26, 2, 2, 75, 76, 5, 8, 5, 2, 76, 77, 7, 26, 2, 2, 77, 19, 3, 2, 2,
	2, 78, 81, 5, 18, 10, 2, 79, 81, 7, 26, 2, 2, 80, 78, 3, 2, 2, 2, 80, 79,
	3, 2, 2, 2, 81, 21, 3, 2, 2, 2, 82, 83, 7, 26, 2, 2, 83, 23, 3, 2, 2, 2,
	84, 85, 7, 27, 2, 2, 85, 86, 7, 26, 2, 2, 86, 25, 3, 2, 2, 2, 87, 88, 7,
	27, 2, 2, 88, 27, 3, 2, 2, 2, 89, 99, 7, 6, 2, 2, 90, 91, 7, 18, 2, 2,
	91, 92, 7, 15, 2, 2, 92, 100, 7, 18, 2, 2, 93, 94, 7, 18, 2, 2, 94, 100,
	7, 15, 2, 2, 95, 96, 7, 15, 2, 2, 96, 100, 7, 18, 2, 2, 97, 100, 7, 18,
	2, 2, 98, 100, 3, 2, 2, 2, 99, 90, 3, 2, 2, 2, 99, 93, 3, 2, 2, 2, 99,
	95, 3, 2, 2, 2, 99, 97, 3, 2, 2, 2, 99, 98, 3, 2, 2, 2, 100, 101, 3, 2,
	2, 2, 101, 102, 7, 12, 2, 2, 102, 29, 3, 2, 2, 2, 9, 35, 43, 51, 62, 65,
	80, 99,
}

var deserializer = antlr.NewATNDeserializer(nil)

var deserializedATN = deserializer.DeserializeFromUInt16(parserATN)

var literalNames = []string{
	"", "'join'", "'JOIN'", "'j'", "'.['", "", "", "'('", "')'", "'['", "']'",
	"','", "'|'", "':'", "", "", "", "", "'<='", "'<'", "'>='", "'>'", "'!='",
	"'=='", "", "", "'.'",
}

var symbolicNames = []string{
	"", "", "", "", "", "ID", "WS", "LPAR", "RPAR", "LBRA", "RBRA", "COMMA",
	"PIPE", "COLON", "NULL", "STRING", "INT", "NUMBER", "LT_EQ", "LT", "GT_EQ",
	"GT", "NEQ", "EQ", "SEL", "DATASOURCE", "DOT", "VAL", "LINECOMMENT",
}

var ruleNames = []string{
	"query", "segment", "element", "cmpr", "fn", "args", "arg", "fnJoin", "fnJoinCond",
	"fnJoinExpr", "selElement", "dsTblElement", "dsElement", "rowRange",
}

type SQParser struct {
	*antlr.BaseParser
}

func NewSQParser(input antlr.TokenStream) *SQParser {
	var decisionToDFA = make([]*antlr.DFA, len(deserializedATN.DecisionToState))
	var sharedContextCache = antlr.NewPredictionContextCache()

	for index, ds := range deserializedATN.DecisionToState {
		decisionToDFA[index] = antlr.NewDFA(ds, index)
	}

	this := new(SQParser)

	this.BaseParser = antlr.NewBaseParser(input)

	this.Interpreter = antlr.NewParserATNSimulator(this, deserializedATN, decisionToDFA, sharedContextCache)
	this.RuleNames = ruleNames
	this.LiteralNames = literalNames
	this.SymbolicNames = symbolicNames
	this.GrammarFileName = "SQ.g4"

	return this
}

// SQParser tokens.
const (
	SQParserEOF         = antlr.TokenEOF
	SQParserT__0        = 1
	SQParserT__1        = 2
	SQParserT__2        = 3
	SQParserT__3        = 4
	SQParserID          = 5
	SQParserWS          = 6
	SQParserLPAR        = 7
	SQParserRPAR        = 8
	SQParserLBRA        = 9
	SQParserRBRA        = 10
	SQParserCOMMA       = 11
	SQParserPIPE        = 12
	SQParserCOLON       = 13
	SQParserNULL        = 14
	SQParserSTRING      = 15
	SQParserINT         = 16
	SQParserNUMBER      = 17
	SQParserLT_EQ       = 18
	SQParserLT          = 19
	SQParserGT_EQ       = 20
	SQParserGT          = 21
	SQParserNEQ         = 22
	SQParserEQ          = 23
	SQParserSEL         = 24
	SQParserDATASOURCE  = 25
	SQParserDOT         = 26
	SQParserVAL         = 27
	SQParserLINECOMMENT = 28
)

// SQParser rules.
const (
	SQParserRULE_query        = 0
	SQParserRULE_segment      = 1
	SQParserRULE_element      = 2
	SQParserRULE_cmpr         = 3
	SQParserRULE_fn           = 4
	SQParserRULE_args         = 5
	SQParserRULE_arg          = 6
	SQParserRULE_fnJoin       = 7
	SQParserRULE_fnJoinCond   = 8
	SQParserRULE_fnJoinExpr   = 9
	SQParserRULE_selElement   = 10
	SQParserRULE_dsTblElement = 11
	SQParserRULE_dsElement    = 12
	SQParserRULE_rowRange     = 13
)

// IQueryContext is an interface to support dynamic dispatch.
type IQueryContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getQueryContext differentiates from other interfaces.
	getQueryContext()
}

type QueryContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyQueryContext() *QueryContext {
	var p = new(QueryContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_query
	return p
}

func (*QueryContext) getQueryContext() {}

func NewQueryContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *QueryContext {
	var p = new(QueryContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_query

	return p
}

func (s *QueryContext) GetParser() antlr.Parser { return s.parser }

func (s *QueryContext) AllSegment() []ISegmentContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*ISegmentContext)(nil)).Elem())
	var tst = make([]ISegmentContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(ISegmentContext)
		}
	}

	return tst
}

func (s *QueryContext) Segment(i int) ISegmentContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ISegmentContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(ISegmentContext)
}

func (s *QueryContext) AllPIPE() []antlr.TerminalNode {
	return s.GetTokens(SQParserPIPE)
}

func (s *QueryContext) PIPE(i int) antlr.TerminalNode {
	return s.GetToken(SQParserPIPE, i)
}

func (s *QueryContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *QueryContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterQuery(s)
	}
}

func (s *QueryContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitQuery(s)
	}
}

func (s *QueryContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitQuery(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) Query() (localctx IQueryContext) {
	localctx = NewQueryContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 0, SQParserRULE_query)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(28)
	p.Segment()

	p.SetState(33)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SQParserPIPE {
		p.SetState(29)
		p.Match(SQParserPIPE)

		p.SetState(30)
		p.Segment()

		p.SetState(35)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}

// ISegmentContext is an interface to support dynamic dispatch.
type ISegmentContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getSegmentContext differentiates from other interfaces.
	getSegmentContext()
}

type SegmentContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySegmentContext() *SegmentContext {
	var p = new(SegmentContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_segment
	return p
}

func (*SegmentContext) getSegmentContext() {}

func NewSegmentContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SegmentContext {
	var p = new(SegmentContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_segment

	return p
}

func (s *SegmentContext) GetParser() antlr.Parser { return s.parser }

func (s *SegmentContext) AllElement() []IElementContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IElementContext)(nil)).Elem())
	var tst = make([]IElementContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IElementContext)
		}
	}

	return tst
}

func (s *SegmentContext) Element(i int) IElementContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IElementContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IElementContext)
}

func (s *SegmentContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(SQParserCOMMA)
}

func (s *SegmentContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(SQParserCOMMA, i)
}

func (s *SegmentContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SegmentContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterSegment(s)
	}
}

func (s *SegmentContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitSegment(s)
	}
}

func (s *SegmentContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitSegment(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) Segment() (localctx ISegmentContext) {
	localctx = NewSegmentContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 2, SQParserRULE_segment)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(36)
	p.Element()

	p.SetState(41)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SQParserCOMMA {
		p.SetState(37)
		p.Match(SQParserCOMMA)

		p.SetState(38)
		p.Element()

		p.SetState(43)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}

// IElementContext is an interface to support dynamic dispatch.
type IElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getElementContext differentiates from other interfaces.
	getElementContext()
}

type ElementContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyElementContext() *ElementContext {
	var p = new(ElementContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_element
	return p
}

func (*ElementContext) getElementContext() {}

func NewElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ElementContext {
	var p = new(ElementContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_element

	return p
}

func (s *ElementContext) GetParser() antlr.Parser { return s.parser }

func (s *ElementContext) DsTblElement() IDsTblElementContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IDsTblElementContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IDsTblElementContext)
}

func (s *ElementContext) DsElement() IDsElementContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IDsElementContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IDsElementContext)
}

func (s *ElementContext) SelElement() ISelElementContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ISelElementContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(ISelElementContext)
}

func (s *ElementContext) Fn() IFnContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IFnContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IFnContext)
}

func (s *ElementContext) RowRange() IRowRangeContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IRowRangeContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IRowRangeContext)
}

func (s *ElementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterElement(s)
	}
}

func (s *ElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitElement(s)
	}
}

func (s *ElementContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitElement(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) Element() (localctx IElementContext) {
	localctx = NewElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 4, SQParserRULE_element)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.SetState(49)
	p.GetErrorHandler().Sync(p)

	la_ := p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 2, p.GetParserRuleContext())

	switch la_ {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		p.SetState(44)
		p.DsTblElement()

	case 2:
		p.EnterOuterAlt(localctx, 2)
		p.SetState(45)
		p.DsElement()

	case 3:
		p.EnterOuterAlt(localctx, 3)
		p.SetState(46)
		p.SelElement()

	case 4:
		p.EnterOuterAlt(localctx, 4)
		p.SetState(47)
		p.Fn()

	case 5:
		p.EnterOuterAlt(localctx, 5)
		p.SetState(48)
		p.RowRange()

	}

	return localctx
}

// ICmprContext is an interface to support dynamic dispatch.
type ICmprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getCmprContext differentiates from other interfaces.
	getCmprContext()
}

type CmprContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyCmprContext() *CmprContext {
	var p = new(CmprContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_cmpr
	return p
}

func (*CmprContext) getCmprContext() {}

func NewCmprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *CmprContext {
	var p = new(CmprContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_cmpr

	return p
}

func (s *CmprContext) GetParser() antlr.Parser { return s.parser }

func (s *CmprContext) LT_EQ() antlr.TerminalNode {
	return s.GetToken(SQParserLT_EQ, 0)
}

func (s *CmprContext) LT() antlr.TerminalNode {
	return s.GetToken(SQParserLT, 0)
}

func (s *CmprContext) GT_EQ() antlr.TerminalNode {
	return s.GetToken(SQParserGT_EQ, 0)
}

func (s *CmprContext) GT() antlr.TerminalNode {
	return s.GetToken(SQParserGT, 0)
}

func (s *CmprContext) EQ() antlr.TerminalNode {
	return s.GetToken(SQParserEQ, 0)
}

func (s *CmprContext) NEQ() antlr.TerminalNode {
	return s.GetToken(SQParserNEQ, 0)
}

func (s *CmprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *CmprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterCmpr(s)
	}
}

func (s *CmprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitCmpr(s)
	}
}

func (s *CmprContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitCmpr(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) Cmpr() (localctx ICmprContext) {
	localctx = NewCmprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 6, SQParserRULE_cmpr)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(51)
	_la = p.GetTokenStream().LA(1)

	if !(((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<SQParserLT_EQ)|(1<<SQParserLT)|(1<<SQParserGT_EQ)|(1<<SQParserGT)|(1<<SQParserNEQ)|(1<<SQParserEQ))) != 0) {
		p.GetErrorHandler().RecoverInline(p)
	} else {
		p.Consume()
	}

	return localctx
}

// IFnContext is an interface to support dynamic dispatch.
type IFnContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getFnContext differentiates from other interfaces.
	getFnContext()
}

type FnContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFnContext() *FnContext {
	var p = new(FnContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_fn
	return p
}

func (*FnContext) getFnContext() {}

func NewFnContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FnContext {
	var p = new(FnContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_fn

	return p
}

func (s *FnContext) GetParser() antlr.Parser { return s.parser }

func (s *FnContext) FnJoin() IFnJoinContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IFnJoinContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IFnJoinContext)
}

func (s *FnContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FnContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterFn(s)
	}
}

func (s *FnContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitFn(s)
	}
}

func (s *FnContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitFn(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) Fn() (localctx IFnContext) {
	localctx = NewFnContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 8, SQParserRULE_fn)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(53)
	p.FnJoin()

	return localctx
}

// IArgsContext is an interface to support dynamic dispatch.
type IArgsContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getArgsContext differentiates from other interfaces.
	getArgsContext()
}

type ArgsContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyArgsContext() *ArgsContext {
	var p = new(ArgsContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_args
	return p
}

func (*ArgsContext) getArgsContext() {}

func NewArgsContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ArgsContext {
	var p = new(ArgsContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_args

	return p
}

func (s *ArgsContext) GetParser() antlr.Parser { return s.parser }

func (s *ArgsContext) AllArg() []IArgContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IArgContext)(nil)).Elem())
	var tst = make([]IArgContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IArgContext)
		}
	}

	return tst
}

func (s *ArgsContext) Arg(i int) IArgContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IArgContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IArgContext)
}

func (s *ArgsContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(SQParserCOMMA)
}

func (s *ArgsContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(SQParserCOMMA, i)
}

func (s *ArgsContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ArgsContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterArgs(s)
	}
}

func (s *ArgsContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitArgs(s)
	}
}

func (s *ArgsContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitArgs(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) Args() (localctx IArgsContext) {
	localctx = NewArgsContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 10, SQParserRULE_args)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(63)
	_la = p.GetTokenStream().LA(1)

	if _la == SQParserID || _la == SQParserSEL {
		p.SetState(55)
		p.Arg()

		p.SetState(60)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		for _la == SQParserCOMMA {
			p.SetState(56)
			p.Match(SQParserCOMMA)

			p.SetState(57)
			p.Arg()

			p.SetState(62)
			p.GetErrorHandler().Sync(p)
			_la = p.GetTokenStream().LA(1)
		}

	}

	return localctx
}

// IArgContext is an interface to support dynamic dispatch.
type IArgContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getArgContext differentiates from other interfaces.
	getArgContext()
}

type ArgContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyArgContext() *ArgContext {
	var p = new(ArgContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_arg
	return p
}

func (*ArgContext) getArgContext() {}

func NewArgContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ArgContext {
	var p = new(ArgContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_arg

	return p
}

func (s *ArgContext) GetParser() antlr.Parser { return s.parser }

func (s *ArgContext) SEL() antlr.TerminalNode {
	return s.GetToken(SQParserSEL, 0)
}

func (s *ArgContext) ID() antlr.TerminalNode {
	return s.GetToken(SQParserID, 0)
}

func (s *ArgContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ArgContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterArg(s)
	}
}

func (s *ArgContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitArg(s)
	}
}

func (s *ArgContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitArg(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) Arg() (localctx IArgContext) {
	localctx = NewArgContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, SQParserRULE_arg)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(65)
	_la = p.GetTokenStream().LA(1)

	if !(_la == SQParserID || _la == SQParserSEL) {
		p.GetErrorHandler().RecoverInline(p)
	} else {
		p.Consume()
	}

	return localctx
}

// IFnJoinContext is an interface to support dynamic dispatch.
type IFnJoinContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getFnJoinContext differentiates from other interfaces.
	getFnJoinContext()
}

type FnJoinContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFnJoinContext() *FnJoinContext {
	var p = new(FnJoinContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_fnJoin
	return p
}

func (*FnJoinContext) getFnJoinContext() {}

func NewFnJoinContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FnJoinContext {
	var p = new(FnJoinContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_fnJoin

	return p
}

func (s *FnJoinContext) GetParser() antlr.Parser { return s.parser }

func (s *FnJoinContext) LPAR() antlr.TerminalNode {
	return s.GetToken(SQParserLPAR, 0)
}

func (s *FnJoinContext) FnJoinExpr() IFnJoinExprContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IFnJoinExprContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IFnJoinExprContext)
}

func (s *FnJoinContext) RPAR() antlr.TerminalNode {
	return s.GetToken(SQParserRPAR, 0)
}

func (s *FnJoinContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FnJoinContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterFnJoin(s)
	}
}

func (s *FnJoinContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitFnJoin(s)
	}
}

func (s *FnJoinContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitFnJoin(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) FnJoin() (localctx IFnJoinContext) {
	localctx = NewFnJoinContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 14, SQParserRULE_fnJoin)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(67)
	_la = p.GetTokenStream().LA(1)

	if !(((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<SQParserT__0)|(1<<SQParserT__1)|(1<<SQParserT__2))) != 0) {
		p.GetErrorHandler().RecoverInline(p)
	} else {
		p.Consume()
	}
	p.SetState(68)
	p.Match(SQParserLPAR)

	p.SetState(69)
	p.FnJoinExpr()

	p.SetState(70)
	p.Match(SQParserRPAR)

	return localctx
}

// IFnJoinCondContext is an interface to support dynamic dispatch.
type IFnJoinCondContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getFnJoinCondContext differentiates from other interfaces.
	getFnJoinCondContext()
}

type FnJoinCondContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFnJoinCondContext() *FnJoinCondContext {
	var p = new(FnJoinCondContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_fnJoinCond
	return p
}

func (*FnJoinCondContext) getFnJoinCondContext() {}

func NewFnJoinCondContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FnJoinCondContext {
	var p = new(FnJoinCondContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_fnJoinCond

	return p
}

func (s *FnJoinCondContext) GetParser() antlr.Parser { return s.parser }

func (s *FnJoinCondContext) AllSEL() []antlr.TerminalNode {
	return s.GetTokens(SQParserSEL)
}

func (s *FnJoinCondContext) SEL(i int) antlr.TerminalNode {
	return s.GetToken(SQParserSEL, i)
}

func (s *FnJoinCondContext) Cmpr() ICmprContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ICmprContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(ICmprContext)
}

func (s *FnJoinCondContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FnJoinCondContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterFnJoinCond(s)
	}
}

func (s *FnJoinCondContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitFnJoinCond(s)
	}
}

func (s *FnJoinCondContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitFnJoinCond(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) FnJoinCond() (localctx IFnJoinCondContext) {
	localctx = NewFnJoinCondContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 16, SQParserRULE_fnJoinCond)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(72)
	p.Match(SQParserSEL)

	p.SetState(73)
	p.Cmpr()

	p.SetState(74)
	p.Match(SQParserSEL)

	return localctx
}

// IFnJoinExprContext is an interface to support dynamic dispatch.
type IFnJoinExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getFnJoinExprContext differentiates from other interfaces.
	getFnJoinExprContext()
}

type FnJoinExprContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFnJoinExprContext() *FnJoinExprContext {
	var p = new(FnJoinExprContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_fnJoinExpr
	return p
}

func (*FnJoinExprContext) getFnJoinExprContext() {}

func NewFnJoinExprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FnJoinExprContext {
	var p = new(FnJoinExprContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_fnJoinExpr

	return p
}

func (s *FnJoinExprContext) GetParser() antlr.Parser { return s.parser }

func (s *FnJoinExprContext) FnJoinCond() IFnJoinCondContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IFnJoinCondContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IFnJoinCondContext)
}

func (s *FnJoinExprContext) SEL() antlr.TerminalNode {
	return s.GetToken(SQParserSEL, 0)
}

func (s *FnJoinExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FnJoinExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterFnJoinExpr(s)
	}
}

func (s *FnJoinExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitFnJoinExpr(s)
	}
}

func (s *FnJoinExprContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitFnJoinExpr(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) FnJoinExpr() (localctx IFnJoinExprContext) {
	localctx = NewFnJoinExprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 18, SQParserRULE_fnJoinExpr)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.SetState(78)
	p.GetErrorHandler().Sync(p)

	la_ := p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 5, p.GetParserRuleContext())

	switch la_ {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		p.SetState(76)
		p.FnJoinCond()

	case 2:
		p.EnterOuterAlt(localctx, 2)
		p.SetState(77)
		p.Match(SQParserSEL)

	}

	return localctx
}

// ISelElementContext is an interface to support dynamic dispatch.
type ISelElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getSelElementContext differentiates from other interfaces.
	getSelElementContext()
}

type SelElementContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySelElementContext() *SelElementContext {
	var p = new(SelElementContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_selElement
	return p
}

func (*SelElementContext) getSelElementContext() {}

func NewSelElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SelElementContext {
	var p = new(SelElementContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_selElement

	return p
}

func (s *SelElementContext) GetParser() antlr.Parser { return s.parser }

func (s *SelElementContext) SEL() antlr.TerminalNode {
	return s.GetToken(SQParserSEL, 0)
}

func (s *SelElementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SelElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterSelElement(s)
	}
}

func (s *SelElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitSelElement(s)
	}
}

func (s *SelElementContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitSelElement(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) SelElement() (localctx ISelElementContext) {
	localctx = NewSelElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 20, SQParserRULE_selElement)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(80)
	p.Match(SQParserSEL)

	return localctx
}

// IDsTblElementContext is an interface to support dynamic dispatch.
type IDsTblElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getDsTblElementContext differentiates from other interfaces.
	getDsTblElementContext()
}

type DsTblElementContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyDsTblElementContext() *DsTblElementContext {
	var p = new(DsTblElementContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_dsTblElement
	return p
}

func (*DsTblElementContext) getDsTblElementContext() {}

func NewDsTblElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *DsTblElementContext {
	var p = new(DsTblElementContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_dsTblElement

	return p
}

func (s *DsTblElementContext) GetParser() antlr.Parser { return s.parser }

func (s *DsTblElementContext) DATASOURCE() antlr.TerminalNode {
	return s.GetToken(SQParserDATASOURCE, 0)
}

func (s *DsTblElementContext) SEL() antlr.TerminalNode {
	return s.GetToken(SQParserSEL, 0)
}

func (s *DsTblElementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DsTblElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterDsTblElement(s)
	}
}

func (s *DsTblElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitDsTblElement(s)
	}
}

func (s *DsTblElementContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitDsTblElement(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) DsTblElement() (localctx IDsTblElementContext) {
	localctx = NewDsTblElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 22, SQParserRULE_dsTblElement)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(82)
	p.Match(SQParserDATASOURCE)

	p.SetState(83)
	p.Match(SQParserSEL)

	return localctx
}

// IDsElementContext is an interface to support dynamic dispatch.
type IDsElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getDsElementContext differentiates from other interfaces.
	getDsElementContext()
}

type DsElementContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyDsElementContext() *DsElementContext {
	var p = new(DsElementContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_dsElement
	return p
}

func (*DsElementContext) getDsElementContext() {}

func NewDsElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *DsElementContext {
	var p = new(DsElementContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_dsElement

	return p
}

func (s *DsElementContext) GetParser() antlr.Parser { return s.parser }

func (s *DsElementContext) DATASOURCE() antlr.TerminalNode {
	return s.GetToken(SQParserDATASOURCE, 0)
}

func (s *DsElementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DsElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterDsElement(s)
	}
}

func (s *DsElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitDsElement(s)
	}
}

func (s *DsElementContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitDsElement(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) DsElement() (localctx IDsElementContext) {
	localctx = NewDsElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 24, SQParserRULE_dsElement)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(85)
	p.Match(SQParserDATASOURCE)

	return localctx
}

// IRowRangeContext is an interface to support dynamic dispatch.
type IRowRangeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// getRowRangeContext differentiates from other interfaces.
	getRowRangeContext()
}

type RowRangeContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyRowRangeContext() *RowRangeContext {
	var p = new(RowRangeContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SQParserRULE_rowRange
	return p
}

func (*RowRangeContext) getRowRangeContext() {}

func NewRowRangeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *RowRangeContext {
	var p = new(RowRangeContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SQParserRULE_rowRange

	return p
}

func (s *RowRangeContext) GetParser() antlr.Parser { return s.parser }

func (s *RowRangeContext) AllINT() []antlr.TerminalNode {
	return s.GetTokens(SQParserINT)
}

func (s *RowRangeContext) INT(i int) antlr.TerminalNode {
	return s.GetToken(SQParserINT, i)
}

func (s *RowRangeContext) COLON() antlr.TerminalNode {
	return s.GetToken(SQParserCOLON, 0)
}

func (s *RowRangeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *RowRangeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.EnterRowRange(s)
	}
}

func (s *RowRangeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SQListener); ok {
		listenerT.ExitRowRange(s)
	}
}

func (s *RowRangeContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SQVisitor:
		return t.VisitRowRange(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SQParser) RowRange() (localctx IRowRangeContext) {
	localctx = NewRowRangeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 26, SQParserRULE_rowRange)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	p.SetState(87)
	p.Match(SQParserT__3)

	p.SetState(97)
	p.GetErrorHandler().Sync(p)

	la_ := p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 6, p.GetParserRuleContext())

	switch la_ {
	case 1:
		p.SetState(88)
		p.Match(SQParserINT)

		p.SetState(89)
		p.Match(SQParserCOLON)

		p.SetState(90)
		p.Match(SQParserINT)

	case 2:
		p.SetState(91)
		p.Match(SQParserINT)

		p.SetState(92)
		p.Match(SQParserCOLON)

	case 3:
		p.SetState(93)
		p.Match(SQParserCOLON)

		p.SetState(94)
		p.Match(SQParserINT)

	case 4:
		p.SetState(95)
		p.Match(SQParserINT)

	case 5:

	}
	p.SetState(99)
	p.Match(SQParserRBRA)

	return localctx
}

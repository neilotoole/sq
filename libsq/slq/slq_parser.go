// Generated from ../grammar/SLQ.g4 by ANTLR 4.5.3.

package slq // SLQ
import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

// Suppress unused import errors
var _ = fmt.Printf
var _ = reflect.Copy
var _ = strconv.Itoa

var parserATN = []uint16{
	3, 1072, 54993, 33286, 44333, 17431, 44785, 36224, 43741, 3, 30, 97, 4,
	2, 9, 2, 4, 3, 9, 3, 4, 4, 9, 4, 4, 5, 9, 5, 4, 6, 9, 6, 4, 7, 9, 7, 4,
	8, 9, 8, 4, 9, 9, 9, 4, 10, 9, 10, 4, 11, 9, 11, 4, 12, 9, 12, 4, 13, 9,
	13, 3, 2, 3, 2, 3, 2, 7, 2, 30, 10, 2, 12, 2, 14, 2, 33, 11, 2, 3, 3, 3,
	3, 3, 3, 7, 3, 38, 10, 3, 12, 3, 14, 3, 41, 11, 3, 3, 4, 3, 4, 3, 4, 3,
	4, 3, 4, 5, 4, 48, 10, 4, 3, 5, 3, 5, 3, 6, 3, 6, 3, 6, 7, 6, 55, 10, 6,
	12, 6, 14, 6, 58, 11, 6, 5, 6, 60, 10, 6, 3, 7, 3, 7, 3, 8, 3, 8, 3, 8,
	3, 8, 3, 8, 3, 9, 3, 9, 3, 9, 3, 9, 3, 9, 5, 9, 74, 10, 9, 3, 10, 3, 10,
	3, 11, 3, 11, 3, 11, 3, 12, 3, 12, 3, 13, 3, 13, 3, 13, 3, 13, 3, 13, 3,
	13, 3, 13, 3, 13, 3, 13, 3, 13, 5, 13, 93, 10, 13, 3, 13, 3, 13, 3, 13,
	2, 2, 14, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 2, 5, 3, 2, 20, 25,
	4, 2, 7, 7, 26, 26, 3, 2, 3, 5, 97, 2, 26, 3, 2, 2, 2, 4, 34, 3, 2, 2,
	2, 6, 47, 3, 2, 2, 2, 8, 49, 3, 2, 2, 2, 10, 59, 3, 2, 2, 2, 12, 61, 3,
	2, 2, 2, 14, 63, 3, 2, 2, 2, 16, 73, 3, 2, 2, 2, 18, 75, 3, 2, 2, 2, 20,
	77, 3, 2, 2, 2, 22, 80, 3, 2, 2, 2, 24, 82, 3, 2, 2, 2, 26, 31, 5, 4, 3,
	2, 27, 28, 7, 14, 2, 2, 28, 30, 5, 4, 3, 2, 29, 27, 3, 2, 2, 2, 30, 33,
	3, 2, 2, 2, 31, 29, 3, 2, 2, 2, 31, 32, 3, 2, 2, 2, 32, 3, 3, 2, 2, 2,
	33, 31, 3, 2, 2, 2, 34, 39, 5, 6, 4, 2, 35, 36, 7, 13, 2, 2, 36, 38, 5,
	6, 4, 2, 37, 35, 3, 2, 2, 2, 38, 41, 3, 2, 2, 2, 39, 37, 3, 2, 2, 2, 39,
	40, 3, 2, 2, 2, 40, 5, 3, 2, 2, 2, 41, 39, 3, 2, 2, 2, 42, 48, 5, 20, 11,
	2, 43, 48, 5, 22, 12, 2, 44, 48, 5, 18, 10, 2, 45, 48, 5, 14, 8, 2, 46,
	48, 5, 24, 13, 2, 47, 42, 3, 2, 2, 2, 47, 43, 3, 2, 2, 2, 47, 44, 3, 2,
	2, 2, 47, 45, 3, 2, 2, 2, 47, 46, 3, 2, 2, 2, 48, 7, 3, 2, 2, 2, 49, 50,
	9, 2, 2, 2, 50, 9, 3, 2, 2, 2, 51, 56, 5, 12, 7, 2, 52, 53, 7, 13, 2, 2,
	53, 55, 5, 12, 7, 2, 54, 52, 3, 2, 2, 2, 55, 58, 3, 2, 2, 2, 56, 54, 3,
	2, 2, 2, 56, 57, 3, 2, 2, 2, 57, 60, 3, 2, 2, 2, 58, 56, 3, 2, 2, 2, 59,
	51, 3, 2, 2, 2, 59, 60, 3, 2, 2, 2, 60, 11, 3, 2, 2, 2, 61, 62, 9, 3, 2,
	2, 62, 13, 3, 2, 2, 2, 63, 64, 9, 4, 2, 2, 64, 65, 7, 9, 2, 2, 65, 66,
	5, 16, 9, 2, 66, 67, 7, 10, 2, 2, 67, 15, 3, 2, 2, 2, 68, 69, 7, 26, 2,
	2, 69, 70, 5, 8, 5, 2, 70, 71, 7, 26, 2, 2, 71, 74, 3, 2, 2, 2, 72, 74,
	7, 26, 2, 2, 73, 68, 3, 2, 2, 2, 73, 72, 3, 2, 2, 2, 74, 17, 3, 2, 2, 2,
	75, 76, 7, 26, 2, 2, 76, 19, 3, 2, 2, 2, 77, 78, 7, 27, 2, 2, 78, 79, 7,
	26, 2, 2, 79, 21, 3, 2, 2, 2, 80, 81, 7, 27, 2, 2, 81, 23, 3, 2, 2, 2,
	82, 92, 7, 6, 2, 2, 83, 84, 7, 18, 2, 2, 84, 85, 7, 15, 2, 2, 85, 93, 7,
	18, 2, 2, 86, 87, 7, 18, 2, 2, 87, 93, 7, 15, 2, 2, 88, 89, 7, 15, 2, 2,
	89, 93, 7, 18, 2, 2, 90, 93, 7, 18, 2, 2, 91, 93, 3, 2, 2, 2, 92, 83, 3,
	2, 2, 2, 92, 86, 3, 2, 2, 2, 92, 88, 3, 2, 2, 2, 92, 90, 3, 2, 2, 2, 92,
	91, 3, 2, 2, 2, 93, 94, 3, 2, 2, 2, 94, 95, 7, 12, 2, 2, 95, 25, 3, 2,
	2, 2, 9, 31, 39, 47, 56, 59, 73, 92,
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
	"query", "segment", "element", "cmpr", "args", "arg", "join", "joinConstraint",
	"selElement", "dsTblElement", "dsElement", "rowRange",
}

type SLQParser struct {
	*antlr.BaseParser
}

func NewSLQParser(input antlr.TokenStream) *SLQParser {
	var decisionToDFA = make([]*antlr.DFA, len(deserializedATN.DecisionToState))
	var sharedContextCache = antlr.NewPredictionContextCache()

	for index, ds := range deserializedATN.DecisionToState {
		decisionToDFA[index] = antlr.NewDFA(ds, index)
	}

	this := new(SLQParser)

	this.BaseParser = antlr.NewBaseParser(input)

	this.Interpreter = antlr.NewParserATNSimulator(this, deserializedATN, decisionToDFA, sharedContextCache)
	this.RuleNames = ruleNames
	this.LiteralNames = literalNames
	this.SymbolicNames = symbolicNames
	this.GrammarFileName = "SLQ.g4"

	return this
}

// SLQParser tokens.
const (
	SLQParserEOF         = antlr.TokenEOF
	SLQParserT__0        = 1
	SLQParserT__1        = 2
	SLQParserT__2        = 3
	SLQParserT__3        = 4
	SLQParserID          = 5
	SLQParserWS          = 6
	SLQParserLPAR        = 7
	SLQParserRPAR        = 8
	SLQParserLBRA        = 9
	SLQParserRBRA        = 10
	SLQParserCOMMA       = 11
	SLQParserPIPE        = 12
	SLQParserCOLON       = 13
	SLQParserNULL        = 14
	SLQParserSTRING      = 15
	SLQParserINT         = 16
	SLQParserNUMBER      = 17
	SLQParserLT_EQ       = 18
	SLQParserLT          = 19
	SLQParserGT_EQ       = 20
	SLQParserGT          = 21
	SLQParserNEQ         = 22
	SLQParserEQ          = 23
	SLQParserSEL         = 24
	SLQParserDATASOURCE  = 25
	SLQParserDOT         = 26
	SLQParserVAL         = 27
	SLQParserLINECOMMENT = 28
)

// SLQParser rules.
const (
	SLQParserRULE_query          = 0
	SLQParserRULE_segment        = 1
	SLQParserRULE_element        = 2
	SLQParserRULE_cmpr           = 3
	SLQParserRULE_args           = 4
	SLQParserRULE_arg            = 5
	SLQParserRULE_join           = 6
	SLQParserRULE_joinConstraint = 7
	SLQParserRULE_selElement     = 8
	SLQParserRULE_dsTblElement   = 9
	SLQParserRULE_dsElement      = 10
	SLQParserRULE_rowRange       = 11
)

// IQueryContext is an interface to support dynamic dispatch.
type IQueryContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsQueryContext differentiates from other interfaces.
	IsQueryContext()
}

type QueryContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyQueryContext() *QueryContext {
	var p = new(QueryContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_query
	return p
}

func (*QueryContext) IsQueryContext() {}

func NewQueryContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *QueryContext {
	var p = new(QueryContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_query

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
	return s.GetTokens(SLQParserPIPE)
}

func (s *QueryContext) PIPE(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserPIPE, i)
}

func (s *QueryContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *QueryContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *QueryContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterQuery(s)
	}
}

func (s *QueryContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitQuery(s)
	}
}

func (s *QueryContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitQuery(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Query() (localctx IQueryContext) {
	localctx = NewQueryContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 0, SLQParserRULE_query)
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
	p.SetState(24)
	p.Segment()

	p.SetState(29)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserPIPE {
		p.SetState(25)
		p.Match(SLQParserPIPE)

		p.SetState(26)
		p.Segment()

		p.SetState(31)
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

	// IsSegmentContext differentiates from other interfaces.
	IsSegmentContext()
}

type SegmentContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySegmentContext() *SegmentContext {
	var p = new(SegmentContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_segment
	return p
}

func (*SegmentContext) IsSegmentContext() {}

func NewSegmentContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SegmentContext {
	var p = new(SegmentContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_segment

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
	return s.GetTokens(SLQParserCOMMA)
}

func (s *SegmentContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserCOMMA, i)
}

func (s *SegmentContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SegmentContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SegmentContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterSegment(s)
	}
}

func (s *SegmentContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitSegment(s)
	}
}

func (s *SegmentContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitSegment(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Segment() (localctx ISegmentContext) {
	localctx = NewSegmentContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 2, SLQParserRULE_segment)
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
	p.SetState(32)
	p.Element()

	p.SetState(37)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserCOMMA {
		p.SetState(33)
		p.Match(SLQParserCOMMA)

		p.SetState(34)
		p.Element()

		p.SetState(39)
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

	// IsElementContext differentiates from other interfaces.
	IsElementContext()
}

type ElementContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyElementContext() *ElementContext {
	var p = new(ElementContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_element
	return p
}

func (*ElementContext) IsElementContext() {}

func NewElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ElementContext {
	var p = new(ElementContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_element

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

func (s *ElementContext) Join() IJoinContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IJoinContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IJoinContext)
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

func (s *ElementContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterElement(s)
	}
}

func (s *ElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitElement(s)
	}
}

func (s *ElementContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitElement(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Element() (localctx IElementContext) {
	localctx = NewElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 4, SLQParserRULE_element)

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

	p.SetState(45)
	p.GetErrorHandler().Sync(p)

	la_ := p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 2, p.GetParserRuleContext())

	switch la_ {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		p.SetState(40)
		p.DsTblElement()

	case 2:
		p.EnterOuterAlt(localctx, 2)
		p.SetState(41)
		p.DsElement()

	case 3:
		p.EnterOuterAlt(localctx, 3)
		p.SetState(42)
		p.SelElement()

	case 4:
		p.EnterOuterAlt(localctx, 4)
		p.SetState(43)
		p.Join()

	case 5:
		p.EnterOuterAlt(localctx, 5)
		p.SetState(44)
		p.RowRange()

	}

	return localctx
}

// ICmprContext is an interface to support dynamic dispatch.
type ICmprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsCmprContext differentiates from other interfaces.
	IsCmprContext()
}

type CmprContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyCmprContext() *CmprContext {
	var p = new(CmprContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_cmpr
	return p
}

func (*CmprContext) IsCmprContext() {}

func NewCmprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *CmprContext {
	var p = new(CmprContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_cmpr

	return p
}

func (s *CmprContext) GetParser() antlr.Parser { return s.parser }

func (s *CmprContext) LT_EQ() antlr.TerminalNode {
	return s.GetToken(SLQParserLT_EQ, 0)
}

func (s *CmprContext) LT() antlr.TerminalNode {
	return s.GetToken(SLQParserLT, 0)
}

func (s *CmprContext) GT_EQ() antlr.TerminalNode {
	return s.GetToken(SLQParserGT_EQ, 0)
}

func (s *CmprContext) GT() antlr.TerminalNode {
	return s.GetToken(SLQParserGT, 0)
}

func (s *CmprContext) EQ() antlr.TerminalNode {
	return s.GetToken(SLQParserEQ, 0)
}

func (s *CmprContext) NEQ() antlr.TerminalNode {
	return s.GetToken(SLQParserNEQ, 0)
}

func (s *CmprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *CmprContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *CmprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterCmpr(s)
	}
}

func (s *CmprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitCmpr(s)
	}
}

func (s *CmprContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitCmpr(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Cmpr() (localctx ICmprContext) {
	localctx = NewCmprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 6, SLQParserRULE_cmpr)
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
	p.SetState(47)
	_la = p.GetTokenStream().LA(1)

	if !(((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<SLQParserLT_EQ)|(1<<SLQParserLT)|(1<<SLQParserGT_EQ)|(1<<SLQParserGT)|(1<<SLQParserNEQ)|(1<<SLQParserEQ))) != 0) {
		p.GetErrorHandler().RecoverInline(p)
	} else {
		p.Consume()
	}

	return localctx
}

// IArgsContext is an interface to support dynamic dispatch.
type IArgsContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsArgsContext differentiates from other interfaces.
	IsArgsContext()
}

type ArgsContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyArgsContext() *ArgsContext {
	var p = new(ArgsContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_args
	return p
}

func (*ArgsContext) IsArgsContext() {}

func NewArgsContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ArgsContext {
	var p = new(ArgsContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_args

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
	return s.GetTokens(SLQParserCOMMA)
}

func (s *ArgsContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserCOMMA, i)
}

func (s *ArgsContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ArgsContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ArgsContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterArgs(s)
	}
}

func (s *ArgsContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitArgs(s)
	}
}

func (s *ArgsContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitArgs(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Args() (localctx IArgsContext) {
	localctx = NewArgsContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 8, SLQParserRULE_args)
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
	p.SetState(57)
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserID || _la == SLQParserSEL {
		p.SetState(49)
		p.Arg()

		p.SetState(54)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		for _la == SLQParserCOMMA {
			p.SetState(50)
			p.Match(SLQParserCOMMA)

			p.SetState(51)
			p.Arg()

			p.SetState(56)
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

	// IsArgContext differentiates from other interfaces.
	IsArgContext()
}

type ArgContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyArgContext() *ArgContext {
	var p = new(ArgContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_arg
	return p
}

func (*ArgContext) IsArgContext() {}

func NewArgContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ArgContext {
	var p = new(ArgContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_arg

	return p
}

func (s *ArgContext) GetParser() antlr.Parser { return s.parser }

func (s *ArgContext) SEL() antlr.TerminalNode {
	return s.GetToken(SLQParserSEL, 0)
}

func (s *ArgContext) ID() antlr.TerminalNode {
	return s.GetToken(SLQParserID, 0)
}

func (s *ArgContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ArgContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ArgContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterArg(s)
	}
}

func (s *ArgContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitArg(s)
	}
}

func (s *ArgContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitArg(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Arg() (localctx IArgContext) {
	localctx = NewArgContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 10, SLQParserRULE_arg)
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
	p.SetState(59)
	_la = p.GetTokenStream().LA(1)

	if !(_la == SLQParserID || _la == SLQParserSEL) {
		p.GetErrorHandler().RecoverInline(p)
	} else {
		p.Consume()
	}

	return localctx
}

// IJoinContext is an interface to support dynamic dispatch.
type IJoinContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsJoinContext differentiates from other interfaces.
	IsJoinContext()
}

type JoinContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyJoinContext() *JoinContext {
	var p = new(JoinContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_join
	return p
}

func (*JoinContext) IsJoinContext() {}

func NewJoinContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *JoinContext {
	var p = new(JoinContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_join

	return p
}

func (s *JoinContext) GetParser() antlr.Parser { return s.parser }

func (s *JoinContext) LPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserLPAR, 0)
}

func (s *JoinContext) JoinConstraint() IJoinConstraintContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IJoinConstraintContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IJoinConstraintContext)
}

func (s *JoinContext) RPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserRPAR, 0)
}

func (s *JoinContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *JoinContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *JoinContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterJoin(s)
	}
}

func (s *JoinContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitJoin(s)
	}
}

func (s *JoinContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitJoin(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Join() (localctx IJoinContext) {
	localctx = NewJoinContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, SLQParserRULE_join)
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
	p.SetState(61)
	_la = p.GetTokenStream().LA(1)

	if !(((_la)&-(0x1f+1)) == 0 && ((1<<uint(_la))&((1<<SLQParserT__0)|(1<<SLQParserT__1)|(1<<SLQParserT__2))) != 0) {
		p.GetErrorHandler().RecoverInline(p)
	} else {
		p.Consume()
	}
	p.SetState(62)
	p.Match(SLQParserLPAR)

	p.SetState(63)
	p.JoinConstraint()

	p.SetState(64)
	p.Match(SLQParserRPAR)

	return localctx
}

// IJoinConstraintContext is an interface to support dynamic dispatch.
type IJoinConstraintContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsJoinConstraintContext differentiates from other interfaces.
	IsJoinConstraintContext()
}

type JoinConstraintContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyJoinConstraintContext() *JoinConstraintContext {
	var p = new(JoinConstraintContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_joinConstraint
	return p
}

func (*JoinConstraintContext) IsJoinConstraintContext() {}

func NewJoinConstraintContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *JoinConstraintContext {
	var p = new(JoinConstraintContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_joinConstraint

	return p
}

func (s *JoinConstraintContext) GetParser() antlr.Parser { return s.parser }

func (s *JoinConstraintContext) AllSEL() []antlr.TerminalNode {
	return s.GetTokens(SLQParserSEL)
}

func (s *JoinConstraintContext) SEL(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserSEL, i)
}

func (s *JoinConstraintContext) Cmpr() ICmprContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ICmprContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(ICmprContext)
}

func (s *JoinConstraintContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *JoinConstraintContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *JoinConstraintContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterJoinConstraint(s)
	}
}

func (s *JoinConstraintContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitJoinConstraint(s)
	}
}

func (s *JoinConstraintContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitJoinConstraint(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) JoinConstraint() (localctx IJoinConstraintContext) {
	localctx = NewJoinConstraintContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 14, SLQParserRULE_joinConstraint)

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

	p.SetState(71)
	p.GetErrorHandler().Sync(p)

	la_ := p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 5, p.GetParserRuleContext())

	switch la_ {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		p.SetState(66)
		p.Match(SLQParserSEL)

		p.SetState(67)
		p.Cmpr()

		p.SetState(68)
		p.Match(SLQParserSEL)

	case 2:
		p.EnterOuterAlt(localctx, 2)
		p.SetState(70)
		p.Match(SLQParserSEL)

	}

	return localctx
}

// ISelElementContext is an interface to support dynamic dispatch.
type ISelElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsSelElementContext differentiates from other interfaces.
	IsSelElementContext()
}

type SelElementContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySelElementContext() *SelElementContext {
	var p = new(SelElementContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_selElement
	return p
}

func (*SelElementContext) IsSelElementContext() {}

func NewSelElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SelElementContext {
	var p = new(SelElementContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_selElement

	return p
}

func (s *SelElementContext) GetParser() antlr.Parser { return s.parser }

func (s *SelElementContext) SEL() antlr.TerminalNode {
	return s.GetToken(SLQParserSEL, 0)
}

func (s *SelElementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SelElementContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SelElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterSelElement(s)
	}
}

func (s *SelElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitSelElement(s)
	}
}

func (s *SelElementContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitSelElement(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) SelElement() (localctx ISelElementContext) {
	localctx = NewSelElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 16, SLQParserRULE_selElement)

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
	p.SetState(73)
	p.Match(SLQParserSEL)

	return localctx
}

// IDsTblElementContext is an interface to support dynamic dispatch.
type IDsTblElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsDsTblElementContext differentiates from other interfaces.
	IsDsTblElementContext()
}

type DsTblElementContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyDsTblElementContext() *DsTblElementContext {
	var p = new(DsTblElementContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_dsTblElement
	return p
}

func (*DsTblElementContext) IsDsTblElementContext() {}

func NewDsTblElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *DsTblElementContext {
	var p = new(DsTblElementContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_dsTblElement

	return p
}

func (s *DsTblElementContext) GetParser() antlr.Parser { return s.parser }

func (s *DsTblElementContext) DATASOURCE() antlr.TerminalNode {
	return s.GetToken(SLQParserDATASOURCE, 0)
}

func (s *DsTblElementContext) SEL() antlr.TerminalNode {
	return s.GetToken(SLQParserSEL, 0)
}

func (s *DsTblElementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DsTblElementContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *DsTblElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterDsTblElement(s)
	}
}

func (s *DsTblElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitDsTblElement(s)
	}
}

func (s *DsTblElementContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitDsTblElement(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) DsTblElement() (localctx IDsTblElementContext) {
	localctx = NewDsTblElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 18, SLQParserRULE_dsTblElement)

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
	p.SetState(75)
	p.Match(SLQParserDATASOURCE)

	p.SetState(76)
	p.Match(SLQParserSEL)

	return localctx
}

// IDsElementContext is an interface to support dynamic dispatch.
type IDsElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsDsElementContext differentiates from other interfaces.
	IsDsElementContext()
}

type DsElementContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyDsElementContext() *DsElementContext {
	var p = new(DsElementContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_dsElement
	return p
}

func (*DsElementContext) IsDsElementContext() {}

func NewDsElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *DsElementContext {
	var p = new(DsElementContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_dsElement

	return p
}

func (s *DsElementContext) GetParser() antlr.Parser { return s.parser }

func (s *DsElementContext) DATASOURCE() antlr.TerminalNode {
	return s.GetToken(SLQParserDATASOURCE, 0)
}

func (s *DsElementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DsElementContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *DsElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterDsElement(s)
	}
}

func (s *DsElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitDsElement(s)
	}
}

func (s *DsElementContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitDsElement(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) DsElement() (localctx IDsElementContext) {
	localctx = NewDsElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 20, SLQParserRULE_dsElement)

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
	p.SetState(78)
	p.Match(SLQParserDATASOURCE)

	return localctx
}

// IRowRangeContext is an interface to support dynamic dispatch.
type IRowRangeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsRowRangeContext differentiates from other interfaces.
	IsRowRangeContext()
}

type RowRangeContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyRowRangeContext() *RowRangeContext {
	var p = new(RowRangeContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_rowRange
	return p
}

func (*RowRangeContext) IsRowRangeContext() {}

func NewRowRangeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *RowRangeContext {
	var p = new(RowRangeContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_rowRange

	return p
}

func (s *RowRangeContext) GetParser() antlr.Parser { return s.parser }

func (s *RowRangeContext) AllINT() []antlr.TerminalNode {
	return s.GetTokens(SLQParserINT)
}

func (s *RowRangeContext) INT(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserINT, i)
}

func (s *RowRangeContext) COLON() antlr.TerminalNode {
	return s.GetToken(SLQParserCOLON, 0)
}

func (s *RowRangeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *RowRangeContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *RowRangeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterRowRange(s)
	}
}

func (s *RowRangeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitRowRange(s)
	}
}

func (s *RowRangeContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitRowRange(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) RowRange() (localctx IRowRangeContext) {
	localctx = NewRowRangeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 22, SLQParserRULE_rowRange)

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
	p.Match(SLQParserT__3)

	p.SetState(90)
	p.GetErrorHandler().Sync(p)

	la_ := p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 6, p.GetParserRuleContext())

	switch la_ {
	case 1:
		p.SetState(81)
		p.Match(SLQParserINT)

		p.SetState(82)
		p.Match(SLQParserCOLON)

		p.SetState(83)
		p.Match(SLQParserINT)

	case 2:
		p.SetState(84)
		p.Match(SLQParserINT)

		p.SetState(85)
		p.Match(SLQParserCOLON)

	case 3:
		p.SetState(86)
		p.Match(SLQParserCOLON)

		p.SetState(87)
		p.Match(SLQParserINT)

	case 4:
		p.SetState(88)
		p.Match(SLQParserINT)

	case 5:

	}
	p.SetState(92)
	p.Match(SLQParserRBRA)

	return localctx
}

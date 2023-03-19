// Code generated from SLQ.g4 by ANTLR 4.12.0. DO NOT EDIT.

package slq // SLQ
import (
	"fmt"
	"strconv"
	"sync"

	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
)

// Suppress unused import errors
var _ = fmt.Printf
var _ = strconv.Itoa
var _ = sync.Once{}

type SLQParser struct {
	*antlr.BaseParser
}

var slqParserStaticData struct {
	once                   sync.Once
	serializedATN          []int32
	literalNames           []string
	symbolicNames          []string
	ruleNames              []string
	predictionContextCache *antlr.PredictionContextCache
	atn                    *antlr.ATN
	decisionToDFA          []*antlr.DFA
}

func slqParserInit() {
	staticData := &slqParserStaticData
	staticData.literalNames = []string{
		"", "';'", "'*'", "'join'", "'JOIN'", "'j'", "'group'", "'GROUP'", "'g'",
		"'.['", "'sum'", "'SUM'", "'avg'", "'AVG'", "'count'", "'COUNT'", "'where'",
		"'WHERE'", "'||'", "'/'", "'%'", "'+'", "'-'", "'<<'", "'>>'", "'&'",
		"'&&'", "'~'", "'!'", "", "", "'('", "')'", "'['", "']'", "','", "'|'",
		"':'", "", "", "", "'<='", "'<'", "'>='", "'>'", "'!='", "'=='",
	}
	staticData.symbolicNames = []string{
		"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
		"", "", "", "", "", "", "", "", "", "", "", "", "ID", "WS", "LPAR",
		"RPAR", "LBRA", "RBRA", "COMMA", "PIPE", "COLON", "NULL", "NN", "NUMBER",
		"LT_EQ", "LT", "GT_EQ", "GT", "NEQ", "EQ", "SEL", "DATASOURCE", "STRING",
		"LINECOMMENT",
	}
	staticData.ruleNames = []string{
		"stmtList", "query", "segment", "element", "cmpr", "fn", "fnElement",
		"join", "joinConstraint", "group", "alias", "selElement", "dsTblElement",
		"dsElement", "rowRange", "fnName", "expr", "literal", "unaryOperator",
	}
	staticData.predictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 1, 50, 204, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2, 4, 7,
		4, 2, 5, 7, 5, 2, 6, 7, 6, 2, 7, 7, 7, 2, 8, 7, 8, 2, 9, 7, 9, 2, 10, 7,
		10, 2, 11, 7, 11, 2, 12, 7, 12, 2, 13, 7, 13, 2, 14, 7, 14, 2, 15, 7, 15,
		2, 16, 7, 16, 2, 17, 7, 17, 2, 18, 7, 18, 1, 0, 5, 0, 40, 8, 0, 10, 0,
		12, 0, 43, 9, 0, 1, 0, 1, 0, 4, 0, 47, 8, 0, 11, 0, 12, 0, 48, 1, 0, 5,
		0, 52, 8, 0, 10, 0, 12, 0, 55, 9, 0, 1, 0, 5, 0, 58, 8, 0, 10, 0, 12, 0,
		61, 9, 0, 1, 1, 1, 1, 1, 1, 5, 1, 66, 8, 1, 10, 1, 12, 1, 69, 9, 1, 1,
		2, 1, 2, 1, 2, 5, 2, 74, 8, 2, 10, 2, 12, 2, 77, 9, 2, 1, 3, 1, 3, 1, 3,
		1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 3, 3, 87, 8, 3, 1, 4, 1, 4, 1, 5, 1, 5, 1,
		5, 1, 5, 1, 5, 5, 5, 96, 8, 5, 10, 5, 12, 5, 99, 9, 5, 1, 5, 3, 5, 102,
		8, 5, 1, 5, 1, 5, 1, 6, 1, 6, 3, 6, 108, 8, 6, 1, 7, 1, 7, 1, 7, 1, 7,
		1, 7, 1, 8, 1, 8, 1, 8, 1, 8, 1, 8, 3, 8, 120, 8, 8, 1, 9, 1, 9, 1, 9,
		1, 9, 1, 9, 5, 9, 127, 8, 9, 10, 9, 12, 9, 130, 9, 9, 1, 9, 1, 9, 1, 10,
		1, 10, 1, 10, 1, 11, 1, 11, 3, 11, 139, 8, 11, 1, 12, 1, 12, 1, 12, 1,
		13, 1, 13, 1, 14, 1, 14, 1, 14, 1, 14, 1, 14, 1, 14, 1, 14, 1, 14, 1, 14,
		3, 14, 155, 8, 14, 1, 14, 1, 14, 1, 15, 1, 15, 1, 16, 1, 16, 1, 16, 1,
		16, 1, 16, 1, 16, 1, 16, 3, 16, 168, 8, 16, 1, 16, 1, 16, 1, 16, 1, 16,
		1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1,
		16, 1, 16, 1, 16, 1, 16, 1, 16, 3, 16, 189, 8, 16, 1, 16, 1, 16, 1, 16,
		1, 16, 5, 16, 195, 8, 16, 10, 16, 12, 16, 198, 9, 16, 1, 17, 1, 17, 1,
		18, 1, 18, 1, 18, 0, 1, 32, 19, 0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20,
		22, 24, 26, 28, 30, 32, 34, 36, 0, 10, 1, 0, 41, 46, 1, 0, 3, 5, 1, 0,
		6, 8, 1, 0, 10, 17, 2, 0, 2, 2, 19, 20, 1, 0, 21, 22, 1, 0, 23, 25, 1,
		0, 41, 44, 2, 0, 38, 40, 49, 49, 2, 0, 21, 22, 27, 28, 220, 0, 41, 1, 0,
		0, 0, 2, 62, 1, 0, 0, 0, 4, 70, 1, 0, 0, 0, 6, 86, 1, 0, 0, 0, 8, 88, 1,
		0, 0, 0, 10, 90, 1, 0, 0, 0, 12, 105, 1, 0, 0, 0, 14, 109, 1, 0, 0, 0,
		16, 119, 1, 0, 0, 0, 18, 121, 1, 0, 0, 0, 20, 133, 1, 0, 0, 0, 22, 136,
		1, 0, 0, 0, 24, 140, 1, 0, 0, 0, 26, 143, 1, 0, 0, 0, 28, 145, 1, 0, 0,
		0, 30, 158, 1, 0, 0, 0, 32, 167, 1, 0, 0, 0, 34, 199, 1, 0, 0, 0, 36, 201,
		1, 0, 0, 0, 38, 40, 5, 1, 0, 0, 39, 38, 1, 0, 0, 0, 40, 43, 1, 0, 0, 0,
		41, 39, 1, 0, 0, 0, 41, 42, 1, 0, 0, 0, 42, 44, 1, 0, 0, 0, 43, 41, 1,
		0, 0, 0, 44, 53, 3, 2, 1, 0, 45, 47, 5, 1, 0, 0, 46, 45, 1, 0, 0, 0, 47,
		48, 1, 0, 0, 0, 48, 46, 1, 0, 0, 0, 48, 49, 1, 0, 0, 0, 49, 50, 1, 0, 0,
		0, 50, 52, 3, 2, 1, 0, 51, 46, 1, 0, 0, 0, 52, 55, 1, 0, 0, 0, 53, 51,
		1, 0, 0, 0, 53, 54, 1, 0, 0, 0, 54, 59, 1, 0, 0, 0, 55, 53, 1, 0, 0, 0,
		56, 58, 5, 1, 0, 0, 57, 56, 1, 0, 0, 0, 58, 61, 1, 0, 0, 0, 59, 57, 1,
		0, 0, 0, 59, 60, 1, 0, 0, 0, 60, 1, 1, 0, 0, 0, 61, 59, 1, 0, 0, 0, 62,
		67, 3, 4, 2, 0, 63, 64, 5, 36, 0, 0, 64, 66, 3, 4, 2, 0, 65, 63, 1, 0,
		0, 0, 66, 69, 1, 0, 0, 0, 67, 65, 1, 0, 0, 0, 67, 68, 1, 0, 0, 0, 68, 3,
		1, 0, 0, 0, 69, 67, 1, 0, 0, 0, 70, 75, 3, 6, 3, 0, 71, 72, 5, 35, 0, 0,
		72, 74, 3, 6, 3, 0, 73, 71, 1, 0, 0, 0, 74, 77, 1, 0, 0, 0, 75, 73, 1,
		0, 0, 0, 75, 76, 1, 0, 0, 0, 76, 5, 1, 0, 0, 0, 77, 75, 1, 0, 0, 0, 78,
		87, 3, 24, 12, 0, 79, 87, 3, 26, 13, 0, 80, 87, 3, 22, 11, 0, 81, 87, 3,
		14, 7, 0, 82, 87, 3, 18, 9, 0, 83, 87, 3, 28, 14, 0, 84, 87, 3, 12, 6,
		0, 85, 87, 3, 32, 16, 0, 86, 78, 1, 0, 0, 0, 86, 79, 1, 0, 0, 0, 86, 80,
		1, 0, 0, 0, 86, 81, 1, 0, 0, 0, 86, 82, 1, 0, 0, 0, 86, 83, 1, 0, 0, 0,
		86, 84, 1, 0, 0, 0, 86, 85, 1, 0, 0, 0, 87, 7, 1, 0, 0, 0, 88, 89, 7, 0,
		0, 0, 89, 9, 1, 0, 0, 0, 90, 91, 3, 30, 15, 0, 91, 101, 5, 31, 0, 0, 92,
		97, 3, 32, 16, 0, 93, 94, 5, 35, 0, 0, 94, 96, 3, 32, 16, 0, 95, 93, 1,
		0, 0, 0, 96, 99, 1, 0, 0, 0, 97, 95, 1, 0, 0, 0, 97, 98, 1, 0, 0, 0, 98,
		102, 1, 0, 0, 0, 99, 97, 1, 0, 0, 0, 100, 102, 5, 2, 0, 0, 101, 92, 1,
		0, 0, 0, 101, 100, 1, 0, 0, 0, 101, 102, 1, 0, 0, 0, 102, 103, 1, 0, 0,
		0, 103, 104, 5, 32, 0, 0, 104, 11, 1, 0, 0, 0, 105, 107, 3, 10, 5, 0, 106,
		108, 3, 20, 10, 0, 107, 106, 1, 0, 0, 0, 107, 108, 1, 0, 0, 0, 108, 13,
		1, 0, 0, 0, 109, 110, 7, 1, 0, 0, 110, 111, 5, 31, 0, 0, 111, 112, 3, 16,
		8, 0, 112, 113, 5, 32, 0, 0, 113, 15, 1, 0, 0, 0, 114, 115, 5, 47, 0, 0,
		115, 116, 3, 8, 4, 0, 116, 117, 5, 47, 0, 0, 117, 120, 1, 0, 0, 0, 118,
		120, 5, 47, 0, 0, 119, 114, 1, 0, 0, 0, 119, 118, 1, 0, 0, 0, 120, 17,
		1, 0, 0, 0, 121, 122, 7, 2, 0, 0, 122, 123, 5, 31, 0, 0, 123, 128, 5, 47,
		0, 0, 124, 125, 5, 35, 0, 0, 125, 127, 5, 47, 0, 0, 126, 124, 1, 0, 0,
		0, 127, 130, 1, 0, 0, 0, 128, 126, 1, 0, 0, 0, 128, 129, 1, 0, 0, 0, 129,
		131, 1, 0, 0, 0, 130, 128, 1, 0, 0, 0, 131, 132, 5, 32, 0, 0, 132, 19,
		1, 0, 0, 0, 133, 134, 5, 37, 0, 0, 134, 135, 5, 29, 0, 0, 135, 21, 1, 0,
		0, 0, 136, 138, 5, 47, 0, 0, 137, 139, 3, 20, 10, 0, 138, 137, 1, 0, 0,
		0, 138, 139, 1, 0, 0, 0, 139, 23, 1, 0, 0, 0, 140, 141, 5, 48, 0, 0, 141,
		142, 5, 47, 0, 0, 142, 25, 1, 0, 0, 0, 143, 144, 5, 48, 0, 0, 144, 27,
		1, 0, 0, 0, 145, 154, 5, 9, 0, 0, 146, 147, 5, 39, 0, 0, 147, 148, 5, 37,
		0, 0, 148, 155, 5, 39, 0, 0, 149, 150, 5, 39, 0, 0, 150, 155, 5, 37, 0,
		0, 151, 152, 5, 37, 0, 0, 152, 155, 5, 39, 0, 0, 153, 155, 5, 39, 0, 0,
		154, 146, 1, 0, 0, 0, 154, 149, 1, 0, 0, 0, 154, 151, 1, 0, 0, 0, 154,
		153, 1, 0, 0, 0, 154, 155, 1, 0, 0, 0, 155, 156, 1, 0, 0, 0, 156, 157,
		5, 34, 0, 0, 157, 29, 1, 0, 0, 0, 158, 159, 7, 3, 0, 0, 159, 31, 1, 0,
		0, 0, 160, 161, 6, 16, -1, 0, 161, 168, 5, 47, 0, 0, 162, 168, 3, 34, 17,
		0, 163, 164, 3, 36, 18, 0, 164, 165, 3, 32, 16, 9, 165, 168, 1, 0, 0, 0,
		166, 168, 3, 10, 5, 0, 167, 160, 1, 0, 0, 0, 167, 162, 1, 0, 0, 0, 167,
		163, 1, 0, 0, 0, 167, 166, 1, 0, 0, 0, 168, 196, 1, 0, 0, 0, 169, 170,
		10, 8, 0, 0, 170, 171, 5, 18, 0, 0, 171, 195, 3, 32, 16, 9, 172, 173, 10,
		7, 0, 0, 173, 174, 7, 4, 0, 0, 174, 195, 3, 32, 16, 8, 175, 176, 10, 6,
		0, 0, 176, 177, 7, 5, 0, 0, 177, 195, 3, 32, 16, 7, 178, 179, 10, 5, 0,
		0, 179, 180, 7, 6, 0, 0, 180, 195, 3, 32, 16, 6, 181, 182, 10, 4, 0, 0,
		182, 183, 7, 7, 0, 0, 183, 195, 3, 32, 16, 5, 184, 188, 10, 3, 0, 0, 185,
		189, 5, 46, 0, 0, 186, 189, 5, 45, 0, 0, 187, 189, 1, 0, 0, 0, 188, 185,
		1, 0, 0, 0, 188, 186, 1, 0, 0, 0, 188, 187, 1, 0, 0, 0, 189, 190, 1, 0,
		0, 0, 190, 195, 3, 32, 16, 4, 191, 192, 10, 2, 0, 0, 192, 193, 5, 26, 0,
		0, 193, 195, 3, 32, 16, 3, 194, 169, 1, 0, 0, 0, 194, 172, 1, 0, 0, 0,
		194, 175, 1, 0, 0, 0, 194, 178, 1, 0, 0, 0, 194, 181, 1, 0, 0, 0, 194,
		184, 1, 0, 0, 0, 194, 191, 1, 0, 0, 0, 195, 198, 1, 0, 0, 0, 196, 194,
		1, 0, 0, 0, 196, 197, 1, 0, 0, 0, 197, 33, 1, 0, 0, 0, 198, 196, 1, 0,
		0, 0, 199, 200, 7, 8, 0, 0, 200, 35, 1, 0, 0, 0, 201, 202, 7, 9, 0, 0,
		202, 37, 1, 0, 0, 0, 18, 41, 48, 53, 59, 67, 75, 86, 97, 101, 107, 119,
		128, 138, 154, 167, 188, 194, 196,
	}
	deserializer := antlr.NewATNDeserializer(nil)
	staticData.atn = deserializer.Deserialize(staticData.serializedATN)
	atn := staticData.atn
	staticData.decisionToDFA = make([]*antlr.DFA, len(atn.DecisionToState))
	decisionToDFA := staticData.decisionToDFA
	for index, state := range atn.DecisionToState {
		decisionToDFA[index] = antlr.NewDFA(state, index)
	}
}

// SLQParserInit initializes any static state used to implement SLQParser. By default the
// static state used to implement the parser is lazily initialized during the first call to
// NewSLQParser(). You can call this function if you wish to initialize the static state ahead
// of time.
func SLQParserInit() {
	staticData := &slqParserStaticData
	staticData.once.Do(slqParserInit)
}

// NewSLQParser produces a new parser instance for the optional input antlr.TokenStream.
func NewSLQParser(input antlr.TokenStream) *SLQParser {
	SLQParserInit()
	this := new(SLQParser)
	this.BaseParser = antlr.NewBaseParser(input)
	staticData := &slqParserStaticData
	this.Interpreter = antlr.NewParserATNSimulator(this, staticData.atn, staticData.decisionToDFA, staticData.predictionContextCache)
	this.RuleNames = staticData.ruleNames
	this.LiteralNames = staticData.literalNames
	this.SymbolicNames = staticData.symbolicNames
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
	SLQParserT__4        = 5
	SLQParserT__5        = 6
	SLQParserT__6        = 7
	SLQParserT__7        = 8
	SLQParserT__8        = 9
	SLQParserT__9        = 10
	SLQParserT__10       = 11
	SLQParserT__11       = 12
	SLQParserT__12       = 13
	SLQParserT__13       = 14
	SLQParserT__14       = 15
	SLQParserT__15       = 16
	SLQParserT__16       = 17
	SLQParserT__17       = 18
	SLQParserT__18       = 19
	SLQParserT__19       = 20
	SLQParserT__20       = 21
	SLQParserT__21       = 22
	SLQParserT__22       = 23
	SLQParserT__23       = 24
	SLQParserT__24       = 25
	SLQParserT__25       = 26
	SLQParserT__26       = 27
	SLQParserT__27       = 28
	SLQParserID          = 29
	SLQParserWS          = 30
	SLQParserLPAR        = 31
	SLQParserRPAR        = 32
	SLQParserLBRA        = 33
	SLQParserRBRA        = 34
	SLQParserCOMMA       = 35
	SLQParserPIPE        = 36
	SLQParserCOLON       = 37
	SLQParserNULL        = 38
	SLQParserNN          = 39
	SLQParserNUMBER      = 40
	SLQParserLT_EQ       = 41
	SLQParserLT          = 42
	SLQParserGT_EQ       = 43
	SLQParserGT          = 44
	SLQParserNEQ         = 45
	SLQParserEQ          = 46
	SLQParserSEL         = 47
	SLQParserDATASOURCE  = 48
	SLQParserSTRING      = 49
	SLQParserLINECOMMENT = 50
)

// SLQParser rules.
const (
	SLQParserRULE_stmtList       = 0
	SLQParserRULE_query          = 1
	SLQParserRULE_segment        = 2
	SLQParserRULE_element        = 3
	SLQParserRULE_cmpr           = 4
	SLQParserRULE_fn             = 5
	SLQParserRULE_fnElement      = 6
	SLQParserRULE_join           = 7
	SLQParserRULE_joinConstraint = 8
	SLQParserRULE_group          = 9
	SLQParserRULE_alias          = 10
	SLQParserRULE_selElement     = 11
	SLQParserRULE_dsTblElement   = 12
	SLQParserRULE_dsElement      = 13
	SLQParserRULE_rowRange       = 14
	SLQParserRULE_fnName         = 15
	SLQParserRULE_expr           = 16
	SLQParserRULE_literal        = 17
	SLQParserRULE_unaryOperator  = 18
)

// IStmtListContext is an interface to support dynamic dispatch.
type IStmtListContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	AllQuery() []IQueryContext
	Query(i int) IQueryContext

	// IsStmtListContext differentiates from other interfaces.
	IsStmtListContext()
}

type StmtListContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyStmtListContext() *StmtListContext {
	var p = new(StmtListContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_stmtList
	return p
}

func (*StmtListContext) IsStmtListContext() {}

func NewStmtListContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *StmtListContext {
	var p = new(StmtListContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_stmtList

	return p
}

func (s *StmtListContext) GetParser() antlr.Parser { return s.parser }

func (s *StmtListContext) AllQuery() []IQueryContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IQueryContext); ok {
			len++
		}
	}

	tst := make([]IQueryContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IQueryContext); ok {
			tst[i] = t.(IQueryContext)
			i++
		}
	}

	return tst
}

func (s *StmtListContext) Query(i int) IQueryContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IQueryContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IQueryContext)
}

func (s *StmtListContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *StmtListContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *StmtListContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterStmtList(s)
	}
}

func (s *StmtListContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitStmtList(s)
	}
}

func (s *StmtListContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitStmtList(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) StmtList() (localctx IStmtListContext) {
	this := p
	_ = this

	localctx = NewStmtListContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 0, SLQParserRULE_stmtList)
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

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(41)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserT__0 {
		{
			p.SetState(38)
			p.Match(SLQParserT__0)
		}

		p.SetState(43)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(44)
		p.Query()
	}
	p.SetState(53)
	p.GetErrorHandler().Sync(p)
	_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 2, p.GetParserRuleContext())

	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			p.SetState(46)
			p.GetErrorHandler().Sync(p)
			_la = p.GetTokenStream().LA(1)

			for ok := true; ok; ok = _la == SLQParserT__0 {
				{
					p.SetState(45)
					p.Match(SLQParserT__0)
				}

				p.SetState(48)
				p.GetErrorHandler().Sync(p)
				_la = p.GetTokenStream().LA(1)
			}
			{
				p.SetState(50)
				p.Query()
			}

		}
		p.SetState(55)
		p.GetErrorHandler().Sync(p)
		_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 2, p.GetParserRuleContext())
	}
	p.SetState(59)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserT__0 {
		{
			p.SetState(56)
			p.Match(SLQParserT__0)
		}

		p.SetState(61)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}

	return localctx
}

// IQueryContext is an interface to support dynamic dispatch.
type IQueryContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	AllSegment() []ISegmentContext
	Segment(i int) ISegmentContext
	AllPIPE() []antlr.TerminalNode
	PIPE(i int) antlr.TerminalNode

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
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ISegmentContext); ok {
			len++
		}
	}

	tst := make([]ISegmentContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ISegmentContext); ok {
			tst[i] = t.(ISegmentContext)
			i++
		}
	}

	return tst
}

func (s *QueryContext) Segment(i int) ISegmentContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISegmentContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

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
	this := p
	_ = this

	localctx = NewQueryContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 2, SLQParserRULE_query)
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
	{
		p.SetState(62)
		p.Segment()
	}
	p.SetState(67)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserPIPE {
		{
			p.SetState(63)
			p.Match(SLQParserPIPE)
		}
		{
			p.SetState(64)
			p.Segment()
		}

		p.SetState(69)
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

	// Getter signatures
	AllElement() []IElementContext
	Element(i int) IElementContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

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
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IElementContext); ok {
			len++
		}
	}

	tst := make([]IElementContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IElementContext); ok {
			tst[i] = t.(IElementContext)
			i++
		}
	}

	return tst
}

func (s *SegmentContext) Element(i int) IElementContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IElementContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

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
	this := p
	_ = this

	localctx = NewSegmentContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 4, SLQParserRULE_segment)
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
	{
		p.SetState(70)
		p.Element()
	}

	p.SetState(75)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserCOMMA {
		{
			p.SetState(71)
			p.Match(SLQParserCOMMA)
		}
		{
			p.SetState(72)
			p.Element()
		}

		p.SetState(77)
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

	// Getter signatures
	DsTblElement() IDsTblElementContext
	DsElement() IDsElementContext
	SelElement() ISelElementContext
	Join() IJoinContext
	Group() IGroupContext
	RowRange() IRowRangeContext
	FnElement() IFnElementContext
	Expr() IExprContext

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IDsTblElementContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IDsTblElementContext)
}

func (s *ElementContext) DsElement() IDsElementContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IDsElementContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IDsElementContext)
}

func (s *ElementContext) SelElement() ISelElementContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISelElementContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISelElementContext)
}

func (s *ElementContext) Join() IJoinContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IJoinContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IJoinContext)
}

func (s *ElementContext) Group() IGroupContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IGroupContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IGroupContext)
}

func (s *ElementContext) RowRange() IRowRangeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IRowRangeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IRowRangeContext)
}

func (s *ElementContext) FnElement() IFnElementContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFnElementContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFnElementContext)
}

func (s *ElementContext) Expr() IExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
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
	this := p
	_ = this

	localctx = NewElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 6, SLQParserRULE_element)

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

	p.SetState(86)
	p.GetErrorHandler().Sync(p)
	switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 6, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(78)
			p.DsTblElement()
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(79)
			p.DsElement()
		}

	case 3:
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(80)
			p.SelElement()
		}

	case 4:
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(81)
			p.Join()
		}

	case 5:
		p.EnterOuterAlt(localctx, 5)
		{
			p.SetState(82)
			p.Group()
		}

	case 6:
		p.EnterOuterAlt(localctx, 6)
		{
			p.SetState(83)
			p.RowRange()
		}

	case 7:
		p.EnterOuterAlt(localctx, 7)
		{
			p.SetState(84)
			p.FnElement()
		}

	case 8:
		p.EnterOuterAlt(localctx, 8)
		{
			p.SetState(85)
			p.expr(0)
		}

	}

	return localctx
}

// ICmprContext is an interface to support dynamic dispatch.
type ICmprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	LT_EQ() antlr.TerminalNode
	LT() antlr.TerminalNode
	GT_EQ() antlr.TerminalNode
	GT() antlr.TerminalNode
	EQ() antlr.TerminalNode
	NEQ() antlr.TerminalNode

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
	this := p
	_ = this

	localctx = NewCmprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 8, SLQParserRULE_cmpr)
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
	{
		p.SetState(88)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&138538465099776) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	return localctx
}

// IFnContext is an interface to support dynamic dispatch.
type IFnContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	FnName() IFnNameContext
	LPAR() antlr.TerminalNode
	RPAR() antlr.TerminalNode
	AllExpr() []IExprContext
	Expr(i int) IExprContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsFnContext differentiates from other interfaces.
	IsFnContext()
}

type FnContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFnContext() *FnContext {
	var p = new(FnContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_fn
	return p
}

func (*FnContext) IsFnContext() {}

func NewFnContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FnContext {
	var p = new(FnContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_fn

	return p
}

func (s *FnContext) GetParser() antlr.Parser { return s.parser }

func (s *FnContext) FnName() IFnNameContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFnNameContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFnNameContext)
}

func (s *FnContext) LPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserLPAR, 0)
}

func (s *FnContext) RPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserRPAR, 0)
}

func (s *FnContext) AllExpr() []IExprContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExprContext); ok {
			len++
		}
	}

	tst := make([]IExprContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExprContext); ok {
			tst[i] = t.(IExprContext)
			i++
		}
	}

	return tst
}

func (s *FnContext) Expr(i int) IExprContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *FnContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(SLQParserCOMMA)
}

func (s *FnContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserCOMMA, i)
}

func (s *FnContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FnContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FnContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterFn(s)
	}
}

func (s *FnContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitFn(s)
	}
}

func (s *FnContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitFn(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Fn() (localctx IFnContext) {
	this := p
	_ = this

	localctx = NewFnContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 10, SLQParserRULE_fn)
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
	{
		p.SetState(90)
		p.FnName()
	}
	{
		p.SetState(91)
		p.Match(SLQParserLPAR)
	}
	p.SetState(101)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case SLQParserT__9, SLQParserT__10, SLQParserT__11, SLQParserT__12, SLQParserT__13, SLQParserT__14, SLQParserT__15, SLQParserT__16, SLQParserT__20, SLQParserT__21, SLQParserT__26, SLQParserT__27, SLQParserNULL, SLQParserNN, SLQParserNUMBER, SLQParserSEL, SLQParserSTRING:
		{
			p.SetState(92)
			p.expr(0)
		}
		p.SetState(97)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		for _la == SLQParserCOMMA {
			{
				p.SetState(93)
				p.Match(SLQParserCOMMA)
			}
			{
				p.SetState(94)
				p.expr(0)
			}

			p.SetState(99)
			p.GetErrorHandler().Sync(p)
			_la = p.GetTokenStream().LA(1)
		}

	case SLQParserT__1:
		{
			p.SetState(100)
			p.Match(SLQParserT__1)
		}

	case SLQParserRPAR:

	default:
	}
	{
		p.SetState(103)
		p.Match(SLQParserRPAR)
	}

	return localctx
}

// IFnElementContext is an interface to support dynamic dispatch.
type IFnElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Fn() IFnContext
	Alias() IAliasContext

	// IsFnElementContext differentiates from other interfaces.
	IsFnElementContext()
}

type FnElementContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFnElementContext() *FnElementContext {
	var p = new(FnElementContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_fnElement
	return p
}

func (*FnElementContext) IsFnElementContext() {}

func NewFnElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FnElementContext {
	var p = new(FnElementContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_fnElement

	return p
}

func (s *FnElementContext) GetParser() antlr.Parser { return s.parser }

func (s *FnElementContext) Fn() IFnContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFnContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFnContext)
}

func (s *FnElementContext) Alias() IAliasContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IAliasContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IAliasContext)
}

func (s *FnElementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FnElementContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FnElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterFnElement(s)
	}
}

func (s *FnElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitFnElement(s)
	}
}

func (s *FnElementContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitFnElement(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) FnElement() (localctx IFnElementContext) {
	this := p
	_ = this

	localctx = NewFnElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, SLQParserRULE_fnElement)
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
	{
		p.SetState(105)
		p.Fn()
	}
	p.SetState(107)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserCOLON {
		{
			p.SetState(106)
			p.Alias()
		}

	}

	return localctx
}

// IJoinContext is an interface to support dynamic dispatch.
type IJoinContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	LPAR() antlr.TerminalNode
	JoinConstraint() IJoinConstraintContext
	RPAR() antlr.TerminalNode

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IJoinConstraintContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	this := p
	_ = this

	localctx = NewJoinContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 14, SLQParserRULE_join)
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
	{
		p.SetState(109)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&56) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}
	{
		p.SetState(110)
		p.Match(SLQParserLPAR)
	}
	{
		p.SetState(111)
		p.JoinConstraint()
	}
	{
		p.SetState(112)
		p.Match(SLQParserRPAR)
	}

	return localctx
}

// IJoinConstraintContext is an interface to support dynamic dispatch.
type IJoinConstraintContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	AllSEL() []antlr.TerminalNode
	SEL(i int) antlr.TerminalNode
	Cmpr() ICmprContext

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ICmprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	this := p
	_ = this

	localctx = NewJoinConstraintContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 16, SLQParserRULE_joinConstraint)

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

	p.SetState(119)
	p.GetErrorHandler().Sync(p)
	switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 10, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(114)
			p.Match(SLQParserSEL)
		}
		{
			p.SetState(115)
			p.Cmpr()
		}
		{
			p.SetState(116)
			p.Match(SLQParserSEL)
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(118)
			p.Match(SLQParserSEL)
		}

	}

	return localctx
}

// IGroupContext is an interface to support dynamic dispatch.
type IGroupContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	LPAR() antlr.TerminalNode
	AllSEL() []antlr.TerminalNode
	SEL(i int) antlr.TerminalNode
	RPAR() antlr.TerminalNode
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsGroupContext differentiates from other interfaces.
	IsGroupContext()
}

type GroupContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyGroupContext() *GroupContext {
	var p = new(GroupContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_group
	return p
}

func (*GroupContext) IsGroupContext() {}

func NewGroupContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *GroupContext {
	var p = new(GroupContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_group

	return p
}

func (s *GroupContext) GetParser() antlr.Parser { return s.parser }

func (s *GroupContext) LPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserLPAR, 0)
}

func (s *GroupContext) AllSEL() []antlr.TerminalNode {
	return s.GetTokens(SLQParserSEL)
}

func (s *GroupContext) SEL(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserSEL, i)
}

func (s *GroupContext) RPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserRPAR, 0)
}

func (s *GroupContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(SLQParserCOMMA)
}

func (s *GroupContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserCOMMA, i)
}

func (s *GroupContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *GroupContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *GroupContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterGroup(s)
	}
}

func (s *GroupContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitGroup(s)
	}
}

func (s *GroupContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitGroup(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Group() (localctx IGroupContext) {
	this := p
	_ = this

	localctx = NewGroupContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 18, SLQParserRULE_group)
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
	{
		p.SetState(121)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&448) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}
	{
		p.SetState(122)
		p.Match(SLQParserLPAR)
	}
	{
		p.SetState(123)
		p.Match(SLQParserSEL)
	}
	p.SetState(128)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserCOMMA {
		{
			p.SetState(124)
			p.Match(SLQParserCOMMA)
		}
		{
			p.SetState(125)
			p.Match(SLQParserSEL)
		}

		p.SetState(130)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(131)
		p.Match(SLQParserRPAR)
	}

	return localctx
}

// IAliasContext is an interface to support dynamic dispatch.
type IAliasContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	COLON() antlr.TerminalNode
	ID() antlr.TerminalNode

	// IsAliasContext differentiates from other interfaces.
	IsAliasContext()
}

type AliasContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyAliasContext() *AliasContext {
	var p = new(AliasContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_alias
	return p
}

func (*AliasContext) IsAliasContext() {}

func NewAliasContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *AliasContext {
	var p = new(AliasContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_alias

	return p
}

func (s *AliasContext) GetParser() antlr.Parser { return s.parser }

func (s *AliasContext) COLON() antlr.TerminalNode {
	return s.GetToken(SLQParserCOLON, 0)
}

func (s *AliasContext) ID() antlr.TerminalNode {
	return s.GetToken(SLQParserID, 0)
}

func (s *AliasContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *AliasContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *AliasContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterAlias(s)
	}
}

func (s *AliasContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitAlias(s)
	}
}

func (s *AliasContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitAlias(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Alias() (localctx IAliasContext) {
	this := p
	_ = this

	localctx = NewAliasContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 20, SLQParserRULE_alias)

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
	{
		p.SetState(133)
		p.Match(SLQParserCOLON)
	}
	{
		p.SetState(134)
		p.Match(SLQParserID)
	}

	return localctx
}

// ISelElementContext is an interface to support dynamic dispatch.
type ISelElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	SEL() antlr.TerminalNode
	Alias() IAliasContext

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

func (s *SelElementContext) Alias() IAliasContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IAliasContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IAliasContext)
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
	this := p
	_ = this

	localctx = NewSelElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 22, SLQParserRULE_selElement)
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
	{
		p.SetState(136)
		p.Match(SLQParserSEL)
	}
	p.SetState(138)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserCOLON {
		{
			p.SetState(137)
			p.Alias()
		}

	}

	return localctx
}

// IDsTblElementContext is an interface to support dynamic dispatch.
type IDsTblElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	DATASOURCE() antlr.TerminalNode
	SEL() antlr.TerminalNode

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
	this := p
	_ = this

	localctx = NewDsTblElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 24, SLQParserRULE_dsTblElement)

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
	{
		p.SetState(140)
		p.Match(SLQParserDATASOURCE)
	}
	{
		p.SetState(141)
		p.Match(SLQParserSEL)
	}

	return localctx
}

// IDsElementContext is an interface to support dynamic dispatch.
type IDsElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	DATASOURCE() antlr.TerminalNode

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
	this := p
	_ = this

	localctx = NewDsElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 26, SLQParserRULE_dsElement)

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
	{
		p.SetState(143)
		p.Match(SLQParserDATASOURCE)
	}

	return localctx
}

// IRowRangeContext is an interface to support dynamic dispatch.
type IRowRangeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	RBRA() antlr.TerminalNode
	AllNN() []antlr.TerminalNode
	NN(i int) antlr.TerminalNode
	COLON() antlr.TerminalNode

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

func (s *RowRangeContext) RBRA() antlr.TerminalNode {
	return s.GetToken(SLQParserRBRA, 0)
}

func (s *RowRangeContext) AllNN() []antlr.TerminalNode {
	return s.GetTokens(SLQParserNN)
}

func (s *RowRangeContext) NN(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserNN, i)
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
	this := p
	_ = this

	localctx = NewRowRangeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 28, SLQParserRULE_rowRange)

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
	{
		p.SetState(145)
		p.Match(SLQParserT__8)
	}
	p.SetState(154)
	p.GetErrorHandler().Sync(p)

	if p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 13, p.GetParserRuleContext()) == 1 {
		{
			p.SetState(146)
			p.Match(SLQParserNN)
		}
		{
			p.SetState(147)
			p.Match(SLQParserCOLON)
		}
		{
			p.SetState(148)
			p.Match(SLQParserNN)
		}

	} else if p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 13, p.GetParserRuleContext()) == 2 {
		{
			p.SetState(149)
			p.Match(SLQParserNN)
		}
		{
			p.SetState(150)
			p.Match(SLQParserCOLON)
		}

	} else if p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 13, p.GetParserRuleContext()) == 3 {
		{
			p.SetState(151)
			p.Match(SLQParserCOLON)
		}
		{
			p.SetState(152)
			p.Match(SLQParserNN)
		}

	} else if p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 13, p.GetParserRuleContext()) == 4 {
		{
			p.SetState(153)
			p.Match(SLQParserNN)
		}

	}
	{
		p.SetState(156)
		p.Match(SLQParserRBRA)
	}

	return localctx
}

// IFnNameContext is an interface to support dynamic dispatch.
type IFnNameContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsFnNameContext differentiates from other interfaces.
	IsFnNameContext()
}

type FnNameContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFnNameContext() *FnNameContext {
	var p = new(FnNameContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_fnName
	return p
}

func (*FnNameContext) IsFnNameContext() {}

func NewFnNameContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FnNameContext {
	var p = new(FnNameContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_fnName

	return p
}

func (s *FnNameContext) GetParser() antlr.Parser { return s.parser }
func (s *FnNameContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FnNameContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FnNameContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterFnName(s)
	}
}

func (s *FnNameContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitFnName(s)
	}
}

func (s *FnNameContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitFnName(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) FnName() (localctx IFnNameContext) {
	this := p
	_ = this

	localctx = NewFnNameContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 30, SLQParserRULE_fnName)
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
	{
		p.SetState(158)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&261120) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	return localctx
}

// IExprContext is an interface to support dynamic dispatch.
type IExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	SEL() antlr.TerminalNode
	Literal() ILiteralContext
	UnaryOperator() IUnaryOperatorContext
	AllExpr() []IExprContext
	Expr(i int) IExprContext
	Fn() IFnContext
	LT() antlr.TerminalNode
	LT_EQ() antlr.TerminalNode
	GT() antlr.TerminalNode
	GT_EQ() antlr.TerminalNode
	EQ() antlr.TerminalNode
	NEQ() antlr.TerminalNode

	// IsExprContext differentiates from other interfaces.
	IsExprContext()
}

type ExprContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyExprContext() *ExprContext {
	var p = new(ExprContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_expr
	return p
}

func (*ExprContext) IsExprContext() {}

func NewExprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ExprContext {
	var p = new(ExprContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_expr

	return p
}

func (s *ExprContext) GetParser() antlr.Parser { return s.parser }

func (s *ExprContext) SEL() antlr.TerminalNode {
	return s.GetToken(SLQParserSEL, 0)
}

func (s *ExprContext) Literal() ILiteralContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ILiteralContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ILiteralContext)
}

func (s *ExprContext) UnaryOperator() IUnaryOperatorContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IUnaryOperatorContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IUnaryOperatorContext)
}

func (s *ExprContext) AllExpr() []IExprContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExprContext); ok {
			len++
		}
	}

	tst := make([]IExprContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExprContext); ok {
			tst[i] = t.(IExprContext)
			i++
		}
	}

	return tst
}

func (s *ExprContext) Expr(i int) IExprContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *ExprContext) Fn() IFnContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFnContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFnContext)
}

func (s *ExprContext) LT() antlr.TerminalNode {
	return s.GetToken(SLQParserLT, 0)
}

func (s *ExprContext) LT_EQ() antlr.TerminalNode {
	return s.GetToken(SLQParserLT_EQ, 0)
}

func (s *ExprContext) GT() antlr.TerminalNode {
	return s.GetToken(SLQParserGT, 0)
}

func (s *ExprContext) GT_EQ() antlr.TerminalNode {
	return s.GetToken(SLQParserGT_EQ, 0)
}

func (s *ExprContext) EQ() antlr.TerminalNode {
	return s.GetToken(SLQParserEQ, 0)
}

func (s *ExprContext) NEQ() antlr.TerminalNode {
	return s.GetToken(SLQParserNEQ, 0)
}

func (s *ExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ExprContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterExpr(s)
	}
}

func (s *ExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitExpr(s)
	}
}

func (s *ExprContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitExpr(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Expr() (localctx IExprContext) {
	return p.expr(0)
}

func (p *SLQParser) expr(_p int) (localctx IExprContext) {
	this := p
	_ = this

	var _parentctx antlr.ParserRuleContext = p.GetParserRuleContext()
	_parentState := p.GetState()
	localctx = NewExprContext(p, p.GetParserRuleContext(), _parentState)
	var _prevctx IExprContext = localctx
	var _ antlr.ParserRuleContext = _prevctx // TODO: To prevent unused variable warning.
	_startState := 32
	p.EnterRecursionRule(localctx, 32, SLQParserRULE_expr, _p)
	var _la int

	defer func() {
		p.UnrollRecursionContexts(_parentctx)
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

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(167)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case SLQParserSEL:
		{
			p.SetState(161)
			p.Match(SLQParserSEL)
		}

	case SLQParserNULL, SLQParserNN, SLQParserNUMBER, SLQParserSTRING:
		{
			p.SetState(162)
			p.Literal()
		}

	case SLQParserT__20, SLQParserT__21, SLQParserT__26, SLQParserT__27:
		{
			p.SetState(163)
			p.UnaryOperator()
		}
		{
			p.SetState(164)
			p.expr(9)
		}

	case SLQParserT__9, SLQParserT__10, SLQParserT__11, SLQParserT__12, SLQParserT__13, SLQParserT__14, SLQParserT__15, SLQParserT__16:
		{
			p.SetState(166)
			p.Fn()
		}

	default:
		panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
	}
	p.GetParserRuleContext().SetStop(p.GetTokenStream().LT(-1))
	p.SetState(196)
	p.GetErrorHandler().Sync(p)
	_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 17, p.GetParserRuleContext())

	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			if p.GetParseListeners() != nil {
				p.TriggerExitRuleEvent()
			}
			_prevctx = localctx
			p.SetState(194)
			p.GetErrorHandler().Sync(p)
			switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 16, p.GetParserRuleContext()) {
			case 1:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(169)

				if !(p.Precpred(p.GetParserRuleContext(), 8)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 8)", ""))
				}
				{
					p.SetState(170)
					p.Match(SLQParserT__17)
				}
				{
					p.SetState(171)
					p.expr(9)
				}

			case 2:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(172)

				if !(p.Precpred(p.GetParserRuleContext(), 7)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 7)", ""))
				}
				{
					p.SetState(173)
					_la = p.GetTokenStream().LA(1)

					if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&1572868) != 0) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(174)
					p.expr(8)
				}

			case 3:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(175)

				if !(p.Precpred(p.GetParserRuleContext(), 6)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 6)", ""))
				}
				{
					p.SetState(176)
					_la = p.GetTokenStream().LA(1)

					if !(_la == SLQParserT__20 || _la == SLQParserT__21) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(177)
					p.expr(7)
				}

			case 4:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(178)

				if !(p.Precpred(p.GetParserRuleContext(), 5)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 5)", ""))
				}
				{
					p.SetState(179)
					_la = p.GetTokenStream().LA(1)

					if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&58720256) != 0) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(180)
					p.expr(6)
				}

			case 5:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(181)

				if !(p.Precpred(p.GetParserRuleContext(), 4)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 4)", ""))
				}
				{
					p.SetState(182)
					_la = p.GetTokenStream().LA(1)

					if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&32985348833280) != 0) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(183)
					p.expr(5)
				}

			case 6:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(184)

				if !(p.Precpred(p.GetParserRuleContext(), 3)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 3)", ""))
				}
				p.SetState(188)
				p.GetErrorHandler().Sync(p)

				switch p.GetTokenStream().LA(1) {
				case SLQParserEQ:
					{
						p.SetState(185)
						p.Match(SLQParserEQ)
					}

				case SLQParserNEQ:
					{
						p.SetState(186)
						p.Match(SLQParserNEQ)
					}

				case SLQParserT__9, SLQParserT__10, SLQParserT__11, SLQParserT__12, SLQParserT__13, SLQParserT__14, SLQParserT__15, SLQParserT__16, SLQParserT__20, SLQParserT__21, SLQParserT__26, SLQParserT__27, SLQParserNULL, SLQParserNN, SLQParserNUMBER, SLQParserSEL, SLQParserSTRING:

				default:
					panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
				}
				{
					p.SetState(190)
					p.expr(4)
				}

			case 7:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(191)

				if !(p.Precpred(p.GetParserRuleContext(), 2)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 2)", ""))
				}
				{
					p.SetState(192)
					p.Match(SLQParserT__25)
				}
				{
					p.SetState(193)
					p.expr(3)
				}

			}

		}
		p.SetState(198)
		p.GetErrorHandler().Sync(p)
		_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 17, p.GetParserRuleContext())
	}

	return localctx
}

// ILiteralContext is an interface to support dynamic dispatch.
type ILiteralContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	NN() antlr.TerminalNode
	NUMBER() antlr.TerminalNode
	STRING() antlr.TerminalNode
	NULL() antlr.TerminalNode

	// IsLiteralContext differentiates from other interfaces.
	IsLiteralContext()
}

type LiteralContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyLiteralContext() *LiteralContext {
	var p = new(LiteralContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_literal
	return p
}

func (*LiteralContext) IsLiteralContext() {}

func NewLiteralContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *LiteralContext {
	var p = new(LiteralContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_literal

	return p
}

func (s *LiteralContext) GetParser() antlr.Parser { return s.parser }

func (s *LiteralContext) NN() antlr.TerminalNode {
	return s.GetToken(SLQParserNN, 0)
}

func (s *LiteralContext) NUMBER() antlr.TerminalNode {
	return s.GetToken(SLQParserNUMBER, 0)
}

func (s *LiteralContext) STRING() antlr.TerminalNode {
	return s.GetToken(SLQParserSTRING, 0)
}

func (s *LiteralContext) NULL() antlr.TerminalNode {
	return s.GetToken(SLQParserNULL, 0)
}

func (s *LiteralContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *LiteralContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *LiteralContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterLiteral(s)
	}
}

func (s *LiteralContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitLiteral(s)
	}
}

func (s *LiteralContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitLiteral(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Literal() (localctx ILiteralContext) {
	this := p
	_ = this

	localctx = NewLiteralContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 34, SLQParserRULE_literal)
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
	{
		p.SetState(199)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&564874098769920) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	return localctx
}

// IUnaryOperatorContext is an interface to support dynamic dispatch.
type IUnaryOperatorContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsUnaryOperatorContext differentiates from other interfaces.
	IsUnaryOperatorContext()
}

type UnaryOperatorContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyUnaryOperatorContext() *UnaryOperatorContext {
	var p = new(UnaryOperatorContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_unaryOperator
	return p
}

func (*UnaryOperatorContext) IsUnaryOperatorContext() {}

func NewUnaryOperatorContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *UnaryOperatorContext {
	var p = new(UnaryOperatorContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_unaryOperator

	return p
}

func (s *UnaryOperatorContext) GetParser() antlr.Parser { return s.parser }
func (s *UnaryOperatorContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *UnaryOperatorContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *UnaryOperatorContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterUnaryOperator(s)
	}
}

func (s *UnaryOperatorContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitUnaryOperator(s)
	}
}

func (s *UnaryOperatorContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitUnaryOperator(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) UnaryOperator() (localctx IUnaryOperatorContext) {
	this := p
	_ = this

	localctx = NewUnaryOperatorContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 36, SLQParserRULE_unaryOperator)
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
	{
		p.SetState(201)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&408944640) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	return localctx
}

func (p *SLQParser) Sempred(localctx antlr.RuleContext, ruleIndex, predIndex int) bool {
	switch ruleIndex {
	case 16:
		var t *ExprContext = nil
		if localctx != nil {
			t = localctx.(*ExprContext)
		}
		return p.Expr_Sempred(t, predIndex)

	default:
		panic("No predicate with index: " + fmt.Sprint(ruleIndex))
	}
}

func (p *SLQParser) Expr_Sempred(localctx antlr.RuleContext, predIndex int) bool {
	this := p
	_ = this

	switch predIndex {
	case 0:
		return p.Precpred(p.GetParserRuleContext(), 8)

	case 1:
		return p.Precpred(p.GetParserRuleContext(), 7)

	case 2:
		return p.Precpred(p.GetParserRuleContext(), 6)

	case 3:
		return p.Precpred(p.GetParserRuleContext(), 5)

	case 4:
		return p.Precpred(p.GetParserRuleContext(), 4)

	case 5:
		return p.Precpred(p.GetParserRuleContext(), 3)

	case 6:
		return p.Precpred(p.GetParserRuleContext(), 2)

	default:
		panic("No predicate with index: " + fmt.Sprint(predIndex))
	}
}

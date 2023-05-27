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
		"", "';'", "'*'", "'sum'", "'avg'", "'max'", "'min'", "'where'", "'join'",
		"'unique'", "'count'", "'.['", "'||'", "'/'", "'%'", "'<<'", "'>>'",
		"'&'", "'&&'", "'~'", "'!'", "", "'group_by'", "'+'", "'-'", "", "",
		"", "'null'", "", "", "'('", "')'", "'['", "']'", "','", "'|'", "':'",
		"", "", "'<='", "'<'", "'>='", "'>'", "'!='", "'=='",
	}
	staticData.symbolicNames = []string{
		"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
		"", "", "", "", "PROPRIETARY_FUNC_NAME", "GROUP_BY", "ORDER_ASC", "ORDER_DESC",
		"ORDER_BY", "ALIAS_RESERVED", "ARG", "NULL", "ID", "WS", "LPAR", "RPAR",
		"LBRA", "RBRA", "COMMA", "PIPE", "COLON", "NN", "NUMBER", "LT_EQ", "LT",
		"GT_EQ", "GT", "NEQ", "EQ", "NAME", "HANDLE", "STRING", "LINECOMMENT",
	}
	staticData.ruleNames = []string{
		"stmtList", "query", "segment", "element", "cmpr", "funcElement", "func",
		"funcName", "join", "joinConstraint", "uniqueFunc", "countFunc", "groupByTerm",
		"groupBy", "orderByTerm", "orderBy", "selector", "selectorElement",
		"alias", "arg", "handleTable", "handle", "rowRange", "expr", "literal",
		"unaryOperator",
	}
	staticData.predictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 1, 49, 263, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2, 4, 7,
		4, 2, 5, 7, 5, 2, 6, 7, 6, 2, 7, 7, 7, 2, 8, 7, 8, 2, 9, 7, 9, 2, 10, 7,
		10, 2, 11, 7, 11, 2, 12, 7, 12, 2, 13, 7, 13, 2, 14, 7, 14, 2, 15, 7, 15,
		2, 16, 7, 16, 2, 17, 7, 17, 2, 18, 7, 18, 2, 19, 7, 19, 2, 20, 7, 20, 2,
		21, 7, 21, 2, 22, 7, 22, 2, 23, 7, 23, 2, 24, 7, 24, 2, 25, 7, 25, 1, 0,
		5, 0, 54, 8, 0, 10, 0, 12, 0, 57, 9, 0, 1, 0, 1, 0, 4, 0, 61, 8, 0, 11,
		0, 12, 0, 62, 1, 0, 5, 0, 66, 8, 0, 10, 0, 12, 0, 69, 9, 0, 1, 0, 5, 0,
		72, 8, 0, 10, 0, 12, 0, 75, 9, 0, 1, 1, 1, 1, 1, 1, 5, 1, 80, 8, 1, 10,
		1, 12, 1, 83, 9, 1, 1, 2, 1, 2, 1, 2, 5, 2, 88, 8, 2, 10, 2, 12, 2, 91,
		9, 2, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3,
		3, 3, 104, 8, 3, 1, 4, 1, 4, 1, 5, 1, 5, 3, 5, 110, 8, 5, 1, 6, 1, 6, 1,
		6, 1, 6, 1, 6, 5, 6, 117, 8, 6, 10, 6, 12, 6, 120, 9, 6, 1, 6, 3, 6, 123,
		8, 6, 1, 6, 1, 6, 1, 7, 1, 7, 1, 8, 1, 8, 1, 8, 1, 8, 1, 8, 1, 9, 1, 9,
		1, 9, 1, 9, 1, 9, 3, 9, 139, 8, 9, 1, 10, 1, 10, 1, 11, 1, 11, 1, 11, 3,
		11, 146, 8, 11, 1, 11, 3, 11, 149, 8, 11, 1, 11, 3, 11, 152, 8, 11, 1,
		12, 1, 12, 3, 12, 156, 8, 12, 1, 13, 1, 13, 1, 13, 1, 13, 1, 13, 5, 13,
		163, 8, 13, 10, 13, 12, 13, 166, 9, 13, 1, 13, 1, 13, 1, 14, 1, 14, 3,
		14, 172, 8, 14, 1, 15, 1, 15, 1, 15, 1, 15, 1, 15, 5, 15, 179, 8, 15, 10,
		15, 12, 15, 182, 9, 15, 1, 15, 1, 15, 1, 16, 1, 16, 3, 16, 188, 8, 16,
		1, 17, 1, 17, 3, 17, 192, 8, 17, 1, 18, 1, 18, 1, 18, 3, 18, 197, 8, 18,
		1, 19, 1, 19, 1, 20, 1, 20, 1, 20, 1, 21, 1, 21, 1, 22, 1, 22, 1, 22, 1,
		22, 1, 22, 1, 22, 1, 22, 1, 22, 1, 22, 3, 22, 215, 8, 22, 1, 22, 1, 22,
		1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 3, 23, 227, 8,
		23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23,
		1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 3, 23, 248,
		8, 23, 1, 23, 1, 23, 1, 23, 1, 23, 5, 23, 254, 8, 23, 10, 23, 12, 23, 257,
		9, 23, 1, 24, 1, 24, 1, 25, 1, 25, 1, 25, 0, 1, 46, 26, 0, 2, 4, 6, 8,
		10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30, 32, 34, 36, 38, 40, 42, 44,
		46, 48, 50, 0, 9, 1, 0, 40, 45, 2, 0, 3, 7, 21, 21, 1, 0, 23, 24, 2, 0,
		27, 27, 29, 29, 2, 0, 2, 2, 13, 14, 1, 0, 15, 17, 1, 0, 40, 43, 3, 0, 28,
		28, 38, 39, 48, 48, 2, 0, 19, 20, 23, 24, 284, 0, 55, 1, 0, 0, 0, 2, 76,
		1, 0, 0, 0, 4, 84, 1, 0, 0, 0, 6, 103, 1, 0, 0, 0, 8, 105, 1, 0, 0, 0,
		10, 107, 1, 0, 0, 0, 12, 111, 1, 0, 0, 0, 14, 126, 1, 0, 0, 0, 16, 128,
		1, 0, 0, 0, 18, 138, 1, 0, 0, 0, 20, 140, 1, 0, 0, 0, 22, 142, 1, 0, 0,
		0, 24, 155, 1, 0, 0, 0, 26, 157, 1, 0, 0, 0, 28, 169, 1, 0, 0, 0, 30, 173,
		1, 0, 0, 0, 32, 185, 1, 0, 0, 0, 34, 189, 1, 0, 0, 0, 36, 196, 1, 0, 0,
		0, 38, 198, 1, 0, 0, 0, 40, 200, 1, 0, 0, 0, 42, 203, 1, 0, 0, 0, 44, 205,
		1, 0, 0, 0, 46, 226, 1, 0, 0, 0, 48, 258, 1, 0, 0, 0, 50, 260, 1, 0, 0,
		0, 52, 54, 5, 1, 0, 0, 53, 52, 1, 0, 0, 0, 54, 57, 1, 0, 0, 0, 55, 53,
		1, 0, 0, 0, 55, 56, 1, 0, 0, 0, 56, 58, 1, 0, 0, 0, 57, 55, 1, 0, 0, 0,
		58, 67, 3, 2, 1, 0, 59, 61, 5, 1, 0, 0, 60, 59, 1, 0, 0, 0, 61, 62, 1,
		0, 0, 0, 62, 60, 1, 0, 0, 0, 62, 63, 1, 0, 0, 0, 63, 64, 1, 0, 0, 0, 64,
		66, 3, 2, 1, 0, 65, 60, 1, 0, 0, 0, 66, 69, 1, 0, 0, 0, 67, 65, 1, 0, 0,
		0, 67, 68, 1, 0, 0, 0, 68, 73, 1, 0, 0, 0, 69, 67, 1, 0, 0, 0, 70, 72,
		5, 1, 0, 0, 71, 70, 1, 0, 0, 0, 72, 75, 1, 0, 0, 0, 73, 71, 1, 0, 0, 0,
		73, 74, 1, 0, 0, 0, 74, 1, 1, 0, 0, 0, 75, 73, 1, 0, 0, 0, 76, 81, 3, 4,
		2, 0, 77, 78, 5, 36, 0, 0, 78, 80, 3, 4, 2, 0, 79, 77, 1, 0, 0, 0, 80,
		83, 1, 0, 0, 0, 81, 79, 1, 0, 0, 0, 81, 82, 1, 0, 0, 0, 82, 3, 1, 0, 0,
		0, 83, 81, 1, 0, 0, 0, 84, 89, 3, 6, 3, 0, 85, 86, 5, 35, 0, 0, 86, 88,
		3, 6, 3, 0, 87, 85, 1, 0, 0, 0, 88, 91, 1, 0, 0, 0, 89, 87, 1, 0, 0, 0,
		89, 90, 1, 0, 0, 0, 90, 5, 1, 0, 0, 0, 91, 89, 1, 0, 0, 0, 92, 104, 3,
		40, 20, 0, 93, 104, 3, 42, 21, 0, 94, 104, 3, 34, 17, 0, 95, 104, 3, 16,
		8, 0, 96, 104, 3, 26, 13, 0, 97, 104, 3, 30, 15, 0, 98, 104, 3, 44, 22,
		0, 99, 104, 3, 20, 10, 0, 100, 104, 3, 22, 11, 0, 101, 104, 3, 10, 5, 0,
		102, 104, 3, 46, 23, 0, 103, 92, 1, 0, 0, 0, 103, 93, 1, 0, 0, 0, 103,
		94, 1, 0, 0, 0, 103, 95, 1, 0, 0, 0, 103, 96, 1, 0, 0, 0, 103, 97, 1, 0,
		0, 0, 103, 98, 1, 0, 0, 0, 103, 99, 1, 0, 0, 0, 103, 100, 1, 0, 0, 0, 103,
		101, 1, 0, 0, 0, 103, 102, 1, 0, 0, 0, 104, 7, 1, 0, 0, 0, 105, 106, 7,
		0, 0, 0, 106, 9, 1, 0, 0, 0, 107, 109, 3, 12, 6, 0, 108, 110, 3, 36, 18,
		0, 109, 108, 1, 0, 0, 0, 109, 110, 1, 0, 0, 0, 110, 11, 1, 0, 0, 0, 111,
		112, 3, 14, 7, 0, 112, 122, 5, 31, 0, 0, 113, 118, 3, 46, 23, 0, 114, 115,
		5, 35, 0, 0, 115, 117, 3, 46, 23, 0, 116, 114, 1, 0, 0, 0, 117, 120, 1,
		0, 0, 0, 118, 116, 1, 0, 0, 0, 118, 119, 1, 0, 0, 0, 119, 123, 1, 0, 0,
		0, 120, 118, 1, 0, 0, 0, 121, 123, 5, 2, 0, 0, 122, 113, 1, 0, 0, 0, 122,
		121, 1, 0, 0, 0, 122, 123, 1, 0, 0, 0, 123, 124, 1, 0, 0, 0, 124, 125,
		5, 32, 0, 0, 125, 13, 1, 0, 0, 0, 126, 127, 7, 1, 0, 0, 127, 15, 1, 0,
		0, 0, 128, 129, 5, 8, 0, 0, 129, 130, 5, 31, 0, 0, 130, 131, 3, 18, 9,
		0, 131, 132, 5, 32, 0, 0, 132, 17, 1, 0, 0, 0, 133, 134, 3, 32, 16, 0,
		134, 135, 3, 8, 4, 0, 135, 136, 3, 32, 16, 0, 136, 139, 1, 0, 0, 0, 137,
		139, 3, 32, 16, 0, 138, 133, 1, 0, 0, 0, 138, 137, 1, 0, 0, 0, 139, 19,
		1, 0, 0, 0, 140, 141, 5, 9, 0, 0, 141, 21, 1, 0, 0, 0, 142, 148, 5, 10,
		0, 0, 143, 145, 5, 31, 0, 0, 144, 146, 3, 32, 16, 0, 145, 144, 1, 0, 0,
		0, 145, 146, 1, 0, 0, 0, 146, 147, 1, 0, 0, 0, 147, 149, 5, 32, 0, 0, 148,
		143, 1, 0, 0, 0, 148, 149, 1, 0, 0, 0, 149, 151, 1, 0, 0, 0, 150, 152,
		3, 36, 18, 0, 151, 150, 1, 0, 0, 0, 151, 152, 1, 0, 0, 0, 152, 23, 1, 0,
		0, 0, 153, 156, 3, 32, 16, 0, 154, 156, 3, 12, 6, 0, 155, 153, 1, 0, 0,
		0, 155, 154, 1, 0, 0, 0, 156, 25, 1, 0, 0, 0, 157, 158, 5, 22, 0, 0, 158,
		159, 5, 31, 0, 0, 159, 164, 3, 24, 12, 0, 160, 161, 5, 35, 0, 0, 161, 163,
		3, 24, 12, 0, 162, 160, 1, 0, 0, 0, 163, 166, 1, 0, 0, 0, 164, 162, 1,
		0, 0, 0, 164, 165, 1, 0, 0, 0, 165, 167, 1, 0, 0, 0, 166, 164, 1, 0, 0,
		0, 167, 168, 5, 32, 0, 0, 168, 27, 1, 0, 0, 0, 169, 171, 3, 32, 16, 0,
		170, 172, 7, 2, 0, 0, 171, 170, 1, 0, 0, 0, 171, 172, 1, 0, 0, 0, 172,
		29, 1, 0, 0, 0, 173, 174, 5, 25, 0, 0, 174, 175, 5, 31, 0, 0, 175, 180,
		3, 28, 14, 0, 176, 177, 5, 35, 0, 0, 177, 179, 3, 28, 14, 0, 178, 176,
		1, 0, 0, 0, 179, 182, 1, 0, 0, 0, 180, 178, 1, 0, 0, 0, 180, 181, 1, 0,
		0, 0, 181, 183, 1, 0, 0, 0, 182, 180, 1, 0, 0, 0, 183, 184, 5, 32, 0, 0,
		184, 31, 1, 0, 0, 0, 185, 187, 5, 46, 0, 0, 186, 188, 5, 46, 0, 0, 187,
		186, 1, 0, 0, 0, 187, 188, 1, 0, 0, 0, 188, 33, 1, 0, 0, 0, 189, 191, 3,
		32, 16, 0, 190, 192, 3, 36, 18, 0, 191, 190, 1, 0, 0, 0, 191, 192, 1, 0,
		0, 0, 192, 35, 1, 0, 0, 0, 193, 197, 5, 26, 0, 0, 194, 195, 5, 37, 0, 0,
		195, 197, 7, 3, 0, 0, 196, 193, 1, 0, 0, 0, 196, 194, 1, 0, 0, 0, 197,
		37, 1, 0, 0, 0, 198, 199, 5, 27, 0, 0, 199, 39, 1, 0, 0, 0, 200, 201, 5,
		47, 0, 0, 201, 202, 5, 46, 0, 0, 202, 41, 1, 0, 0, 0, 203, 204, 5, 47,
		0, 0, 204, 43, 1, 0, 0, 0, 205, 214, 5, 11, 0, 0, 206, 207, 5, 38, 0, 0,
		207, 208, 5, 37, 0, 0, 208, 215, 5, 38, 0, 0, 209, 210, 5, 38, 0, 0, 210,
		215, 5, 37, 0, 0, 211, 212, 5, 37, 0, 0, 212, 215, 5, 38, 0, 0, 213, 215,
		5, 38, 0, 0, 214, 206, 1, 0, 0, 0, 214, 209, 1, 0, 0, 0, 214, 211, 1, 0,
		0, 0, 214, 213, 1, 0, 0, 0, 214, 215, 1, 0, 0, 0, 215, 216, 1, 0, 0, 0,
		216, 217, 5, 34, 0, 0, 217, 45, 1, 0, 0, 0, 218, 219, 6, 23, -1, 0, 219,
		227, 3, 32, 16, 0, 220, 227, 3, 48, 24, 0, 221, 227, 3, 38, 19, 0, 222,
		223, 3, 50, 25, 0, 223, 224, 3, 46, 23, 9, 224, 227, 1, 0, 0, 0, 225, 227,
		3, 12, 6, 0, 226, 218, 1, 0, 0, 0, 226, 220, 1, 0, 0, 0, 226, 221, 1, 0,
		0, 0, 226, 222, 1, 0, 0, 0, 226, 225, 1, 0, 0, 0, 227, 255, 1, 0, 0, 0,
		228, 229, 10, 8, 0, 0, 229, 230, 5, 12, 0, 0, 230, 254, 3, 46, 23, 9, 231,
		232, 10, 7, 0, 0, 232, 233, 7, 4, 0, 0, 233, 254, 3, 46, 23, 8, 234, 235,
		10, 6, 0, 0, 235, 236, 7, 2, 0, 0, 236, 254, 3, 46, 23, 7, 237, 238, 10,
		5, 0, 0, 238, 239, 7, 5, 0, 0, 239, 254, 3, 46, 23, 6, 240, 241, 10, 4,
		0, 0, 241, 242, 7, 6, 0, 0, 242, 254, 3, 46, 23, 5, 243, 247, 10, 3, 0,
		0, 244, 248, 5, 45, 0, 0, 245, 248, 5, 44, 0, 0, 246, 248, 1, 0, 0, 0,
		247, 244, 1, 0, 0, 0, 247, 245, 1, 0, 0, 0, 247, 246, 1, 0, 0, 0, 248,
		249, 1, 0, 0, 0, 249, 254, 3, 46, 23, 4, 250, 251, 10, 2, 0, 0, 251, 252,
		5, 18, 0, 0, 252, 254, 3, 46, 23, 3, 253, 228, 1, 0, 0, 0, 253, 231, 1,
		0, 0, 0, 253, 234, 1, 0, 0, 0, 253, 237, 1, 0, 0, 0, 253, 240, 1, 0, 0,
		0, 253, 243, 1, 0, 0, 0, 253, 250, 1, 0, 0, 0, 254, 257, 1, 0, 0, 0, 255,
		253, 1, 0, 0, 0, 255, 256, 1, 0, 0, 0, 256, 47, 1, 0, 0, 0, 257, 255, 1,
		0, 0, 0, 258, 259, 7, 7, 0, 0, 259, 49, 1, 0, 0, 0, 260, 261, 7, 8, 0,
		0, 261, 51, 1, 0, 0, 0, 26, 55, 62, 67, 73, 81, 89, 103, 109, 118, 122,
		138, 145, 148, 151, 155, 164, 171, 180, 187, 191, 196, 214, 226, 247, 253,
		255,
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
	SLQParserEOF                   = antlr.TokenEOF
	SLQParserT__0                  = 1
	SLQParserT__1                  = 2
	SLQParserT__2                  = 3
	SLQParserT__3                  = 4
	SLQParserT__4                  = 5
	SLQParserT__5                  = 6
	SLQParserT__6                  = 7
	SLQParserT__7                  = 8
	SLQParserT__8                  = 9
	SLQParserT__9                  = 10
	SLQParserT__10                 = 11
	SLQParserT__11                 = 12
	SLQParserT__12                 = 13
	SLQParserT__13                 = 14
	SLQParserT__14                 = 15
	SLQParserT__15                 = 16
	SLQParserT__16                 = 17
	SLQParserT__17                 = 18
	SLQParserT__18                 = 19
	SLQParserT__19                 = 20
	SLQParserPROPRIETARY_FUNC_NAME = 21
	SLQParserGROUP_BY              = 22
	SLQParserORDER_ASC             = 23
	SLQParserORDER_DESC            = 24
	SLQParserORDER_BY              = 25
	SLQParserALIAS_RESERVED        = 26
	SLQParserARG                   = 27
	SLQParserNULL                  = 28
	SLQParserID                    = 29
	SLQParserWS                    = 30
	SLQParserLPAR                  = 31
	SLQParserRPAR                  = 32
	SLQParserLBRA                  = 33
	SLQParserRBRA                  = 34
	SLQParserCOMMA                 = 35
	SLQParserPIPE                  = 36
	SLQParserCOLON                 = 37
	SLQParserNN                    = 38
	SLQParserNUMBER                = 39
	SLQParserLT_EQ                 = 40
	SLQParserLT                    = 41
	SLQParserGT_EQ                 = 42
	SLQParserGT                    = 43
	SLQParserNEQ                   = 44
	SLQParserEQ                    = 45
	SLQParserNAME                  = 46
	SLQParserHANDLE                = 47
	SLQParserSTRING                = 48
	SLQParserLINECOMMENT           = 49
)

// SLQParser rules.
const (
	SLQParserRULE_stmtList        = 0
	SLQParserRULE_query           = 1
	SLQParserRULE_segment         = 2
	SLQParserRULE_element         = 3
	SLQParserRULE_cmpr            = 4
	SLQParserRULE_funcElement     = 5
	SLQParserRULE_func            = 6
	SLQParserRULE_funcName        = 7
	SLQParserRULE_join            = 8
	SLQParserRULE_joinConstraint  = 9
	SLQParserRULE_uniqueFunc      = 10
	SLQParserRULE_countFunc       = 11
	SLQParserRULE_groupByTerm     = 12
	SLQParserRULE_groupBy         = 13
	SLQParserRULE_orderByTerm     = 14
	SLQParserRULE_orderBy         = 15
	SLQParserRULE_selector        = 16
	SLQParserRULE_selectorElement = 17
	SLQParserRULE_alias           = 18
	SLQParserRULE_arg             = 19
	SLQParserRULE_handleTable     = 20
	SLQParserRULE_handle          = 21
	SLQParserRULE_rowRange        = 22
	SLQParserRULE_expr            = 23
	SLQParserRULE_literal         = 24
	SLQParserRULE_unaryOperator   = 25
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
	p.SetState(55)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserT__0 {
		{
			p.SetState(52)
			p.Match(SLQParserT__0)
		}

		p.SetState(57)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(58)
		p.Query()
	}
	p.SetState(67)
	p.GetErrorHandler().Sync(p)
	_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 2, p.GetParserRuleContext())

	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			p.SetState(60)
			p.GetErrorHandler().Sync(p)
			_la = p.GetTokenStream().LA(1)

			for ok := true; ok; ok = _la == SLQParserT__0 {
				{
					p.SetState(59)
					p.Match(SLQParserT__0)
				}

				p.SetState(62)
				p.GetErrorHandler().Sync(p)
				_la = p.GetTokenStream().LA(1)
			}
			{
				p.SetState(64)
				p.Query()
			}

		}
		p.SetState(69)
		p.GetErrorHandler().Sync(p)
		_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 2, p.GetParserRuleContext())
	}
	p.SetState(73)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserT__0 {
		{
			p.SetState(70)
			p.Match(SLQParserT__0)
		}

		p.SetState(75)
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
		p.SetState(76)
		p.Segment()
	}
	p.SetState(81)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserPIPE {
		{
			p.SetState(77)
			p.Match(SLQParserPIPE)
		}
		{
			p.SetState(78)
			p.Segment()
		}

		p.SetState(83)
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
		p.SetState(84)
		p.Element()
	}

	p.SetState(89)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserCOMMA {
		{
			p.SetState(85)
			p.Match(SLQParserCOMMA)
		}
		{
			p.SetState(86)
			p.Element()
		}

		p.SetState(91)
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
	HandleTable() IHandleTableContext
	Handle() IHandleContext
	SelectorElement() ISelectorElementContext
	Join() IJoinContext
	GroupBy() IGroupByContext
	OrderBy() IOrderByContext
	RowRange() IRowRangeContext
	UniqueFunc() IUniqueFuncContext
	CountFunc() ICountFuncContext
	FuncElement() IFuncElementContext
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

func (s *ElementContext) HandleTable() IHandleTableContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IHandleTableContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IHandleTableContext)
}

func (s *ElementContext) Handle() IHandleContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IHandleContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IHandleContext)
}

func (s *ElementContext) SelectorElement() ISelectorElementContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISelectorElementContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISelectorElementContext)
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

func (s *ElementContext) GroupBy() IGroupByContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IGroupByContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IGroupByContext)
}

func (s *ElementContext) OrderBy() IOrderByContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IOrderByContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IOrderByContext)
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

func (s *ElementContext) UniqueFunc() IUniqueFuncContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IUniqueFuncContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IUniqueFuncContext)
}

func (s *ElementContext) CountFunc() ICountFuncContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ICountFuncContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ICountFuncContext)
}

func (s *ElementContext) FuncElement() IFuncElementContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFuncElementContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFuncElementContext)
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

	p.SetState(103)
	p.GetErrorHandler().Sync(p)
	switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 6, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(92)
			p.HandleTable()
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(93)
			p.Handle()
		}

	case 3:
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(94)
			p.SelectorElement()
		}

	case 4:
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(95)
			p.Join()
		}

	case 5:
		p.EnterOuterAlt(localctx, 5)
		{
			p.SetState(96)
			p.GroupBy()
		}

	case 6:
		p.EnterOuterAlt(localctx, 6)
		{
			p.SetState(97)
			p.OrderBy()
		}

	case 7:
		p.EnterOuterAlt(localctx, 7)
		{
			p.SetState(98)
			p.RowRange()
		}

	case 8:
		p.EnterOuterAlt(localctx, 8)
		{
			p.SetState(99)
			p.UniqueFunc()
		}

	case 9:
		p.EnterOuterAlt(localctx, 9)
		{
			p.SetState(100)
			p.CountFunc()
		}

	case 10:
		p.EnterOuterAlt(localctx, 10)
		{
			p.SetState(101)
			p.FuncElement()
		}

	case 11:
		p.EnterOuterAlt(localctx, 11)
		{
			p.SetState(102)
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
		p.SetState(105)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&69269232549888) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	return localctx
}

// IFuncElementContext is an interface to support dynamic dispatch.
type IFuncElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Func_() IFuncContext
	Alias() IAliasContext

	// IsFuncElementContext differentiates from other interfaces.
	IsFuncElementContext()
}

type FuncElementContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFuncElementContext() *FuncElementContext {
	var p = new(FuncElementContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_funcElement
	return p
}

func (*FuncElementContext) IsFuncElementContext() {}

func NewFuncElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FuncElementContext {
	var p = new(FuncElementContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_funcElement

	return p
}

func (s *FuncElementContext) GetParser() antlr.Parser { return s.parser }

func (s *FuncElementContext) Func_() IFuncContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFuncContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFuncContext)
}

func (s *FuncElementContext) Alias() IAliasContext {
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

func (s *FuncElementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FuncElementContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FuncElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterFuncElement(s)
	}
}

func (s *FuncElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitFuncElement(s)
	}
}

func (s *FuncElementContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitFuncElement(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) FuncElement() (localctx IFuncElementContext) {
	this := p
	_ = this

	localctx = NewFuncElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 10, SLQParserRULE_funcElement)
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
		p.SetState(107)
		p.Func_()
	}
	p.SetState(109)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserALIAS_RESERVED || _la == SLQParserCOLON {
		{
			p.SetState(108)
			p.Alias()
		}

	}

	return localctx
}

// IFuncContext is an interface to support dynamic dispatch.
type IFuncContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	FuncName() IFuncNameContext
	LPAR() antlr.TerminalNode
	RPAR() antlr.TerminalNode
	AllExpr() []IExprContext
	Expr(i int) IExprContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsFuncContext differentiates from other interfaces.
	IsFuncContext()
}

type FuncContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFuncContext() *FuncContext {
	var p = new(FuncContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_func
	return p
}

func (*FuncContext) IsFuncContext() {}

func NewFuncContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FuncContext {
	var p = new(FuncContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_func

	return p
}

func (s *FuncContext) GetParser() antlr.Parser { return s.parser }

func (s *FuncContext) FuncName() IFuncNameContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFuncNameContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFuncNameContext)
}

func (s *FuncContext) LPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserLPAR, 0)
}

func (s *FuncContext) RPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserRPAR, 0)
}

func (s *FuncContext) AllExpr() []IExprContext {
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

func (s *FuncContext) Expr(i int) IExprContext {
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

func (s *FuncContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(SLQParserCOMMA)
}

func (s *FuncContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserCOMMA, i)
}

func (s *FuncContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FuncContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FuncContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterFunc(s)
	}
}

func (s *FuncContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitFunc(s)
	}
}

func (s *FuncContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitFunc(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Func_() (localctx IFuncContext) {
	this := p
	_ = this

	localctx = NewFuncContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, SLQParserRULE_func)
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
		p.SetState(111)
		p.FuncName()
	}
	{
		p.SetState(112)
		p.Match(SLQParserLPAR)
	}
	p.SetState(122)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case SLQParserT__2, SLQParserT__3, SLQParserT__4, SLQParserT__5, SLQParserT__6, SLQParserT__18, SLQParserT__19, SLQParserPROPRIETARY_FUNC_NAME, SLQParserORDER_ASC, SLQParserORDER_DESC, SLQParserARG, SLQParserNULL, SLQParserNN, SLQParserNUMBER, SLQParserNAME, SLQParserSTRING:
		{
			p.SetState(113)
			p.expr(0)
		}
		p.SetState(118)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		for _la == SLQParserCOMMA {
			{
				p.SetState(114)
				p.Match(SLQParserCOMMA)
			}
			{
				p.SetState(115)
				p.expr(0)
			}

			p.SetState(120)
			p.GetErrorHandler().Sync(p)
			_la = p.GetTokenStream().LA(1)
		}

	case SLQParserT__1:
		{
			p.SetState(121)
			p.Match(SLQParserT__1)
		}

	case SLQParserRPAR:

	default:
	}
	{
		p.SetState(124)
		p.Match(SLQParserRPAR)
	}

	return localctx
}

// IFuncNameContext is an interface to support dynamic dispatch.
type IFuncNameContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	PROPRIETARY_FUNC_NAME() antlr.TerminalNode

	// IsFuncNameContext differentiates from other interfaces.
	IsFuncNameContext()
}

type FuncNameContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFuncNameContext() *FuncNameContext {
	var p = new(FuncNameContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_funcName
	return p
}

func (*FuncNameContext) IsFuncNameContext() {}

func NewFuncNameContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FuncNameContext {
	var p = new(FuncNameContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_funcName

	return p
}

func (s *FuncNameContext) GetParser() antlr.Parser { return s.parser }

func (s *FuncNameContext) PROPRIETARY_FUNC_NAME() antlr.TerminalNode {
	return s.GetToken(SLQParserPROPRIETARY_FUNC_NAME, 0)
}

func (s *FuncNameContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FuncNameContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FuncNameContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterFuncName(s)
	}
}

func (s *FuncNameContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitFuncName(s)
	}
}

func (s *FuncNameContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitFuncName(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) FuncName() (localctx IFuncNameContext) {
	this := p
	_ = this

	localctx = NewFuncNameContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 14, SLQParserRULE_funcName)
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
		p.SetState(126)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&2097400) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
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
	p.EnterRule(localctx, 16, SLQParserRULE_join)

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
		p.SetState(128)
		p.Match(SLQParserT__7)
	}

	{
		p.SetState(129)
		p.Match(SLQParserLPAR)
	}
	{
		p.SetState(130)
		p.JoinConstraint()
	}
	{
		p.SetState(131)
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
	AllSelector() []ISelectorContext
	Selector(i int) ISelectorContext
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

func (s *JoinConstraintContext) AllSelector() []ISelectorContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ISelectorContext); ok {
			len++
		}
	}

	tst := make([]ISelectorContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ISelectorContext); ok {
			tst[i] = t.(ISelectorContext)
			i++
		}
	}

	return tst
}

func (s *JoinConstraintContext) Selector(i int) ISelectorContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISelectorContext); ok {
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

	return t.(ISelectorContext)
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
	p.EnterRule(localctx, 18, SLQParserRULE_joinConstraint)

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

	p.SetState(138)
	p.GetErrorHandler().Sync(p)
	switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 10, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(133)
			p.Selector()
		}
		{
			p.SetState(134)
			p.Cmpr()
		}
		{
			p.SetState(135)
			p.Selector()
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(137)
			p.Selector()
		}

	}

	return localctx
}

// IUniqueFuncContext is an interface to support dynamic dispatch.
type IUniqueFuncContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsUniqueFuncContext differentiates from other interfaces.
	IsUniqueFuncContext()
}

type UniqueFuncContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyUniqueFuncContext() *UniqueFuncContext {
	var p = new(UniqueFuncContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_uniqueFunc
	return p
}

func (*UniqueFuncContext) IsUniqueFuncContext() {}

func NewUniqueFuncContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *UniqueFuncContext {
	var p = new(UniqueFuncContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_uniqueFunc

	return p
}

func (s *UniqueFuncContext) GetParser() antlr.Parser { return s.parser }
func (s *UniqueFuncContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *UniqueFuncContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *UniqueFuncContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterUniqueFunc(s)
	}
}

func (s *UniqueFuncContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitUniqueFunc(s)
	}
}

func (s *UniqueFuncContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitUniqueFunc(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) UniqueFunc() (localctx IUniqueFuncContext) {
	this := p
	_ = this

	localctx = NewUniqueFuncContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 20, SLQParserRULE_uniqueFunc)

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
		p.Match(SLQParserT__8)
	}

	return localctx
}

// ICountFuncContext is an interface to support dynamic dispatch.
type ICountFuncContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	LPAR() antlr.TerminalNode
	RPAR() antlr.TerminalNode
	Alias() IAliasContext
	Selector() ISelectorContext

	// IsCountFuncContext differentiates from other interfaces.
	IsCountFuncContext()
}

type CountFuncContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyCountFuncContext() *CountFuncContext {
	var p = new(CountFuncContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_countFunc
	return p
}

func (*CountFuncContext) IsCountFuncContext() {}

func NewCountFuncContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *CountFuncContext {
	var p = new(CountFuncContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_countFunc

	return p
}

func (s *CountFuncContext) GetParser() antlr.Parser { return s.parser }

func (s *CountFuncContext) LPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserLPAR, 0)
}

func (s *CountFuncContext) RPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserRPAR, 0)
}

func (s *CountFuncContext) Alias() IAliasContext {
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

func (s *CountFuncContext) Selector() ISelectorContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISelectorContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISelectorContext)
}

func (s *CountFuncContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *CountFuncContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *CountFuncContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterCountFunc(s)
	}
}

func (s *CountFuncContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitCountFunc(s)
	}
}

func (s *CountFuncContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitCountFunc(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) CountFunc() (localctx ICountFuncContext) {
	this := p
	_ = this

	localctx = NewCountFuncContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 22, SLQParserRULE_countFunc)
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
		p.SetState(142)
		p.Match(SLQParserT__9)
	}
	p.SetState(148)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserLPAR {
		{
			p.SetState(143)
			p.Match(SLQParserLPAR)
		}
		p.SetState(145)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		if _la == SLQParserNAME {
			{
				p.SetState(144)
				p.Selector()
			}

		}
		{
			p.SetState(147)
			p.Match(SLQParserRPAR)
		}

	}
	p.SetState(151)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserALIAS_RESERVED || _la == SLQParserCOLON {
		{
			p.SetState(150)
			p.Alias()
		}

	}

	return localctx
}

// IGroupByTermContext is an interface to support dynamic dispatch.
type IGroupByTermContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Selector() ISelectorContext
	Func_() IFuncContext

	// IsGroupByTermContext differentiates from other interfaces.
	IsGroupByTermContext()
}

type GroupByTermContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyGroupByTermContext() *GroupByTermContext {
	var p = new(GroupByTermContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_groupByTerm
	return p
}

func (*GroupByTermContext) IsGroupByTermContext() {}

func NewGroupByTermContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *GroupByTermContext {
	var p = new(GroupByTermContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_groupByTerm

	return p
}

func (s *GroupByTermContext) GetParser() antlr.Parser { return s.parser }

func (s *GroupByTermContext) Selector() ISelectorContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISelectorContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISelectorContext)
}

func (s *GroupByTermContext) Func_() IFuncContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFuncContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFuncContext)
}

func (s *GroupByTermContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *GroupByTermContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *GroupByTermContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterGroupByTerm(s)
	}
}

func (s *GroupByTermContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitGroupByTerm(s)
	}
}

func (s *GroupByTermContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitGroupByTerm(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) GroupByTerm() (localctx IGroupByTermContext) {
	this := p
	_ = this

	localctx = NewGroupByTermContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 24, SLQParserRULE_groupByTerm)

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

	p.SetState(155)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case SLQParserNAME:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(153)
			p.Selector()
		}

	case SLQParserT__2, SLQParserT__3, SLQParserT__4, SLQParserT__5, SLQParserT__6, SLQParserPROPRIETARY_FUNC_NAME:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(154)
			p.Func_()
		}

	default:
		panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
	}

	return localctx
}

// IGroupByContext is an interface to support dynamic dispatch.
type IGroupByContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	GROUP_BY() antlr.TerminalNode
	LPAR() antlr.TerminalNode
	AllGroupByTerm() []IGroupByTermContext
	GroupByTerm(i int) IGroupByTermContext
	RPAR() antlr.TerminalNode
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsGroupByContext differentiates from other interfaces.
	IsGroupByContext()
}

type GroupByContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyGroupByContext() *GroupByContext {
	var p = new(GroupByContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_groupBy
	return p
}

func (*GroupByContext) IsGroupByContext() {}

func NewGroupByContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *GroupByContext {
	var p = new(GroupByContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_groupBy

	return p
}

func (s *GroupByContext) GetParser() antlr.Parser { return s.parser }

func (s *GroupByContext) GROUP_BY() antlr.TerminalNode {
	return s.GetToken(SLQParserGROUP_BY, 0)
}

func (s *GroupByContext) LPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserLPAR, 0)
}

func (s *GroupByContext) AllGroupByTerm() []IGroupByTermContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IGroupByTermContext); ok {
			len++
		}
	}

	tst := make([]IGroupByTermContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IGroupByTermContext); ok {
			tst[i] = t.(IGroupByTermContext)
			i++
		}
	}

	return tst
}

func (s *GroupByContext) GroupByTerm(i int) IGroupByTermContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IGroupByTermContext); ok {
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

	return t.(IGroupByTermContext)
}

func (s *GroupByContext) RPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserRPAR, 0)
}

func (s *GroupByContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(SLQParserCOMMA)
}

func (s *GroupByContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserCOMMA, i)
}

func (s *GroupByContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *GroupByContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *GroupByContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterGroupBy(s)
	}
}

func (s *GroupByContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitGroupBy(s)
	}
}

func (s *GroupByContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitGroupBy(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) GroupBy() (localctx IGroupByContext) {
	this := p
	_ = this

	localctx = NewGroupByContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 26, SLQParserRULE_groupBy)
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
		p.SetState(157)
		p.Match(SLQParserGROUP_BY)
	}
	{
		p.SetState(158)
		p.Match(SLQParserLPAR)
	}
	{
		p.SetState(159)
		p.GroupByTerm()
	}
	p.SetState(164)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserCOMMA {
		{
			p.SetState(160)
			p.Match(SLQParserCOMMA)
		}
		{
			p.SetState(161)
			p.GroupByTerm()
		}

		p.SetState(166)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(167)
		p.Match(SLQParserRPAR)
	}

	return localctx
}

// IOrderByTermContext is an interface to support dynamic dispatch.
type IOrderByTermContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Selector() ISelectorContext
	ORDER_ASC() antlr.TerminalNode
	ORDER_DESC() antlr.TerminalNode

	// IsOrderByTermContext differentiates from other interfaces.
	IsOrderByTermContext()
}

type OrderByTermContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyOrderByTermContext() *OrderByTermContext {
	var p = new(OrderByTermContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_orderByTerm
	return p
}

func (*OrderByTermContext) IsOrderByTermContext() {}

func NewOrderByTermContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *OrderByTermContext {
	var p = new(OrderByTermContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_orderByTerm

	return p
}

func (s *OrderByTermContext) GetParser() antlr.Parser { return s.parser }

func (s *OrderByTermContext) Selector() ISelectorContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISelectorContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISelectorContext)
}

func (s *OrderByTermContext) ORDER_ASC() antlr.TerminalNode {
	return s.GetToken(SLQParserORDER_ASC, 0)
}

func (s *OrderByTermContext) ORDER_DESC() antlr.TerminalNode {
	return s.GetToken(SLQParserORDER_DESC, 0)
}

func (s *OrderByTermContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *OrderByTermContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *OrderByTermContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterOrderByTerm(s)
	}
}

func (s *OrderByTermContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitOrderByTerm(s)
	}
}

func (s *OrderByTermContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitOrderByTerm(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) OrderByTerm() (localctx IOrderByTermContext) {
	this := p
	_ = this

	localctx = NewOrderByTermContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 28, SLQParserRULE_orderByTerm)
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
		p.SetState(169)
		p.Selector()
	}
	p.SetState(171)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserORDER_ASC || _la == SLQParserORDER_DESC {
		{
			p.SetState(170)
			_la = p.GetTokenStream().LA(1)

			if !(_la == SLQParserORDER_ASC || _la == SLQParserORDER_DESC) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}

	}

	return localctx
}

// IOrderByContext is an interface to support dynamic dispatch.
type IOrderByContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	ORDER_BY() antlr.TerminalNode
	LPAR() antlr.TerminalNode
	AllOrderByTerm() []IOrderByTermContext
	OrderByTerm(i int) IOrderByTermContext
	RPAR() antlr.TerminalNode
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsOrderByContext differentiates from other interfaces.
	IsOrderByContext()
}

type OrderByContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyOrderByContext() *OrderByContext {
	var p = new(OrderByContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_orderBy
	return p
}

func (*OrderByContext) IsOrderByContext() {}

func NewOrderByContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *OrderByContext {
	var p = new(OrderByContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_orderBy

	return p
}

func (s *OrderByContext) GetParser() antlr.Parser { return s.parser }

func (s *OrderByContext) ORDER_BY() antlr.TerminalNode {
	return s.GetToken(SLQParserORDER_BY, 0)
}

func (s *OrderByContext) LPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserLPAR, 0)
}

func (s *OrderByContext) AllOrderByTerm() []IOrderByTermContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IOrderByTermContext); ok {
			len++
		}
	}

	tst := make([]IOrderByTermContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IOrderByTermContext); ok {
			tst[i] = t.(IOrderByTermContext)
			i++
		}
	}

	return tst
}

func (s *OrderByContext) OrderByTerm(i int) IOrderByTermContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IOrderByTermContext); ok {
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

	return t.(IOrderByTermContext)
}

func (s *OrderByContext) RPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserRPAR, 0)
}

func (s *OrderByContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(SLQParserCOMMA)
}

func (s *OrderByContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserCOMMA, i)
}

func (s *OrderByContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *OrderByContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *OrderByContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterOrderBy(s)
	}
}

func (s *OrderByContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitOrderBy(s)
	}
}

func (s *OrderByContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitOrderBy(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) OrderBy() (localctx IOrderByContext) {
	this := p
	_ = this

	localctx = NewOrderByContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 30, SLQParserRULE_orderBy)
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
		p.SetState(173)
		p.Match(SLQParserORDER_BY)
	}
	{
		p.SetState(174)
		p.Match(SLQParserLPAR)
	}
	{
		p.SetState(175)
		p.OrderByTerm()
	}
	p.SetState(180)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserCOMMA {
		{
			p.SetState(176)
			p.Match(SLQParserCOMMA)
		}
		{
			p.SetState(177)
			p.OrderByTerm()
		}

		p.SetState(182)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(183)
		p.Match(SLQParserRPAR)
	}

	return localctx
}

// ISelectorContext is an interface to support dynamic dispatch.
type ISelectorContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	AllNAME() []antlr.TerminalNode
	NAME(i int) antlr.TerminalNode

	// IsSelectorContext differentiates from other interfaces.
	IsSelectorContext()
}

type SelectorContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySelectorContext() *SelectorContext {
	var p = new(SelectorContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_selector
	return p
}

func (*SelectorContext) IsSelectorContext() {}

func NewSelectorContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SelectorContext {
	var p = new(SelectorContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_selector

	return p
}

func (s *SelectorContext) GetParser() antlr.Parser { return s.parser }

func (s *SelectorContext) AllNAME() []antlr.TerminalNode {
	return s.GetTokens(SLQParserNAME)
}

func (s *SelectorContext) NAME(i int) antlr.TerminalNode {
	return s.GetToken(SLQParserNAME, i)
}

func (s *SelectorContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SelectorContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SelectorContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterSelector(s)
	}
}

func (s *SelectorContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitSelector(s)
	}
}

func (s *SelectorContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitSelector(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Selector() (localctx ISelectorContext) {
	this := p
	_ = this

	localctx = NewSelectorContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 32, SLQParserRULE_selector)

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
		p.SetState(185)
		p.Match(SLQParserNAME)
	}
	p.SetState(187)
	p.GetErrorHandler().Sync(p)

	if p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 18, p.GetParserRuleContext()) == 1 {
		{
			p.SetState(186)
			p.Match(SLQParserNAME)
		}

	}

	return localctx
}

// ISelectorElementContext is an interface to support dynamic dispatch.
type ISelectorElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Selector() ISelectorContext
	Alias() IAliasContext

	// IsSelectorElementContext differentiates from other interfaces.
	IsSelectorElementContext()
}

type SelectorElementContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySelectorElementContext() *SelectorElementContext {
	var p = new(SelectorElementContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_selectorElement
	return p
}

func (*SelectorElementContext) IsSelectorElementContext() {}

func NewSelectorElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SelectorElementContext {
	var p = new(SelectorElementContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_selectorElement

	return p
}

func (s *SelectorElementContext) GetParser() antlr.Parser { return s.parser }

func (s *SelectorElementContext) Selector() ISelectorContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISelectorContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISelectorContext)
}

func (s *SelectorElementContext) Alias() IAliasContext {
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

func (s *SelectorElementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SelectorElementContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SelectorElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterSelectorElement(s)
	}
}

func (s *SelectorElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitSelectorElement(s)
	}
}

func (s *SelectorElementContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitSelectorElement(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) SelectorElement() (localctx ISelectorElementContext) {
	this := p
	_ = this

	localctx = NewSelectorElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 34, SLQParserRULE_selectorElement)
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
		p.SetState(189)
		p.Selector()
	}

	p.SetState(191)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserALIAS_RESERVED || _la == SLQParserCOLON {
		{
			p.SetState(190)
			p.Alias()
		}

	}

	return localctx
}

// IAliasContext is an interface to support dynamic dispatch.
type IAliasContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	ALIAS_RESERVED() antlr.TerminalNode
	COLON() antlr.TerminalNode
	ARG() antlr.TerminalNode
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

func (s *AliasContext) ALIAS_RESERVED() antlr.TerminalNode {
	return s.GetToken(SLQParserALIAS_RESERVED, 0)
}

func (s *AliasContext) COLON() antlr.TerminalNode {
	return s.GetToken(SLQParserCOLON, 0)
}

func (s *AliasContext) ARG() antlr.TerminalNode {
	return s.GetToken(SLQParserARG, 0)
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
	p.EnterRule(localctx, 36, SLQParserRULE_alias)
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

	p.SetState(196)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case SLQParserALIAS_RESERVED:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(193)
			p.Match(SLQParserALIAS_RESERVED)
		}

	case SLQParserCOLON:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(194)
			p.Match(SLQParserCOLON)
		}
		{
			p.SetState(195)
			_la = p.GetTokenStream().LA(1)

			if !(_la == SLQParserARG || _la == SLQParserID) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}

	default:
		panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
	}

	return localctx
}

// IArgContext is an interface to support dynamic dispatch.
type IArgContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	ARG() antlr.TerminalNode

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

func (s *ArgContext) ARG() antlr.TerminalNode {
	return s.GetToken(SLQParserARG, 0)
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
	this := p
	_ = this

	localctx = NewArgContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 38, SLQParserRULE_arg)

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
		p.SetState(198)
		p.Match(SLQParserARG)
	}

	return localctx
}

// IHandleTableContext is an interface to support dynamic dispatch.
type IHandleTableContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	HANDLE() antlr.TerminalNode
	NAME() antlr.TerminalNode

	// IsHandleTableContext differentiates from other interfaces.
	IsHandleTableContext()
}

type HandleTableContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyHandleTableContext() *HandleTableContext {
	var p = new(HandleTableContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_handleTable
	return p
}

func (*HandleTableContext) IsHandleTableContext() {}

func NewHandleTableContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *HandleTableContext {
	var p = new(HandleTableContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_handleTable

	return p
}

func (s *HandleTableContext) GetParser() antlr.Parser { return s.parser }

func (s *HandleTableContext) HANDLE() antlr.TerminalNode {
	return s.GetToken(SLQParserHANDLE, 0)
}

func (s *HandleTableContext) NAME() antlr.TerminalNode {
	return s.GetToken(SLQParserNAME, 0)
}

func (s *HandleTableContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *HandleTableContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *HandleTableContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterHandleTable(s)
	}
}

func (s *HandleTableContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitHandleTable(s)
	}
}

func (s *HandleTableContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitHandleTable(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) HandleTable() (localctx IHandleTableContext) {
	this := p
	_ = this

	localctx = NewHandleTableContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 40, SLQParserRULE_handleTable)

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
		p.SetState(200)
		p.Match(SLQParserHANDLE)
	}
	{
		p.SetState(201)
		p.Match(SLQParserNAME)
	}

	return localctx
}

// IHandleContext is an interface to support dynamic dispatch.
type IHandleContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	HANDLE() antlr.TerminalNode

	// IsHandleContext differentiates from other interfaces.
	IsHandleContext()
}

type HandleContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyHandleContext() *HandleContext {
	var p = new(HandleContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = SLQParserRULE_handle
	return p
}

func (*HandleContext) IsHandleContext() {}

func NewHandleContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *HandleContext {
	var p = new(HandleContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_handle

	return p
}

func (s *HandleContext) GetParser() antlr.Parser { return s.parser }

func (s *HandleContext) HANDLE() antlr.TerminalNode {
	return s.GetToken(SLQParserHANDLE, 0)
}

func (s *HandleContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *HandleContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *HandleContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterHandle(s)
	}
}

func (s *HandleContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitHandle(s)
	}
}

func (s *HandleContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitHandle(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Handle() (localctx IHandleContext) {
	this := p
	_ = this

	localctx = NewHandleContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 42, SLQParserRULE_handle)

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
		p.SetState(203)
		p.Match(SLQParserHANDLE)
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
	p.EnterRule(localctx, 44, SLQParserRULE_rowRange)

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
		p.SetState(205)
		p.Match(SLQParserT__10)
	}
	p.SetState(214)
	p.GetErrorHandler().Sync(p)

	if p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 21, p.GetParserRuleContext()) == 1 {
		{
			p.SetState(206)
			p.Match(SLQParserNN)
		}
		{
			p.SetState(207)
			p.Match(SLQParserCOLON)
		}
		{
			p.SetState(208)
			p.Match(SLQParserNN)
		}

	} else if p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 21, p.GetParserRuleContext()) == 2 {
		{
			p.SetState(209)
			p.Match(SLQParserNN)
		}
		{
			p.SetState(210)
			p.Match(SLQParserCOLON)
		}

	} else if p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 21, p.GetParserRuleContext()) == 3 {
		{
			p.SetState(211)
			p.Match(SLQParserCOLON)
		}
		{
			p.SetState(212)
			p.Match(SLQParserNN)
		}

	} else if p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 21, p.GetParserRuleContext()) == 4 {
		{
			p.SetState(213)
			p.Match(SLQParserNN)
		}

	}
	{
		p.SetState(216)
		p.Match(SLQParserRBRA)
	}

	return localctx
}

// IExprContext is an interface to support dynamic dispatch.
type IExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Selector() ISelectorContext
	Literal() ILiteralContext
	Arg() IArgContext
	UnaryOperator() IUnaryOperatorContext
	AllExpr() []IExprContext
	Expr(i int) IExprContext
	Func_() IFuncContext
	ORDER_ASC() antlr.TerminalNode
	ORDER_DESC() antlr.TerminalNode
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

func (s *ExprContext) Selector() ISelectorContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISelectorContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISelectorContext)
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

func (s *ExprContext) Arg() IArgContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IArgContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IArgContext)
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

func (s *ExprContext) Func_() IFuncContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFuncContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFuncContext)
}

func (s *ExprContext) ORDER_ASC() antlr.TerminalNode {
	return s.GetToken(SLQParserORDER_ASC, 0)
}

func (s *ExprContext) ORDER_DESC() antlr.TerminalNode {
	return s.GetToken(SLQParserORDER_DESC, 0)
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
	_startState := 46
	p.EnterRecursionRule(localctx, 46, SLQParserRULE_expr, _p)
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
	p.SetState(226)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case SLQParserNAME:
		{
			p.SetState(219)
			p.Selector()
		}

	case SLQParserNULL, SLQParserNN, SLQParserNUMBER, SLQParserSTRING:
		{
			p.SetState(220)
			p.Literal()
		}

	case SLQParserARG:
		{
			p.SetState(221)
			p.Arg()
		}

	case SLQParserT__18, SLQParserT__19, SLQParserORDER_ASC, SLQParserORDER_DESC:
		{
			p.SetState(222)
			p.UnaryOperator()
		}
		{
			p.SetState(223)
			p.expr(9)
		}

	case SLQParserT__2, SLQParserT__3, SLQParserT__4, SLQParserT__5, SLQParserT__6, SLQParserPROPRIETARY_FUNC_NAME:
		{
			p.SetState(225)
			p.Func_()
		}

	default:
		panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
	}
	p.GetParserRuleContext().SetStop(p.GetTokenStream().LT(-1))
	p.SetState(255)
	p.GetErrorHandler().Sync(p)
	_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 25, p.GetParserRuleContext())

	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			if p.GetParseListeners() != nil {
				p.TriggerExitRuleEvent()
			}
			_prevctx = localctx
			p.SetState(253)
			p.GetErrorHandler().Sync(p)
			switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 24, p.GetParserRuleContext()) {
			case 1:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(228)

				if !(p.Precpred(p.GetParserRuleContext(), 8)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 8)", ""))
				}
				{
					p.SetState(229)
					p.Match(SLQParserT__11)
				}
				{
					p.SetState(230)
					p.expr(9)
				}

			case 2:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(231)

				if !(p.Precpred(p.GetParserRuleContext(), 7)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 7)", ""))
				}
				{
					p.SetState(232)
					_la = p.GetTokenStream().LA(1)

					if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&24580) != 0) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(233)
					p.expr(8)
				}

			case 3:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(234)

				if !(p.Precpred(p.GetParserRuleContext(), 6)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 6)", ""))
				}
				{
					p.SetState(235)
					_la = p.GetTokenStream().LA(1)

					if !(_la == SLQParserORDER_ASC || _la == SLQParserORDER_DESC) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(236)
					p.expr(7)
				}

			case 4:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(237)

				if !(p.Precpred(p.GetParserRuleContext(), 5)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 5)", ""))
				}
				{
					p.SetState(238)
					_la = p.GetTokenStream().LA(1)

					if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&229376) != 0) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(239)
					p.expr(6)
				}

			case 5:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(240)

				if !(p.Precpred(p.GetParserRuleContext(), 4)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 4)", ""))
				}
				{
					p.SetState(241)
					_la = p.GetTokenStream().LA(1)

					if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&16492674416640) != 0) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(242)
					p.expr(5)
				}

			case 6:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(243)

				if !(p.Precpred(p.GetParserRuleContext(), 3)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 3)", ""))
				}
				p.SetState(247)
				p.GetErrorHandler().Sync(p)

				switch p.GetTokenStream().LA(1) {
				case SLQParserEQ:
					{
						p.SetState(244)
						p.Match(SLQParserEQ)
					}

				case SLQParserNEQ:
					{
						p.SetState(245)
						p.Match(SLQParserNEQ)
					}

				case SLQParserT__2, SLQParserT__3, SLQParserT__4, SLQParserT__5, SLQParserT__6, SLQParserT__18, SLQParserT__19, SLQParserPROPRIETARY_FUNC_NAME, SLQParserORDER_ASC, SLQParserORDER_DESC, SLQParserARG, SLQParserNULL, SLQParserNN, SLQParserNUMBER, SLQParserNAME, SLQParserSTRING:

				default:
					panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
				}
				{
					p.SetState(249)
					p.expr(4)
				}

			case 7:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(250)

				if !(p.Precpred(p.GetParserRuleContext(), 2)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 2)", ""))
				}
				{
					p.SetState(251)
					p.Match(SLQParserT__17)
				}
				{
					p.SetState(252)
					p.expr(3)
				}

			}

		}
		p.SetState(257)
		p.GetErrorHandler().Sync(p)
		_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 25, p.GetParserRuleContext())
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
	p.EnterRule(localctx, 48, SLQParserRULE_literal)
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
		p.SetState(258)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&282299878866944) != 0) {
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

	// Getter signatures
	ORDER_DESC() antlr.TerminalNode
	ORDER_ASC() antlr.TerminalNode

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

func (s *UnaryOperatorContext) ORDER_DESC() antlr.TerminalNode {
	return s.GetToken(SLQParserORDER_DESC, 0)
}

func (s *UnaryOperatorContext) ORDER_ASC() antlr.TerminalNode {
	return s.GetToken(SLQParserORDER_ASC, 0)
}

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
	p.EnterRule(localctx, 50, SLQParserRULE_unaryOperator)
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
		p.SetState(260)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&26738688) != 0) {
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
	case 23:
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

// Code generated from SLQ.g4 by ANTLR 4.13.0. DO NOT EDIT.

package slq // SLQ
import (
	"fmt"
	"strconv"
	"sync"

	"github.com/antlr4-go/antlr/v4"
)

// Suppress unused import errors
var _ = fmt.Printf
var _ = strconv.Itoa
var _ = sync.Once{}

type SLQParser struct {
	*antlr.BaseParser
}

var SLQParserStaticData struct {
	PredictionContextCache *antlr.PredictionContextCache
	atn                    *antlr.ATN
	serializedATN          []int32
	LiteralNames           []string
	SymbolicNames          []string
	RuleNames              []string
	decisionToDFA          []*antlr.DFA
	once                   sync.Once
}

func slqParserInit() {
	staticData := &SLQParserStaticData
	staticData.LiteralNames = []string{
		"", "';'", "'*'", "'sum'", "'avg'", "'max'", "'min'", "'schema'", "'catalog'",
		"'rownum'", "'unique'", "'uniq'", "'count'", "'+'", "'-'", "'.['", "'||'",
		"'/'", "'%'", "'<<'", "'>>'", "'&'", "'&&'", "'~'", "'!'", "", "", "",
		"", "'having'", "", "", "", "'null'", "", "", "'('", "')'", "'['", "']'",
		"','", "'|'", "':'", "", "", "'<='", "'<'", "'>='", "'>'", "'!='", "'=='",
	}
	staticData.SymbolicNames = []string{
		"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
		"", "", "", "", "", "", "", "", "PROPRIETARY_FUNC_NAME", "JOIN_TYPE",
		"WHERE", "GROUP_BY", "HAVING", "ORDER_BY", "ALIAS_RESERVED", "ARG",
		"NULL", "ID", "WS", "LPAR", "RPAR", "LBRA", "RBRA", "COMMA", "PIPE",
		"COLON", "NN", "NUMBER", "LT_EQ", "LT", "GT_EQ", "GT", "NEQ", "EQ",
		"NAME", "HANDLE", "STRING", "LINECOMMENT",
	}
	staticData.RuleNames = []string{
		"stmtList", "query", "segment", "element", "funcElement", "func", "funcName",
		"join", "joinTable", "uniqueFunc", "countFunc", "where", "groupByTerm",
		"groupBy", "having", "orderByTerm", "orderBy", "selector", "selectorElement",
		"alias", "arg", "handleTable", "handle", "rowRange", "exprElement",
		"expr", "literal", "unaryOperator",
	}
	staticData.PredictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 1, 54, 291, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2, 4, 7,
		4, 2, 5, 7, 5, 2, 6, 7, 6, 2, 7, 7, 7, 2, 8, 7, 8, 2, 9, 7, 9, 2, 10, 7,
		10, 2, 11, 7, 11, 2, 12, 7, 12, 2, 13, 7, 13, 2, 14, 7, 14, 2, 15, 7, 15,
		2, 16, 7, 16, 2, 17, 7, 17, 2, 18, 7, 18, 2, 19, 7, 19, 2, 20, 7, 20, 2,
		21, 7, 21, 2, 22, 7, 22, 2, 23, 7, 23, 2, 24, 7, 24, 2, 25, 7, 25, 2, 26,
		7, 26, 2, 27, 7, 27, 1, 0, 5, 0, 58, 8, 0, 10, 0, 12, 0, 61, 9, 0, 1, 0,
		1, 0, 4, 0, 65, 8, 0, 11, 0, 12, 0, 66, 1, 0, 5, 0, 70, 8, 0, 10, 0, 12,
		0, 73, 9, 0, 1, 0, 5, 0, 76, 8, 0, 10, 0, 12, 0, 79, 9, 0, 1, 1, 1, 1,
		1, 1, 5, 1, 84, 8, 1, 10, 1, 12, 1, 87, 9, 1, 1, 2, 1, 2, 1, 2, 5, 2, 92,
		8, 2, 10, 2, 12, 2, 95, 9, 2, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3,
		1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 3, 3, 110, 8, 3, 1, 4, 1, 4, 3, 4,
		114, 8, 4, 1, 5, 1, 5, 1, 5, 1, 5, 1, 5, 5, 5, 121, 8, 5, 10, 5, 12, 5,
		124, 9, 5, 1, 5, 3, 5, 127, 8, 5, 1, 5, 1, 5, 1, 6, 1, 6, 1, 7, 1, 7, 1,
		7, 1, 7, 1, 7, 3, 7, 138, 8, 7, 1, 7, 1, 7, 1, 8, 3, 8, 143, 8, 8, 1, 8,
		1, 8, 3, 8, 147, 8, 8, 1, 9, 1, 9, 1, 10, 1, 10, 1, 10, 3, 10, 154, 8,
		10, 1, 10, 3, 10, 157, 8, 10, 1, 10, 3, 10, 160, 8, 10, 1, 11, 1, 11, 1,
		11, 3, 11, 165, 8, 11, 1, 11, 1, 11, 1, 12, 1, 12, 3, 12, 171, 8, 12, 1,
		13, 1, 13, 1, 13, 1, 13, 1, 13, 5, 13, 178, 8, 13, 10, 13, 12, 13, 181,
		9, 13, 1, 13, 1, 13, 1, 14, 1, 14, 1, 14, 1, 14, 1, 14, 1, 15, 1, 15, 3,
		15, 192, 8, 15, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 5, 16, 199, 8, 16, 10,
		16, 12, 16, 202, 9, 16, 1, 16, 1, 16, 1, 17, 1, 17, 3, 17, 208, 8, 17,
		1, 18, 1, 18, 3, 18, 212, 8, 18, 1, 19, 1, 19, 1, 19, 3, 19, 217, 8, 19,
		1, 20, 1, 20, 1, 21, 1, 21, 1, 21, 1, 22, 1, 22, 1, 23, 1, 23, 1, 23, 1,
		23, 1, 23, 1, 23, 1, 23, 1, 23, 1, 23, 3, 23, 235, 8, 23, 1, 23, 1, 23,
		1, 24, 1, 24, 3, 24, 241, 8, 24, 1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 1,
		25, 1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 3, 25, 255, 8, 25, 1, 25,
		1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 1,
		25, 1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 3, 25, 276, 8, 25,
		1, 25, 1, 25, 1, 25, 1, 25, 5, 25, 282, 8, 25, 10, 25, 12, 25, 285, 9,
		25, 1, 26, 1, 26, 1, 27, 1, 27, 1, 27, 0, 1, 50, 28, 0, 2, 4, 6, 8, 10,
		12, 14, 16, 18, 20, 22, 24, 26, 28, 30, 32, 34, 36, 38, 40, 42, 44, 46,
		48, 50, 52, 54, 0, 9, 2, 0, 3, 9, 25, 25, 1, 0, 10, 11, 1, 0, 13, 14, 3,
		0, 32, 32, 34, 34, 53, 53, 2, 0, 2, 2, 17, 18, 1, 0, 19, 21, 1, 0, 45,
		48, 3, 0, 33, 33, 43, 44, 53, 53, 2, 0, 13, 14, 23, 24, 317, 0, 59, 1,
		0, 0, 0, 2, 80, 1, 0, 0, 0, 4, 88, 1, 0, 0, 0, 6, 109, 1, 0, 0, 0, 8, 111,
		1, 0, 0, 0, 10, 115, 1, 0, 0, 0, 12, 130, 1, 0, 0, 0, 14, 132, 1, 0, 0,
		0, 16, 142, 1, 0, 0, 0, 18, 148, 1, 0, 0, 0, 20, 150, 1, 0, 0, 0, 22, 161,
		1, 0, 0, 0, 24, 170, 1, 0, 0, 0, 26, 172, 1, 0, 0, 0, 28, 184, 1, 0, 0,
		0, 30, 189, 1, 0, 0, 0, 32, 193, 1, 0, 0, 0, 34, 205, 1, 0, 0, 0, 36, 209,
		1, 0, 0, 0, 38, 216, 1, 0, 0, 0, 40, 218, 1, 0, 0, 0, 42, 220, 1, 0, 0,
		0, 44, 223, 1, 0, 0, 0, 46, 225, 1, 0, 0, 0, 48, 238, 1, 0, 0, 0, 50, 254,
		1, 0, 0, 0, 52, 286, 1, 0, 0, 0, 54, 288, 1, 0, 0, 0, 56, 58, 5, 1, 0,
		0, 57, 56, 1, 0, 0, 0, 58, 61, 1, 0, 0, 0, 59, 57, 1, 0, 0, 0, 59, 60,
		1, 0, 0, 0, 60, 62, 1, 0, 0, 0, 61, 59, 1, 0, 0, 0, 62, 71, 3, 2, 1, 0,
		63, 65, 5, 1, 0, 0, 64, 63, 1, 0, 0, 0, 65, 66, 1, 0, 0, 0, 66, 64, 1,
		0, 0, 0, 66, 67, 1, 0, 0, 0, 67, 68, 1, 0, 0, 0, 68, 70, 3, 2, 1, 0, 69,
		64, 1, 0, 0, 0, 70, 73, 1, 0, 0, 0, 71, 69, 1, 0, 0, 0, 71, 72, 1, 0, 0,
		0, 72, 77, 1, 0, 0, 0, 73, 71, 1, 0, 0, 0, 74, 76, 5, 1, 0, 0, 75, 74,
		1, 0, 0, 0, 76, 79, 1, 0, 0, 0, 77, 75, 1, 0, 0, 0, 77, 78, 1, 0, 0, 0,
		78, 1, 1, 0, 0, 0, 79, 77, 1, 0, 0, 0, 80, 85, 3, 4, 2, 0, 81, 82, 5, 41,
		0, 0, 82, 84, 3, 4, 2, 0, 83, 81, 1, 0, 0, 0, 84, 87, 1, 0, 0, 0, 85, 83,
		1, 0, 0, 0, 85, 86, 1, 0, 0, 0, 86, 3, 1, 0, 0, 0, 87, 85, 1, 0, 0, 0,
		88, 93, 3, 6, 3, 0, 89, 90, 5, 40, 0, 0, 90, 92, 3, 6, 3, 0, 91, 89, 1,
		0, 0, 0, 92, 95, 1, 0, 0, 0, 93, 91, 1, 0, 0, 0, 93, 94, 1, 0, 0, 0, 94,
		5, 1, 0, 0, 0, 95, 93, 1, 0, 0, 0, 96, 110, 3, 42, 21, 0, 97, 110, 3, 44,
		22, 0, 98, 110, 3, 36, 18, 0, 99, 110, 3, 14, 7, 0, 100, 110, 3, 26, 13,
		0, 101, 110, 3, 28, 14, 0, 102, 110, 3, 32, 16, 0, 103, 110, 3, 46, 23,
		0, 104, 110, 3, 18, 9, 0, 105, 110, 3, 20, 10, 0, 106, 110, 3, 22, 11,
		0, 107, 110, 3, 8, 4, 0, 108, 110, 3, 48, 24, 0, 109, 96, 1, 0, 0, 0, 109,
		97, 1, 0, 0, 0, 109, 98, 1, 0, 0, 0, 109, 99, 1, 0, 0, 0, 109, 100, 1,
		0, 0, 0, 109, 101, 1, 0, 0, 0, 109, 102, 1, 0, 0, 0, 109, 103, 1, 0, 0,
		0, 109, 104, 1, 0, 0, 0, 109, 105, 1, 0, 0, 0, 109, 106, 1, 0, 0, 0, 109,
		107, 1, 0, 0, 0, 109, 108, 1, 0, 0, 0, 110, 7, 1, 0, 0, 0, 111, 113, 3,
		10, 5, 0, 112, 114, 3, 38, 19, 0, 113, 112, 1, 0, 0, 0, 113, 114, 1, 0,
		0, 0, 114, 9, 1, 0, 0, 0, 115, 116, 3, 12, 6, 0, 116, 126, 5, 36, 0, 0,
		117, 122, 3, 50, 25, 0, 118, 119, 5, 40, 0, 0, 119, 121, 3, 50, 25, 0,
		120, 118, 1, 0, 0, 0, 121, 124, 1, 0, 0, 0, 122, 120, 1, 0, 0, 0, 122,
		123, 1, 0, 0, 0, 123, 127, 1, 0, 0, 0, 124, 122, 1, 0, 0, 0, 125, 127,
		5, 2, 0, 0, 126, 117, 1, 0, 0, 0, 126, 125, 1, 0, 0, 0, 126, 127, 1, 0,
		0, 0, 127, 128, 1, 0, 0, 0, 128, 129, 5, 37, 0, 0, 129, 11, 1, 0, 0, 0,
		130, 131, 7, 0, 0, 0, 131, 13, 1, 0, 0, 0, 132, 133, 5, 26, 0, 0, 133,
		134, 5, 36, 0, 0, 134, 137, 3, 16, 8, 0, 135, 136, 5, 40, 0, 0, 136, 138,
		3, 50, 25, 0, 137, 135, 1, 0, 0, 0, 137, 138, 1, 0, 0, 0, 138, 139, 1,
		0, 0, 0, 139, 140, 5, 37, 0, 0, 140, 15, 1, 0, 0, 0, 141, 143, 5, 52, 0,
		0, 142, 141, 1, 0, 0, 0, 142, 143, 1, 0, 0, 0, 143, 144, 1, 0, 0, 0, 144,
		146, 5, 51, 0, 0, 145, 147, 3, 38, 19, 0, 146, 145, 1, 0, 0, 0, 146, 147,
		1, 0, 0, 0, 147, 17, 1, 0, 0, 0, 148, 149, 7, 1, 0, 0, 149, 19, 1, 0, 0,
		0, 150, 156, 5, 12, 0, 0, 151, 153, 5, 36, 0, 0, 152, 154, 3, 34, 17, 0,
		153, 152, 1, 0, 0, 0, 153, 154, 1, 0, 0, 0, 154, 155, 1, 0, 0, 0, 155,
		157, 5, 37, 0, 0, 156, 151, 1, 0, 0, 0, 156, 157, 1, 0, 0, 0, 157, 159,
		1, 0, 0, 0, 158, 160, 3, 38, 19, 0, 159, 158, 1, 0, 0, 0, 159, 160, 1,
		0, 0, 0, 160, 21, 1, 0, 0, 0, 161, 162, 5, 27, 0, 0, 162, 164, 5, 36, 0,
		0, 163, 165, 3, 50, 25, 0, 164, 163, 1, 0, 0, 0, 164, 165, 1, 0, 0, 0,
		165, 166, 1, 0, 0, 0, 166, 167, 5, 37, 0, 0, 167, 23, 1, 0, 0, 0, 168,
		171, 3, 34, 17, 0, 169, 171, 3, 10, 5, 0, 170, 168, 1, 0, 0, 0, 170, 169,
		1, 0, 0, 0, 171, 25, 1, 0, 0, 0, 172, 173, 5, 28, 0, 0, 173, 174, 5, 36,
		0, 0, 174, 179, 3, 24, 12, 0, 175, 176, 5, 40, 0, 0, 176, 178, 3, 24, 12,
		0, 177, 175, 1, 0, 0, 0, 178, 181, 1, 0, 0, 0, 179, 177, 1, 0, 0, 0, 179,
		180, 1, 0, 0, 0, 180, 182, 1, 0, 0, 0, 181, 179, 1, 0, 0, 0, 182, 183,
		5, 37, 0, 0, 183, 27, 1, 0, 0, 0, 184, 185, 5, 29, 0, 0, 185, 186, 5, 36,
		0, 0, 186, 187, 3, 50, 25, 0, 187, 188, 5, 37, 0, 0, 188, 29, 1, 0, 0,
		0, 189, 191, 3, 34, 17, 0, 190, 192, 7, 2, 0, 0, 191, 190, 1, 0, 0, 0,
		191, 192, 1, 0, 0, 0, 192, 31, 1, 0, 0, 0, 193, 194, 5, 30, 0, 0, 194,
		195, 5, 36, 0, 0, 195, 200, 3, 30, 15, 0, 196, 197, 5, 40, 0, 0, 197, 199,
		3, 30, 15, 0, 198, 196, 1, 0, 0, 0, 199, 202, 1, 0, 0, 0, 200, 198, 1,
		0, 0, 0, 200, 201, 1, 0, 0, 0, 201, 203, 1, 0, 0, 0, 202, 200, 1, 0, 0,
		0, 203, 204, 5, 37, 0, 0, 204, 33, 1, 0, 0, 0, 205, 207, 5, 51, 0, 0, 206,
		208, 5, 51, 0, 0, 207, 206, 1, 0, 0, 0, 207, 208, 1, 0, 0, 0, 208, 35,
		1, 0, 0, 0, 209, 211, 3, 34, 17, 0, 210, 212, 3, 38, 19, 0, 211, 210, 1,
		0, 0, 0, 211, 212, 1, 0, 0, 0, 212, 37, 1, 0, 0, 0, 213, 217, 5, 31, 0,
		0, 214, 215, 5, 42, 0, 0, 215, 217, 7, 3, 0, 0, 216, 213, 1, 0, 0, 0, 216,
		214, 1, 0, 0, 0, 217, 39, 1, 0, 0, 0, 218, 219, 5, 32, 0, 0, 219, 41, 1,
		0, 0, 0, 220, 221, 5, 52, 0, 0, 221, 222, 5, 51, 0, 0, 222, 43, 1, 0, 0,
		0, 223, 224, 5, 52, 0, 0, 224, 45, 1, 0, 0, 0, 225, 234, 5, 15, 0, 0, 226,
		227, 5, 43, 0, 0, 227, 228, 5, 42, 0, 0, 228, 235, 5, 43, 0, 0, 229, 230,
		5, 43, 0, 0, 230, 235, 5, 42, 0, 0, 231, 232, 5, 42, 0, 0, 232, 235, 5,
		43, 0, 0, 233, 235, 5, 43, 0, 0, 234, 226, 1, 0, 0, 0, 234, 229, 1, 0,
		0, 0, 234, 231, 1, 0, 0, 0, 234, 233, 1, 0, 0, 0, 234, 235, 1, 0, 0, 0,
		235, 236, 1, 0, 0, 0, 236, 237, 5, 39, 0, 0, 237, 47, 1, 0, 0, 0, 238,
		240, 3, 50, 25, 0, 239, 241, 3, 38, 19, 0, 240, 239, 1, 0, 0, 0, 240, 241,
		1, 0, 0, 0, 241, 49, 1, 0, 0, 0, 242, 243, 6, 25, -1, 0, 243, 244, 5, 36,
		0, 0, 244, 245, 3, 50, 25, 0, 245, 246, 5, 37, 0, 0, 246, 255, 1, 0, 0,
		0, 247, 255, 3, 34, 17, 0, 248, 255, 3, 52, 26, 0, 249, 255, 3, 40, 20,
		0, 250, 251, 3, 54, 27, 0, 251, 252, 3, 50, 25, 9, 252, 255, 1, 0, 0, 0,
		253, 255, 3, 10, 5, 0, 254, 242, 1, 0, 0, 0, 254, 247, 1, 0, 0, 0, 254,
		248, 1, 0, 0, 0, 254, 249, 1, 0, 0, 0, 254, 250, 1, 0, 0, 0, 254, 253,
		1, 0, 0, 0, 255, 283, 1, 0, 0, 0, 256, 257, 10, 8, 0, 0, 257, 258, 5, 16,
		0, 0, 258, 282, 3, 50, 25, 9, 259, 260, 10, 7, 0, 0, 260, 261, 7, 4, 0,
		0, 261, 282, 3, 50, 25, 8, 262, 263, 10, 6, 0, 0, 263, 264, 7, 2, 0, 0,
		264, 282, 3, 50, 25, 7, 265, 266, 10, 5, 0, 0, 266, 267, 7, 5, 0, 0, 267,
		282, 3, 50, 25, 6, 268, 269, 10, 4, 0, 0, 269, 270, 7, 6, 0, 0, 270, 282,
		3, 50, 25, 5, 271, 275, 10, 3, 0, 0, 272, 276, 5, 50, 0, 0, 273, 276, 5,
		49, 0, 0, 274, 276, 1, 0, 0, 0, 275, 272, 1, 0, 0, 0, 275, 273, 1, 0, 0,
		0, 275, 274, 1, 0, 0, 0, 276, 277, 1, 0, 0, 0, 277, 282, 3, 50, 25, 4,
		278, 279, 10, 2, 0, 0, 279, 280, 5, 22, 0, 0, 280, 282, 3, 50, 25, 3, 281,
		256, 1, 0, 0, 0, 281, 259, 1, 0, 0, 0, 281, 262, 1, 0, 0, 0, 281, 265,
		1, 0, 0, 0, 281, 268, 1, 0, 0, 0, 281, 271, 1, 0, 0, 0, 281, 278, 1, 0,
		0, 0, 282, 285, 1, 0, 0, 0, 283, 281, 1, 0, 0, 0, 283, 284, 1, 0, 0, 0,
		284, 51, 1, 0, 0, 0, 285, 283, 1, 0, 0, 0, 286, 287, 7, 7, 0, 0, 287, 53,
		1, 0, 0, 0, 288, 289, 7, 8, 0, 0, 289, 55, 1, 0, 0, 0, 30, 59, 66, 71,
		77, 85, 93, 109, 113, 122, 126, 137, 142, 146, 153, 156, 159, 164, 170,
		179, 191, 200, 207, 211, 216, 234, 240, 254, 275, 281, 283,
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
	staticData := &SLQParserStaticData
	staticData.once.Do(slqParserInit)
}

// NewSLQParser produces a new parser instance for the optional input antlr.TokenStream.
func NewSLQParser(input antlr.TokenStream) *SLQParser {
	SLQParserInit()
	this := new(SLQParser)
	this.BaseParser = antlr.NewBaseParser(input)
	staticData := &SLQParserStaticData
	this.Interpreter = antlr.NewParserATNSimulator(this, staticData.atn, staticData.decisionToDFA, staticData.PredictionContextCache)
	this.RuleNames = staticData.RuleNames
	this.LiteralNames = staticData.LiteralNames
	this.SymbolicNames = staticData.SymbolicNames
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
	SLQParserT__20                 = 21
	SLQParserT__21                 = 22
	SLQParserT__22                 = 23
	SLQParserT__23                 = 24
	SLQParserPROPRIETARY_FUNC_NAME = 25
	SLQParserJOIN_TYPE             = 26
	SLQParserWHERE                 = 27
	SLQParserGROUP_BY              = 28
	SLQParserHAVING                = 29
	SLQParserORDER_BY              = 30
	SLQParserALIAS_RESERVED        = 31
	SLQParserARG                   = 32
	SLQParserNULL                  = 33
	SLQParserID                    = 34
	SLQParserWS                    = 35
	SLQParserLPAR                  = 36
	SLQParserRPAR                  = 37
	SLQParserLBRA                  = 38
	SLQParserRBRA                  = 39
	SLQParserCOMMA                 = 40
	SLQParserPIPE                  = 41
	SLQParserCOLON                 = 42
	SLQParserNN                    = 43
	SLQParserNUMBER                = 44
	SLQParserLT_EQ                 = 45
	SLQParserLT                    = 46
	SLQParserGT_EQ                 = 47
	SLQParserGT                    = 48
	SLQParserNEQ                   = 49
	SLQParserEQ                    = 50
	SLQParserNAME                  = 51
	SLQParserHANDLE                = 52
	SLQParserSTRING                = 53
	SLQParserLINECOMMENT           = 54
)

// SLQParser rules.
const (
	SLQParserRULE_stmtList        = 0
	SLQParserRULE_query           = 1
	SLQParserRULE_segment         = 2
	SLQParserRULE_element         = 3
	SLQParserRULE_funcElement     = 4
	SLQParserRULE_func            = 5
	SLQParserRULE_funcName        = 6
	SLQParserRULE_join            = 7
	SLQParserRULE_joinTable       = 8
	SLQParserRULE_uniqueFunc      = 9
	SLQParserRULE_countFunc       = 10
	SLQParserRULE_where           = 11
	SLQParserRULE_groupByTerm     = 12
	SLQParserRULE_groupBy         = 13
	SLQParserRULE_having          = 14
	SLQParserRULE_orderByTerm     = 15
	SLQParserRULE_orderBy         = 16
	SLQParserRULE_selector        = 17
	SLQParserRULE_selectorElement = 18
	SLQParserRULE_alias           = 19
	SLQParserRULE_arg             = 20
	SLQParserRULE_handleTable     = 21
	SLQParserRULE_handle          = 22
	SLQParserRULE_rowRange        = 23
	SLQParserRULE_exprElement     = 24
	SLQParserRULE_expr            = 25
	SLQParserRULE_literal         = 26
	SLQParserRULE_unaryOperator   = 27
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyStmtListContext() *StmtListContext {
	var p = new(StmtListContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_stmtList
	return p
}

func InitEmptyStmtListContext(p *StmtListContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_stmtList
}

func (*StmtListContext) IsStmtListContext() {}

func NewStmtListContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *StmtListContext {
	var p = new(StmtListContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewStmtListContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 0, SLQParserRULE_stmtList)
	var _la int

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(59)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserT__0 {
		{
			p.SetState(56)
			p.Match(SLQParserT__0)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

		p.SetState(61)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(62)
		p.Query()
	}
	p.SetState(71)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 2, p.GetParserRuleContext())
	if p.HasError() {
		goto errorExit
	}
	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			p.SetState(64)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)

			for ok := true; ok; ok = _la == SLQParserT__0 {
				{
					p.SetState(63)
					p.Match(SLQParserT__0)
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}

				p.SetState(66)
				p.GetErrorHandler().Sync(p)
				if p.HasError() {
					goto errorExit
				}
				_la = p.GetTokenStream().LA(1)
			}
			{
				p.SetState(68)
				p.Query()
			}

		}
		p.SetState(73)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 2, p.GetParserRuleContext())
		if p.HasError() {
			goto errorExit
		}
	}
	p.SetState(77)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserT__0 {
		{
			p.SetState(74)
			p.Match(SLQParserT__0)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

		p.SetState(79)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyQueryContext() *QueryContext {
	var p = new(QueryContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_query
	return p
}

func InitEmptyQueryContext(p *QueryContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_query
}

func (*QueryContext) IsQueryContext() {}

func NewQueryContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *QueryContext {
	var p = new(QueryContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewQueryContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 2, SLQParserRULE_query)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(80)
		p.Segment()
	}
	p.SetState(85)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserPIPE {
		{
			p.SetState(81)
			p.Match(SLQParserPIPE)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(82)
			p.Segment()
		}

		p.SetState(87)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptySegmentContext() *SegmentContext {
	var p = new(SegmentContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_segment
	return p
}

func InitEmptySegmentContext(p *SegmentContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_segment
}

func (*SegmentContext) IsSegmentContext() {}

func NewSegmentContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SegmentContext {
	var p = new(SegmentContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewSegmentContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 4, SLQParserRULE_segment)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(88)
		p.Element()
	}

	p.SetState(93)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserCOMMA {
		{
			p.SetState(89)
			p.Match(SLQParserCOMMA)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(90)
			p.Element()
		}

		p.SetState(95)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	Having() IHavingContext
	OrderBy() IOrderByContext
	RowRange() IRowRangeContext
	UniqueFunc() IUniqueFuncContext
	CountFunc() ICountFuncContext
	Where() IWhereContext
	FuncElement() IFuncElementContext
	ExprElement() IExprElementContext

	// IsElementContext differentiates from other interfaces.
	IsElementContext()
}

type ElementContext struct {
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyElementContext() *ElementContext {
	var p = new(ElementContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_element
	return p
}

func InitEmptyElementContext(p *ElementContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_element
}

func (*ElementContext) IsElementContext() {}

func NewElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ElementContext {
	var p = new(ElementContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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

func (s *ElementContext) Having() IHavingContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IHavingContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IHavingContext)
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

func (s *ElementContext) Where() IWhereContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IWhereContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IWhereContext)
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

func (s *ElementContext) ExprElement() IExprElementContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprElementContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprElementContext)
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
	p.EnterRule(localctx, 6, SLQParserRULE_element)
	p.SetState(109)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 6, p.GetParserRuleContext()) {
	case 1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(96)
			p.HandleTable()
		}

	case 2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(97)
			p.Handle()
		}

	case 3:
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(98)
			p.SelectorElement()
		}

	case 4:
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(99)
			p.Join()
		}

	case 5:
		p.EnterOuterAlt(localctx, 5)
		{
			p.SetState(100)
			p.GroupBy()
		}

	case 6:
		p.EnterOuterAlt(localctx, 6)
		{
			p.SetState(101)
			p.Having()
		}

	case 7:
		p.EnterOuterAlt(localctx, 7)
		{
			p.SetState(102)
			p.OrderBy()
		}

	case 8:
		p.EnterOuterAlt(localctx, 8)
		{
			p.SetState(103)
			p.RowRange()
		}

	case 9:
		p.EnterOuterAlt(localctx, 9)
		{
			p.SetState(104)
			p.UniqueFunc()
		}

	case 10:
		p.EnterOuterAlt(localctx, 10)
		{
			p.SetState(105)
			p.CountFunc()
		}

	case 11:
		p.EnterOuterAlt(localctx, 11)
		{
			p.SetState(106)
			p.Where()
		}

	case 12:
		p.EnterOuterAlt(localctx, 12)
		{
			p.SetState(107)
			p.FuncElement()
		}

	case 13:
		p.EnterOuterAlt(localctx, 13)
		{
			p.SetState(108)
			p.ExprElement()
		}

	case antlr.ATNInvalidAltNumber:
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyFuncElementContext() *FuncElementContext {
	var p = new(FuncElementContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_funcElement
	return p
}

func InitEmptyFuncElementContext(p *FuncElementContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_funcElement
}

func (*FuncElementContext) IsFuncElementContext() {}

func NewFuncElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FuncElementContext {
	var p = new(FuncElementContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewFuncElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 8, SLQParserRULE_funcElement)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(111)
		p.Func_()
	}
	p.SetState(113)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserALIAS_RESERVED || _la == SLQParserCOLON {
		{
			p.SetState(112)
			p.Alias()
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyFuncContext() *FuncContext {
	var p = new(FuncContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_func
	return p
}

func InitEmptyFuncContext(p *FuncContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_func
}

func (*FuncContext) IsFuncContext() {}

func NewFuncContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FuncContext {
	var p = new(FuncContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewFuncContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 10, SLQParserRULE_func)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(115)
		p.FuncName()
	}
	{
		p.SetState(116)
		p.Match(SLQParserLPAR)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(126)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	switch p.GetTokenStream().LA(1) {
	case SLQParserT__2, SLQParserT__3, SLQParserT__4, SLQParserT__5, SLQParserT__6, SLQParserT__7, SLQParserT__8, SLQParserT__12, SLQParserT__13, SLQParserT__22, SLQParserT__23, SLQParserPROPRIETARY_FUNC_NAME, SLQParserARG, SLQParserNULL, SLQParserLPAR, SLQParserNN, SLQParserNUMBER, SLQParserNAME, SLQParserSTRING:
		{
			p.SetState(117)
			p.expr(0)
		}
		p.SetState(122)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for _la == SLQParserCOMMA {
			{
				p.SetState(118)
				p.Match(SLQParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(119)
				p.expr(0)
			}

			p.SetState(124)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}

	case SLQParserT__1:
		{
			p.SetState(125)
			p.Match(SLQParserT__1)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case SLQParserRPAR:

	default:
	}
	{
		p.SetState(128)
		p.Match(SLQParserRPAR)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyFuncNameContext() *FuncNameContext {
	var p = new(FuncNameContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_funcName
	return p
}

func InitEmptyFuncNameContext(p *FuncNameContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_funcName
}

func (*FuncNameContext) IsFuncNameContext() {}

func NewFuncNameContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FuncNameContext {
	var p = new(FuncNameContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewFuncNameContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, SLQParserRULE_funcName)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(130)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&33555448) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IJoinContext is an interface to support dynamic dispatch.
type IJoinContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	JOIN_TYPE() antlr.TerminalNode
	LPAR() antlr.TerminalNode
	JoinTable() IJoinTableContext
	RPAR() antlr.TerminalNode
	COMMA() antlr.TerminalNode
	Expr() IExprContext

	// IsJoinContext differentiates from other interfaces.
	IsJoinContext()
}

type JoinContext struct {
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyJoinContext() *JoinContext {
	var p = new(JoinContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_join
	return p
}

func InitEmptyJoinContext(p *JoinContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_join
}

func (*JoinContext) IsJoinContext() {}

func NewJoinContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *JoinContext {
	var p = new(JoinContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_join

	return p
}

func (s *JoinContext) GetParser() antlr.Parser { return s.parser }

func (s *JoinContext) JOIN_TYPE() antlr.TerminalNode {
	return s.GetToken(SLQParserJOIN_TYPE, 0)
}

func (s *JoinContext) LPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserLPAR, 0)
}

func (s *JoinContext) JoinTable() IJoinTableContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IJoinTableContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IJoinTableContext)
}

func (s *JoinContext) RPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserRPAR, 0)
}

func (s *JoinContext) COMMA() antlr.TerminalNode {
	return s.GetToken(SLQParserCOMMA, 0)
}

func (s *JoinContext) Expr() IExprContext {
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
	p.EnterRule(localctx, 14, SLQParserRULE_join)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(132)
		p.Match(SLQParserJOIN_TYPE)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(133)
		p.Match(SLQParserLPAR)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(134)
		p.JoinTable()
	}
	p.SetState(137)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserCOMMA {
		{
			p.SetState(135)
			p.Match(SLQParserCOMMA)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(136)
			p.expr(0)
		}

	}
	{
		p.SetState(139)
		p.Match(SLQParserRPAR)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IJoinTableContext is an interface to support dynamic dispatch.
type IJoinTableContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	NAME() antlr.TerminalNode
	HANDLE() antlr.TerminalNode
	Alias() IAliasContext

	// IsJoinTableContext differentiates from other interfaces.
	IsJoinTableContext()
}

type JoinTableContext struct {
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyJoinTableContext() *JoinTableContext {
	var p = new(JoinTableContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_joinTable
	return p
}

func InitEmptyJoinTableContext(p *JoinTableContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_joinTable
}

func (*JoinTableContext) IsJoinTableContext() {}

func NewJoinTableContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *JoinTableContext {
	var p = new(JoinTableContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_joinTable

	return p
}

func (s *JoinTableContext) GetParser() antlr.Parser { return s.parser }

func (s *JoinTableContext) NAME() antlr.TerminalNode {
	return s.GetToken(SLQParserNAME, 0)
}

func (s *JoinTableContext) HANDLE() antlr.TerminalNode {
	return s.GetToken(SLQParserHANDLE, 0)
}

func (s *JoinTableContext) Alias() IAliasContext {
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

func (s *JoinTableContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *JoinTableContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *JoinTableContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterJoinTable(s)
	}
}

func (s *JoinTableContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitJoinTable(s)
	}
}

func (s *JoinTableContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitJoinTable(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) JoinTable() (localctx IJoinTableContext) {
	localctx = NewJoinTableContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 16, SLQParserRULE_joinTable)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(142)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserHANDLE {
		{
			p.SetState(141)
			p.Match(SLQParserHANDLE)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	}
	{
		p.SetState(144)
		p.Match(SLQParserNAME)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(146)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserALIAS_RESERVED || _la == SLQParserCOLON {
		{
			p.SetState(145)
			p.Alias()
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyUniqueFuncContext() *UniqueFuncContext {
	var p = new(UniqueFuncContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_uniqueFunc
	return p
}

func InitEmptyUniqueFuncContext(p *UniqueFuncContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_uniqueFunc
}

func (*UniqueFuncContext) IsUniqueFuncContext() {}

func NewUniqueFuncContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *UniqueFuncContext {
	var p = new(UniqueFuncContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewUniqueFuncContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 18, SLQParserRULE_uniqueFunc)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(148)
		_la = p.GetTokenStream().LA(1)

		if !(_la == SLQParserT__9 || _la == SLQParserT__10) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyCountFuncContext() *CountFuncContext {
	var p = new(CountFuncContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_countFunc
	return p
}

func InitEmptyCountFuncContext(p *CountFuncContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_countFunc
}

func (*CountFuncContext) IsCountFuncContext() {}

func NewCountFuncContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *CountFuncContext {
	var p = new(CountFuncContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewCountFuncContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 20, SLQParserRULE_countFunc)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(150)
		p.Match(SLQParserT__11)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(156)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserLPAR {
		{
			p.SetState(151)
			p.Match(SLQParserLPAR)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		p.SetState(153)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == SLQParserNAME {
			{
				p.SetState(152)
				p.Selector()
			}

		}
		{
			p.SetState(155)
			p.Match(SLQParserRPAR)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	}
	p.SetState(159)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserALIAS_RESERVED || _la == SLQParserCOLON {
		{
			p.SetState(158)
			p.Alias()
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IWhereContext is an interface to support dynamic dispatch.
type IWhereContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	WHERE() antlr.TerminalNode
	LPAR() antlr.TerminalNode
	RPAR() antlr.TerminalNode
	Expr() IExprContext

	// IsWhereContext differentiates from other interfaces.
	IsWhereContext()
}

type WhereContext struct {
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyWhereContext() *WhereContext {
	var p = new(WhereContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_where
	return p
}

func InitEmptyWhereContext(p *WhereContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_where
}

func (*WhereContext) IsWhereContext() {}

func NewWhereContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *WhereContext {
	var p = new(WhereContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_where

	return p
}

func (s *WhereContext) GetParser() antlr.Parser { return s.parser }

func (s *WhereContext) WHERE() antlr.TerminalNode {
	return s.GetToken(SLQParserWHERE, 0)
}

func (s *WhereContext) LPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserLPAR, 0)
}

func (s *WhereContext) RPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserRPAR, 0)
}

func (s *WhereContext) Expr() IExprContext {
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

func (s *WhereContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *WhereContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *WhereContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterWhere(s)
	}
}

func (s *WhereContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitWhere(s)
	}
}

func (s *WhereContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitWhere(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Where() (localctx IWhereContext) {
	localctx = NewWhereContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 22, SLQParserRULE_where)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(161)
		p.Match(SLQParserWHERE)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(162)
		p.Match(SLQParserLPAR)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(164)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&11285469010617336) != 0 {
		{
			p.SetState(163)
			p.expr(0)
		}

	}
	{
		p.SetState(166)
		p.Match(SLQParserRPAR)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyGroupByTermContext() *GroupByTermContext {
	var p = new(GroupByTermContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_groupByTerm
	return p
}

func InitEmptyGroupByTermContext(p *GroupByTermContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_groupByTerm
}

func (*GroupByTermContext) IsGroupByTermContext() {}

func NewGroupByTermContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *GroupByTermContext {
	var p = new(GroupByTermContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewGroupByTermContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 24, SLQParserRULE_groupByTerm)
	p.SetState(170)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case SLQParserNAME:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(168)
			p.Selector()
		}

	case SLQParserT__2, SLQParserT__3, SLQParserT__4, SLQParserT__5, SLQParserT__6, SLQParserT__7, SLQParserT__8, SLQParserPROPRIETARY_FUNC_NAME:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(169)
			p.Func_()
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyGroupByContext() *GroupByContext {
	var p = new(GroupByContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_groupBy
	return p
}

func InitEmptyGroupByContext(p *GroupByContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_groupBy
}

func (*GroupByContext) IsGroupByContext() {}

func NewGroupByContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *GroupByContext {
	var p = new(GroupByContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewGroupByContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 26, SLQParserRULE_groupBy)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(172)
		p.Match(SLQParserGROUP_BY)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(173)
		p.Match(SLQParserLPAR)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(174)
		p.GroupByTerm()
	}
	p.SetState(179)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserCOMMA {
		{
			p.SetState(175)
			p.Match(SLQParserCOMMA)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(176)
			p.GroupByTerm()
		}

		p.SetState(181)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(182)
		p.Match(SLQParserRPAR)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IHavingContext is an interface to support dynamic dispatch.
type IHavingContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	HAVING() antlr.TerminalNode
	LPAR() antlr.TerminalNode
	Expr() IExprContext
	RPAR() antlr.TerminalNode

	// IsHavingContext differentiates from other interfaces.
	IsHavingContext()
}

type HavingContext struct {
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyHavingContext() *HavingContext {
	var p = new(HavingContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_having
	return p
}

func InitEmptyHavingContext(p *HavingContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_having
}

func (*HavingContext) IsHavingContext() {}

func NewHavingContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *HavingContext {
	var p = new(HavingContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_having

	return p
}

func (s *HavingContext) GetParser() antlr.Parser { return s.parser }

func (s *HavingContext) HAVING() antlr.TerminalNode {
	return s.GetToken(SLQParserHAVING, 0)
}

func (s *HavingContext) LPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserLPAR, 0)
}

func (s *HavingContext) Expr() IExprContext {
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

func (s *HavingContext) RPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserRPAR, 0)
}

func (s *HavingContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *HavingContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *HavingContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterHaving(s)
	}
}

func (s *HavingContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitHaving(s)
	}
}

func (s *HavingContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitHaving(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) Having() (localctx IHavingContext) {
	localctx = NewHavingContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 28, SLQParserRULE_having)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(184)
		p.Match(SLQParserHAVING)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(185)
		p.Match(SLQParserLPAR)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(186)
		p.expr(0)
	}
	{
		p.SetState(187)
		p.Match(SLQParserRPAR)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IOrderByTermContext is an interface to support dynamic dispatch.
type IOrderByTermContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Selector() ISelectorContext

	// IsOrderByTermContext differentiates from other interfaces.
	IsOrderByTermContext()
}

type OrderByTermContext struct {
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyOrderByTermContext() *OrderByTermContext {
	var p = new(OrderByTermContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_orderByTerm
	return p
}

func InitEmptyOrderByTermContext(p *OrderByTermContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_orderByTerm
}

func (*OrderByTermContext) IsOrderByTermContext() {}

func NewOrderByTermContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *OrderByTermContext {
	var p = new(OrderByTermContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewOrderByTermContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 30, SLQParserRULE_orderByTerm)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(189)
		p.Selector()
	}
	p.SetState(191)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserT__12 || _la == SLQParserT__13 {
		{
			p.SetState(190)
			_la = p.GetTokenStream().LA(1)

			if !(_la == SLQParserT__12 || _la == SLQParserT__13) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyOrderByContext() *OrderByContext {
	var p = new(OrderByContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_orderBy
	return p
}

func InitEmptyOrderByContext(p *OrderByContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_orderBy
}

func (*OrderByContext) IsOrderByContext() {}

func NewOrderByContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *OrderByContext {
	var p = new(OrderByContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewOrderByContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 32, SLQParserRULE_orderBy)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(193)
		p.Match(SLQParserORDER_BY)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(194)
		p.Match(SLQParserLPAR)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(195)
		p.OrderByTerm()
	}
	p.SetState(200)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == SLQParserCOMMA {
		{
			p.SetState(196)
			p.Match(SLQParserCOMMA)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(197)
			p.OrderByTerm()
		}

		p.SetState(202)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(203)
		p.Match(SLQParserRPAR)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptySelectorContext() *SelectorContext {
	var p = new(SelectorContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_selector
	return p
}

func InitEmptySelectorContext(p *SelectorContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_selector
}

func (*SelectorContext) IsSelectorContext() {}

func NewSelectorContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SelectorContext {
	var p = new(SelectorContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewSelectorContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 34, SLQParserRULE_selector)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(205)
		p.Match(SLQParserNAME)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(207)
	p.GetErrorHandler().Sync(p)

	if p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 21, p.GetParserRuleContext()) == 1 {
		{
			p.SetState(206)
			p.Match(SLQParserNAME)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	} else if p.HasError() { // JIM
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptySelectorElementContext() *SelectorElementContext {
	var p = new(SelectorElementContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_selectorElement
	return p
}

func InitEmptySelectorElementContext(p *SelectorElementContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_selectorElement
}

func (*SelectorElementContext) IsSelectorElementContext() {}

func NewSelectorElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SelectorElementContext {
	var p = new(SelectorElementContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewSelectorElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 36, SLQParserRULE_selectorElement)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(209)
		p.Selector()
	}

	p.SetState(211)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserALIAS_RESERVED || _la == SLQParserCOLON {
		{
			p.SetState(210)
			p.Alias()
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	STRING() antlr.TerminalNode

	// IsAliasContext differentiates from other interfaces.
	IsAliasContext()
}

type AliasContext struct {
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyAliasContext() *AliasContext {
	var p = new(AliasContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_alias
	return p
}

func InitEmptyAliasContext(p *AliasContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_alias
}

func (*AliasContext) IsAliasContext() {}

func NewAliasContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *AliasContext {
	var p = new(AliasContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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

func (s *AliasContext) STRING() antlr.TerminalNode {
	return s.GetToken(SLQParserSTRING, 0)
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
	localctx = NewAliasContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 38, SLQParserRULE_alias)
	var _la int

	p.SetState(216)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case SLQParserALIAS_RESERVED:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(213)
			p.Match(SLQParserALIAS_RESERVED)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case SLQParserCOLON:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(214)
			p.Match(SLQParserCOLON)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(215)
			_la = p.GetTokenStream().LA(1)

			if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&9007220729577472) != 0) {
				p.GetErrorHandler().RecoverInline(p)
			} else {
				p.GetErrorHandler().ReportMatch(p)
				p.Consume()
			}
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyArgContext() *ArgContext {
	var p = new(ArgContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_arg
	return p
}

func InitEmptyArgContext(p *ArgContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_arg
}

func (*ArgContext) IsArgContext() {}

func NewArgContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ArgContext {
	var p = new(ArgContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewArgContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 40, SLQParserRULE_arg)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(218)
		p.Match(SLQParserARG)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyHandleTableContext() *HandleTableContext {
	var p = new(HandleTableContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_handleTable
	return p
}

func InitEmptyHandleTableContext(p *HandleTableContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_handleTable
}

func (*HandleTableContext) IsHandleTableContext() {}

func NewHandleTableContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *HandleTableContext {
	var p = new(HandleTableContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewHandleTableContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 42, SLQParserRULE_handleTable)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(220)
		p.Match(SLQParserHANDLE)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(221)
		p.Match(SLQParserNAME)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyHandleContext() *HandleContext {
	var p = new(HandleContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_handle
	return p
}

func InitEmptyHandleContext(p *HandleContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_handle
}

func (*HandleContext) IsHandleContext() {}

func NewHandleContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *HandleContext {
	var p = new(HandleContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewHandleContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 44, SLQParserRULE_handle)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(223)
		p.Match(SLQParserHANDLE)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyRowRangeContext() *RowRangeContext {
	var p = new(RowRangeContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_rowRange
	return p
}

func InitEmptyRowRangeContext(p *RowRangeContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_rowRange
}

func (*RowRangeContext) IsRowRangeContext() {}

func NewRowRangeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *RowRangeContext {
	var p = new(RowRangeContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewRowRangeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 46, SLQParserRULE_rowRange)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(225)
		p.Match(SLQParserT__14)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(234)
	p.GetErrorHandler().Sync(p)

	if p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 24, p.GetParserRuleContext()) == 1 {
		{
			p.SetState(226)
			p.Match(SLQParserNN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(227)
			p.Match(SLQParserCOLON)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(228)
			p.Match(SLQParserNN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	} else if p.HasError() { // JIM
		goto errorExit
	} else if p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 24, p.GetParserRuleContext()) == 2 {
		{
			p.SetState(229)
			p.Match(SLQParserNN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(230)
			p.Match(SLQParserCOLON)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	} else if p.HasError() { // JIM
		goto errorExit
	} else if p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 24, p.GetParserRuleContext()) == 3 {
		{
			p.SetState(231)
			p.Match(SLQParserCOLON)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(232)
			p.Match(SLQParserNN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	} else if p.HasError() { // JIM
		goto errorExit
	} else if p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 24, p.GetParserRuleContext()) == 4 {
		{
			p.SetState(233)
			p.Match(SLQParserNN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	} else if p.HasError() { // JIM
		goto errorExit
	}
	{
		p.SetState(236)
		p.Match(SLQParserRBRA)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IExprElementContext is an interface to support dynamic dispatch.
type IExprElementContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Expr() IExprContext
	Alias() IAliasContext

	// IsExprElementContext differentiates from other interfaces.
	IsExprElementContext()
}

type ExprElementContext struct {
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyExprElementContext() *ExprElementContext {
	var p = new(ExprElementContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_exprElement
	return p
}

func InitEmptyExprElementContext(p *ExprElementContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_exprElement
}

func (*ExprElementContext) IsExprElementContext() {}

func NewExprElementContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ExprElementContext {
	var p = new(ExprElementContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_exprElement

	return p
}

func (s *ExprElementContext) GetParser() antlr.Parser { return s.parser }

func (s *ExprElementContext) Expr() IExprContext {
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

func (s *ExprElementContext) Alias() IAliasContext {
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

func (s *ExprElementContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ExprElementContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ExprElementContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.EnterExprElement(s)
	}
}

func (s *ExprElementContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(SLQListener); ok {
		listenerT.ExitExprElement(s)
	}
}

func (s *ExprElementContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case SLQVisitor:
		return t.VisitExprElement(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *SLQParser) ExprElement() (localctx IExprElementContext) {
	localctx = NewExprElementContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 48, SLQParserRULE_exprElement)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(238)
		p.expr(0)
	}
	p.SetState(240)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == SLQParserALIAS_RESERVED || _la == SLQParserCOLON {
		{
			p.SetState(239)
			p.Alias()
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IExprContext is an interface to support dynamic dispatch.
type IExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	LPAR() antlr.TerminalNode
	AllExpr() []IExprContext
	Expr(i int) IExprContext
	RPAR() antlr.TerminalNode
	Selector() ISelectorContext
	Literal() ILiteralContext
	Arg() IArgContext
	UnaryOperator() IUnaryOperatorContext
	Func_() IFuncContext
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyExprContext() *ExprContext {
	var p = new(ExprContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_expr
	return p
}

func InitEmptyExprContext(p *ExprContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_expr
}

func (*ExprContext) IsExprContext() {}

func NewExprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ExprContext {
	var p = new(ExprContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = SLQParserRULE_expr

	return p
}

func (s *ExprContext) GetParser() antlr.Parser { return s.parser }

func (s *ExprContext) LPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserLPAR, 0)
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

func (s *ExprContext) RPAR() antlr.TerminalNode {
	return s.GetToken(SLQParserRPAR, 0)
}

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
	var _parentctx antlr.ParserRuleContext = p.GetParserRuleContext()

	_parentState := p.GetState()
	localctx = NewExprContext(p, p.GetParserRuleContext(), _parentState)
	var _prevctx IExprContext = localctx
	var _ antlr.ParserRuleContext = _prevctx // TODO: To prevent unused variable warning.
	_startState := 50
	p.EnterRecursionRule(localctx, 50, SLQParserRULE_expr, _p)
	var _la int

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(254)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case SLQParserLPAR:
		{
			p.SetState(243)
			p.Match(SLQParserLPAR)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(244)
			p.expr(0)
		}
		{
			p.SetState(245)
			p.Match(SLQParserRPAR)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case SLQParserNAME:
		{
			p.SetState(247)
			p.Selector()
		}

	case SLQParserNULL, SLQParserNN, SLQParserNUMBER, SLQParserSTRING:
		{
			p.SetState(248)
			p.Literal()
		}

	case SLQParserARG:
		{
			p.SetState(249)
			p.Arg()
		}

	case SLQParserT__12, SLQParserT__13, SLQParserT__22, SLQParserT__23:
		{
			p.SetState(250)
			p.UnaryOperator()
		}
		{
			p.SetState(251)
			p.expr(9)
		}

	case SLQParserT__2, SLQParserT__3, SLQParserT__4, SLQParserT__5, SLQParserT__6, SLQParserT__7, SLQParserT__8, SLQParserPROPRIETARY_FUNC_NAME:
		{
			p.SetState(253)
			p.Func_()
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}
	p.GetParserRuleContext().SetStop(p.GetTokenStream().LT(-1))
	p.SetState(283)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 29, p.GetParserRuleContext())
	if p.HasError() {
		goto errorExit
	}
	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			if p.GetParseListeners() != nil {
				p.TriggerExitRuleEvent()
			}
			_prevctx = localctx
			p.SetState(281)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}

			switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 28, p.GetParserRuleContext()) {
			case 1:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(256)

				if !(p.Precpred(p.GetParserRuleContext(), 8)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 8)", ""))
					goto errorExit
				}
				{
					p.SetState(257)
					p.Match(SLQParserT__15)
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				{
					p.SetState(258)
					p.expr(9)
				}

			case 2:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(259)

				if !(p.Precpred(p.GetParserRuleContext(), 7)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 7)", ""))
					goto errorExit
				}
				{
					p.SetState(260)
					_la = p.GetTokenStream().LA(1)

					if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&393220) != 0) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(261)
					p.expr(8)
				}

			case 3:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(262)

				if !(p.Precpred(p.GetParserRuleContext(), 6)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 6)", ""))
					goto errorExit
				}
				{
					p.SetState(263)
					_la = p.GetTokenStream().LA(1)

					if !(_la == SLQParserT__12 || _la == SLQParserT__13) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(264)
					p.expr(7)
				}

			case 4:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(265)

				if !(p.Precpred(p.GetParserRuleContext(), 5)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 5)", ""))
					goto errorExit
				}
				{
					p.SetState(266)
					_la = p.GetTokenStream().LA(1)

					if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&3670016) != 0) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(267)
					p.expr(6)
				}

			case 5:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(268)

				if !(p.Precpred(p.GetParserRuleContext(), 4)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 4)", ""))
					goto errorExit
				}
				{
					p.SetState(269)
					_la = p.GetTokenStream().LA(1)

					if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&527765581332480) != 0) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(270)
					p.expr(5)
				}

			case 6:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(271)

				if !(p.Precpred(p.GetParserRuleContext(), 3)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 3)", ""))
					goto errorExit
				}
				p.SetState(275)
				p.GetErrorHandler().Sync(p)
				if p.HasError() {
					goto errorExit
				}

				switch p.GetTokenStream().LA(1) {
				case SLQParserEQ:
					{
						p.SetState(272)
						p.Match(SLQParserEQ)
						if p.HasError() {
							// Recognition error - abort rule
							goto errorExit
						}
					}

				case SLQParserNEQ:
					{
						p.SetState(273)
						p.Match(SLQParserNEQ)
						if p.HasError() {
							// Recognition error - abort rule
							goto errorExit
						}
					}

				case SLQParserT__2, SLQParserT__3, SLQParserT__4, SLQParserT__5, SLQParserT__6, SLQParserT__7, SLQParserT__8, SLQParserT__12, SLQParserT__13, SLQParserT__22, SLQParserT__23, SLQParserPROPRIETARY_FUNC_NAME, SLQParserARG, SLQParserNULL, SLQParserLPAR, SLQParserNN, SLQParserNUMBER, SLQParserNAME, SLQParserSTRING:

				default:
					p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
					goto errorExit
				}
				{
					p.SetState(277)
					p.expr(4)
				}

			case 7:
				localctx = NewExprContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, SLQParserRULE_expr)
				p.SetState(278)

				if !(p.Precpred(p.GetParserRuleContext(), 2)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 2)", ""))
					goto errorExit
				}
				{
					p.SetState(279)
					p.Match(SLQParserT__21)
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				{
					p.SetState(280)
					p.expr(3)
				}

			case antlr.ATNInvalidAltNumber:
				goto errorExit
			}

		}
		p.SetState(285)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 29, p.GetParserRuleContext())
		if p.HasError() {
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.UnrollRecursionContexts(_parentctx)
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyLiteralContext() *LiteralContext {
	var p = new(LiteralContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_literal
	return p
}

func InitEmptyLiteralContext(p *LiteralContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_literal
}

func (*LiteralContext) IsLiteralContext() {}

func NewLiteralContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *LiteralContext {
	var p = new(LiteralContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewLiteralContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 52, SLQParserRULE_literal)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(286)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&9033596123742208) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
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
	parser antlr.Parser
	antlr.BaseParserRuleContext
}

func NewEmptyUnaryOperatorContext() *UnaryOperatorContext {
	var p = new(UnaryOperatorContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_unaryOperator
	return p
}

func InitEmptyUnaryOperatorContext(p *UnaryOperatorContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = SLQParserRULE_unaryOperator
}

func (*UnaryOperatorContext) IsUnaryOperatorContext() {}

func NewUnaryOperatorContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *UnaryOperatorContext {
	var p = new(UnaryOperatorContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

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
	localctx = NewUnaryOperatorContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 54, SLQParserRULE_unaryOperator)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(288)
		_la = p.GetTokenStream().LA(1)

		if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&25190400) != 0) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

func (p *SLQParser) Sempred(localctx antlr.RuleContext, ruleIndex, predIndex int) bool {
	switch ruleIndex {
	case 25:
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

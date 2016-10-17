package main

import (
	"github.com/neilotoole/gotils/testing"

	"path/filepath"

	"fmt"
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/neilotoole/go-lg/lg"
	_ "github.com/neilotoole/sq/libsq/driver/impl"
)

func main() {
	//testing.Run(parser.TestScanner_Scan, parser.TestScanner_ScanString)
	//testing.Run(parser.TestToken_equals, parser.TestToken_Includes)
	//testing.Run(parser.TestParser_Parse)
	//testing.Run(parser.TestParser_ParseError)
	//testing.Run(parser.TestIsEven)
	//testing.Run(parser.TestParse)

	//tests := []testing.Test{
	//	token.TestScanner_Scan,
	//	token.TestScanner_ScanString,
	//	token.TestToken_equals,
	//	token.TestBuffer_IncludeExclude,
	//	token.TestNewBufferFromScanner,
	//	token.TestBuffer_CurserOps,
	//	token.TestBuffer_Until,
	//	parser.TestParser_Parse,
	//	parser.TestParser_ParseError,
	//	parser.TestIsEven,
	//}

	//testing.Run(tests...)
	//
	//tests := []testing.Test{
	//	//ir.TestIR_Build,
	//	ql.TestSegment,
	//	ql.TestTypes_IsNode,
	//	//ir.TestTypes_Actual,
	//	ql.TestToTreeString,
	//	ql.TestWalker,
	//	ql.TestChildIndex,
	//	ql.TestRowRange1,
	//	ql.TestRowRange2,
	//	ql.TestRowRange3,
	//	ql.TestRowRange4,
	//	ql.TestRowRange5,
	//	ql.TestRowRange6,
	//	ql.TestBuild2,
	//}
	tests := []testing.Test{
		////ir.TestIR_Build,
		//ql.TestSegment,
		//ql.TestTypes_IsNode,
		////ir.TestTypes_Actual,
		//ql.TestToTreeString,
		//ql.TestWalker,
		//ql.TestChildIndex,
		//ql.TestRowRange1,
		//ql.TestRowRange2,
		//ql.TestRowRange3,
		//ql.TestRowRange4,
		//ql.TestRowRange5,
		//ql.TestRowRange6,
		query.TestBuild2,
		//ql.TestInspector_findSegments,
		query.TestInspector_findSelectableSegments,
	}

	testing.Run("ql", tests...)
	//
	//tests = []testing.Test{
	//	driver.TestDataSources,
	//	driver.TestSourceGetTypeFromRef,
	//	driver.TestSource_Driver,
	//	xslx.TestGenExcelColNames,
	//}
	//
	//testing.Run("driver", tests...)

	//testing.Run(ir.TestSegment)
}

func init() {
	home, err := homedir.Dir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to get user homedir: %v", err)
		os.Exit(1)
	}

	path := filepath.Join(home, ".sq", "sq.log")
	//os.Setenv("__LG_LOG_FILEPATH", path)
	logFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, fmt.Sprintf("Error: unable to initialize log file: %v", err))
		os.Exit(1)
	}
	lg.Use(logFile)

	//lg.Exclude("github.com/neilotoole/sq/sq/ql")
}

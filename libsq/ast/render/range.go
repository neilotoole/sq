package render

import (
	"fmt"
	"math"

	"github.com/neilotoole/sq/libsq/ast"
)

func doRange(_ *Context, rr *ast.RowRangeNode) (string, error) {
	if rr == nil {
		return "", nil
	}

	if rr.Limit < 0 && rr.Offset < 0 {
		return "", nil
	}

	limit := ""
	offset := ""
	if rr.Limit > -1 {
		limit = fmt.Sprintf("LIMIT %d", rr.Limit)
	}
	if rr.Offset > -1 {
		offset = fmt.Sprintf("OFFSET %d", rr.Offset)

		if rr.Limit == -1 {
			// MySQL requires a LIMIT if OFFSET is used. Therefore
			// we make the LIMIT a very large number
			limit = fmt.Sprintf("LIMIT %d", uint64(math.MaxInt64))
		}
	}

	sql := AppendSQL(limit, offset)
	return sql, nil
}

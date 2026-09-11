package mysql

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// extractColumnDef returns the column definition (everything after the
// backtick-quoted column name, minus a trailing comma) for col, parsed from the
// output of SHOW CREATE TABLE. This reuses MySQL's own canonical DDL so the
// full definition (type, nullability, default, charset/collation,
// AUTO_INCREMENT, comment) is preserved verbatim.
func extractColumnDef(showCreate, col string) (string, error) {
	prefix := stringz.BacktickQuote(col) // `col`
	for _, line := range strings.Split(showCreate, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix+" ") {
			continue
		}
		def := strings.TrimSpace(trimmed[len(prefix):])
		def = strings.TrimSuffix(def, ",")
		return strings.TrimSpace(def), nil
	}
	return "", errz.Errorf("column %q not found in SHOW CREATE TABLE output", col)
}

package sqlz

import (
	"database/sql"
	"strings"
)

// NullBool is similar to sql.NullBool, but accepts more boolean
// representations, e.g. "YES", "Y", "NO", "N", etc. These boolean
// values are returned by SQL Server and Postgres at times.
type NullBool struct {
	sql.NullBool
}

// Scan implements the Scanner interface.
func (n *NullBool) Scan(value any) error {
	if value == nil {
		n.Bool, n.Valid = false, false
		return nil
	}

	if s, ok := value.(string); ok {
		s = strings.ToLower(strings.TrimSpace(s))
		switch s {
		case "yes", "y":
			n.Bool, n.Valid = true, true
			return nil
		case "no", "n":
			n.Bool, n.Valid = false, true
			return nil
		default:
			// let sql.NullBool.Scan handle it
		}
	}

	return n.NullBool.Scan(value)
}

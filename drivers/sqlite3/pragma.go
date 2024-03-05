package sqlite3

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/debugz"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// getDBProperties returns a map of the DB's settings, as exposed
// via SQLite's pragma mechanism. The supplied incr func should
// be invoked for each row read from the DB.
//
// See: https://www.sqlite.org/pragma.html
func getDBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	pragmas, err := listPragmaNames(ctx, db)
	if err != nil {
		return nil, err
	}

	m := make(map[string]any, len(pragmas))
	for _, pragma := range pragmas {
		var val any
		val, err = readPragma(ctx, db, pragma)
		if err != nil {
			return nil, errz.Wrapf(errw(err), "read pragma: %s", pragma)
		}

		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		if val != nil {
			m[pragma] = val
		}
	}

	return m, nil
}

// readPragma reads the values of pragma from the DB,and returns its value,
// which is either a scalar value such as a string, or a map[string]any.
func readPragma(ctx context.Context, db sqlz.DB, pragma string) (any, error) {
	var (
		q    = fmt.Sprintf(`SELECT * FROM "pragma_%s"`, pragma)
		rows *sql.Rows
		err  error
	)

	if rows, err = db.QueryContext(ctx, q); err != nil {
		if strings.HasPrefix(err.Error(), "no such table") {
			// Some of the pragmas can't be selected from. Ignore these.
			// SQLite returns a generic (1) SQLITE_ERROR in this case,
			// so we match using the error string.
			return nil, nil //nolint:nilnil
		}

		return nil, errw(err)
	}

	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	if !rows.Next() {
		return nil, nil //nolint:nilnil
	}

	cols, err := rows.Columns()
	if err != nil {
		return nil, errw(err)
	}

	switch len(cols) {
	case 0:
		// Shouldn't happen
		return nil, nil //nolint:nilnil
	case 1:
		var val any
		if err = rows.Scan(&val); err != nil {
			return nil, errw(err)
		}

		return val, nil
	default:
		// continue below
	}

	arr := make([]any, 0)
	for {
		vals := make([]any, len(cols))
		for i := range vals {
			vals[i] = new(any)
		}
		if err = rows.Scan(vals...); err != nil {
			return nil, errw(err)
		}

		m := map[string]any{}
		for i := range cols {
			v := vals[i]
			switch v := v.(type) {
			case nil:
				m[cols[i]] = nil
			case *any:
				if v == nil {
					m[cols[i]] = nil
				} else {
					m[cols[i]] = *v
				}
			default:
				m[cols[i]] = vals[i]
			}
		}

		arr = append(arr, m)

		if !rows.Next() {
			break
		}
	}

	return arr, nil
}

// listPragmaNames lists the pragmas from pragma_pragma_list.
// See: https://www.sqlite.org/pragma.html#pragma_pragma_list
func listPragmaNames(ctx context.Context, db sqlz.DB) ([]string, error) {
	const qPragmas = `SELECT name FROM pragma_pragma_list ORDER BY name`

	rows, err := db.QueryContext(ctx, qPragmas)
	if err != nil {
		return nil, errw(err)
	}

	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	var (
		names []string
		name  string
	)
	for rows.Next() {
		if err = rows.Scan(&name); err != nil {
			return nil, errw(err)
		}

		names = append(names, name)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return names, nil
}

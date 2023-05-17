package sqlite3

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
)

func (d *database) getPragmas(ctx context.Context) (map[string]any, error) {
	pragmas, err := d.listPragmas(ctx)
	if err != nil {
		return nil, err
	}

	m := make(map[string]any, len(pragmas))
	for _, pragma := range pragmas {
		var val any
		val, err = d.readPragma(ctx, pragma)
		if err != nil {
			return nil, errz.Wrapf(err, "read pragma: %s", pragma)
		}

		if val != nil {
			m[pragma] = val
		}
	}

	return m, nil
}

// readPragma reads the values of pragma from the DB,and returns its value,
// which is either a scalar value such as a string, or a map[string]any.
func (d *database) readPragma(ctx context.Context, pragma string) (any, error) {
	var (
		q    = fmt.Sprintf(`SELECT * FROM "pragma_%s"`, pragma)
		rows *sql.Rows
		err  error
	)

	if rows, err = d.db.QueryContext(ctx, q); err != nil {
		if strings.HasPrefix(err.Error(), "no such table") {
			// Some of the pragmas can't be selected from. Ignore these.
			// SQLite returns a generic (1) SQLITE_ERROR in this case,
			// so we match using the error string.
			return nil, nil
		}

		return nil, errz.Err(err)
	}

	defer lg.WarnIfCloseError(lg.FromContext(ctx), lgm.CloseDBRows, rows)

	if !rows.Next() {
		return nil, nil
	}

	cols, err := rows.Columns()
	if err != nil {
		return nil, errz.Err(err)
	}

	switch len(cols) {
	case 0:
		// Shouldn't happen
		return nil, nil
	case 1:
		var val any
		if err = rows.Scan(&val); err != nil {
			return nil, errz.Err(err)
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
			return nil, errz.Err(err)
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

func (d *database) listPragmas(ctx context.Context) ([]string, error) {
	const qPragmas = `SELECT name FROM pragma_pragma_list ORDER BY name`

	rows, err := d.db.QueryContext(ctx, qPragmas)
	if err != nil {
		return nil, errz.Err(err)
	}

	defer lg.WarnIfCloseError(lg.FromContext(ctx), lgm.CloseDBRows, rows)

	var (
		names []string
		name  string
	)
	for rows.Next() {
		if err = rows.Scan(&name); err != nil {
			return nil, errz.Err(err)
		}

		names = append(names, name)
	}

	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	return names, nil
}

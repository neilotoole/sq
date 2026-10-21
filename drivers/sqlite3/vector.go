package sqlite3

import (
	"context"
	"database/sql"
	"errors"

	mattn "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/drivers/sqlite3/vecdecl"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/vector"
	"github.com/neilotoole/sq/libsq/driver"
)

// vectorDecoder decodes the scanned values of a vector result column.
type vectorDecoder struct {
	// typ is the column's declared vector type.
	typ vector.Type

	// name is the column name, used in error messages.
	name string

	// idx is the column index.
	idx int

	// vec0 is true for a sqlite-vec vec0 column, and false for a libSQL
	// declared vector column.
	vec0 bool
}

// newVectorDecoders returns decoders for the kind.Vector columns of recMeta,
// in column order, or nil if there are none. A column with a vector hint is
// a vec0 column; otherwise the column's declared type must be a libSQL
// vector type.
func newVectorDecoders(recMeta record.Meta, hints driver.ResultHints) []vectorDecoder {
	var decoders []vectorDecoder
	for i, fm := range recMeta {
		if fm.Kind() != kind.Vector {
			continue
		}

		dec := vectorDecoder{idx: i, name: fm.MungedName()}
		if vt, ok := hints.Vectors[i]; ok {
			dec.typ, dec.vec0 = vt, true
		} else if vt, ok = vecdecl.ParseLibSQLType(fm.DatabaseTypeName()); ok {
			dec.typ = vt
		} else {
			continue
		}

		decoders = append(decoders, dec)
	}
	return decoders
}

// decode decodes the scanned value val, returning a vector.Vector, or nil if
// val is NULL. Row is the 1-based row number, used in error messages.
func (vd vectorDecoder) decode(val any, row int) (any, error) {
	var (
		v   vector.Vector
		err error
	)

	switch val := unwrapScanned(val).(type) {
	case nil:
		return nil, nil //nolint:nilnil // NULL stays nil.
	case []byte:
		if vd.vec0 {
			v, err = vector.DecodeVec0(val, vd.typ)
		} else {
			v, err = vector.DecodeLibSQL(val, vd.typ)
		}
	case string:
		// libSQL stores a vector inserted as JSON text unchanged.
		if vd.vec0 {
			err = errz.New("invalid vec0 vector: unexpected text value")
		} else {
			v, err = vector.ParseJSON(val, vd.typ)
		}
	default:
		err = errz.Errorf("invalid %s vector: unexpected value of type %T", vd.typ.Elem, val)
	}

	if err != nil {
		return nil, errz.Wrapf(err, "column %q, row %d", vd.name, row)
	}
	return v, nil
}

// unwrapScanned returns the value held by val, a scan destination from
// rows.Scan, before newRecordFromScanRow converts it. It returns nil for
// NULL.
func unwrapScanned(val any) any {
	switch val := val.(type) {
	case *any:
		if val == nil {
			return nil
		}
		return *val
	case *[]byte:
		if val == nil || *val == nil {
			return nil
		}
		return *val
	case *sql.RawBytes:
		if val == nil || *val == nil {
			return nil
		}
		return []byte(*val)
	case *sql.NullString:
		if val == nil || !val.Valid {
			return nil
		}
		return val.String
	case *string:
		if val == nil {
			return nil
		}
		return *val
	default:
		return val
	}
}

// vec0TableHints returns the vector columns of the vec0 table tbl, declared
// by ddl, and the table's visible columns in SELECT * order. Both are nil if
// the table has no vector columns, or if SQLite can't open the table: the
// query against the table then reports SQLite's error.
func vec0TableHints(ctx context.Context, db sqlz.DB, tbl, ddl string) (map[string]vector.Type, []string, error) {
	vecCols, err := vecdecl.ParseVec0(ddl)
	if err != nil {
		return nil, nil, errz.Wrapf(err, "sqlite3: table %s", stringz.DoubleQuote(tbl))
	}

	var vectors map[string]vector.Type
	for _, col := range vecCols {
		if col.Vector == nil {
			continue
		}
		if vectors == nil {
			vectors = make(map[string]vector.Type)
		}
		vectors[col.Name] = *col.Vector
	}
	if len(vectors) == 0 {
		return nil, nil, nil
	}

	cols, err := visibleColumnNames(ctx, db, tbl)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, nil, errz.Err(ctxErr)
		}
		var sqliteErr mattn.Error
		if errors.As(err, &sqliteErr) {
			lg.FromContext(ctx).Debug("Skip vector hints for unopenable vec0 table",
				lga.Table, tbl, lga.Err, err)
			return nil, nil, nil
		}
		return nil, nil, errw(err)
	}

	return vectors, cols, nil
}

// visibleColumnNames returns the names of tbl's visible columns, in SELECT *
// order. pragma_table_info omits hidden columns, such as vec0's distance and
// k. The returned error is not wrapped.
func visibleColumnNames(ctx context.Context, db sqlz.DB, tbl string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_info(?) ORDER BY cid`, tbl)
	if err != nil {
		return nil, err
	}
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	var cols []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

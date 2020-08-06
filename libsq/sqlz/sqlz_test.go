package sqlz_test

import (
	"database/sql"
	"reflect"

	"github.com/neilotoole/sq/libsq/sqlz"
)

// stdlibColumnType exists to verify that sql.ColumnType
// and FieldMeta conform to a common (sql.ColumnType's)
// method set.
type stdlibColumnType interface {
	Name() string
	Length() (length int64, ok bool)
	DecimalSize() (precision, scale int64, ok bool)
	ScanType() reflect.Type
	Nullable() (nullable, ok bool)
	DatabaseTypeName() string
}

var _ stdlibColumnType = (*sql.ColumnType)(nil)
var _ stdlibColumnType = (*sqlz.FieldMeta)(nil)

package sqlz

import (
	"database/sql"
	"reflect"
	"time"

	"github.com/shopspring/decimal"
)

// Cached results from reflect.TypeOf for commonly used types.
var (
	RTypeInt         = reflect.TypeOf(0)
	RTypeInt8        = reflect.TypeOf(int8(0))
	RTypeInt16       = reflect.TypeOf(int16(0))
	RTypeInt32       = reflect.TypeOf(int32(0))
	RTypeInt64       = reflect.TypeOf(int64(0))
	RTypeNullInt64   = reflect.TypeOf(sql.NullInt64{})
	RTypeDecimal     = reflect.TypeOf(decimal.Decimal{})
	RTypeNullDecimal = reflect.TypeOf(decimal.NullDecimal{})
	RTypeFloat32     = reflect.TypeOf(float32(0))
	RTypeFloat64     = reflect.TypeOf(float64(0))
	RTypeNullFloat64 = reflect.TypeOf(sql.NullFloat64{})
	RTypeBool        = reflect.TypeOf(true)
	RTypeNullBool    = reflect.TypeOf(sql.NullBool{})
	RTypeString      = reflect.TypeOf("")
	RTypeNullString  = reflect.TypeOf(sql.NullString{})
	RTypeTime        = reflect.TypeOf(time.Time{})
	RTypeNullTime    = reflect.TypeOf(sql.NullTime{})
	RTypeBytes       = reflect.TypeOf([]byte{})
	RTypeNil         = reflect.TypeOf(nil)
	RTypeAny         = reflect.TypeOf((any)(nil))
)

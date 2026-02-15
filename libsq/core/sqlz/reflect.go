package sqlz

import (
	"database/sql"
	"reflect"
	"time"

	"github.com/shopspring/decimal"
)

// Cached results from reflect.TypeOf for commonly used types.
var (
	RTypeInt         = reflect.TypeFor[int]()
	RTypeInt8        = reflect.TypeFor[int8]()
	RTypeInt16       = reflect.TypeFor[int16]()
	RTypeInt32       = reflect.TypeFor[int32]()
	RTypeInt64       = reflect.TypeFor[int64]()
	RTypeNullInt64   = reflect.TypeFor[sql.NullInt64]()
	RTypeDecimal     = reflect.TypeFor[decimal.Decimal]()
	RTypeNullDecimal = reflect.TypeFor[decimal.NullDecimal]()
	RTypeFloat32     = reflect.TypeFor[float32]()
	RTypeFloat64     = reflect.TypeFor[float64]()
	RTypeNullFloat64 = reflect.TypeFor[sql.NullFloat64]()
	RTypeBool        = reflect.TypeFor[bool]()
	RTypeNullBool    = reflect.TypeFor[sql.NullBool]()
	RTypeString      = reflect.TypeFor[string]()
	RTypeNullString  = reflect.TypeFor[sql.NullString]()
	RTypeTime        = reflect.TypeFor[time.Time]()
	RTypeNullTime    = reflect.TypeFor[sql.NullTime]()
	RTypeBytes       = reflect.TypeFor[[]byte]()
	RTypeNil         = reflect.TypeOf(nil)
	RTypeAny         = reflect.TypeOf((any)(nil))
)

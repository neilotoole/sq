package sqlz

import (
	"database/sql"
	"reflect"
	"time"

	"github.com/twpayne/go-geom/encoding/wkb"
)

// Cached results from reflect.TypeOf for commonly used types.
var (
	RTypeInt         = reflect.TypeOf(0)
	RTypeInt8        = reflect.TypeOf(int8(0))
	RTypeInt16       = reflect.TypeOf(int16(0))
	RTypeInt32       = reflect.TypeOf(int32(0))
	RTypeInt64       = reflect.TypeOf(int64(0))
	RTypeInt64P      = reflect.TypeOf((*int64)(nil))
	RTypeNullInt64   = reflect.TypeOf(sql.NullInt64{})
	RTypeFloat32     = reflect.TypeOf(float32(0))
	RTypeFloat64     = reflect.TypeOf(float64(0))
	RTypeFloat64P    = reflect.TypeOf((*float64)(nil))
	RTypeNullFloat64 = reflect.TypeOf(sql.NullFloat64{})
	RTypeBool        = reflect.TypeOf(true)
	RTypeBoolP       = reflect.TypeOf((*bool)(nil))
	RTypeNullBool    = reflect.TypeOf(sql.NullBool{})
	RTypeString      = reflect.TypeOf("")
	RTypeStringP     = reflect.TypeOf((*string)(nil))
	RTypeNullString  = reflect.TypeOf(sql.NullString{})
	RTypeTime        = reflect.TypeOf(time.Time{})
	RTypeTimeP       = reflect.TypeOf((*time.Time)(nil))
	RTypeNullTime    = reflect.TypeOf(sql.NullTime{})
	RTypeBytes       = reflect.TypeOf([]byte{})
	RTypeBytesP      = reflect.TypeOf((*[]byte)(nil))
	RTypeNil         = reflect.TypeOf(nil)

	RTypeGeoPoint           = reflect.TypeOf((*wkb.Point)(nil))
	RTypeGeoLineString      = reflect.TypeOf((*wkb.LineString)(nil))
	RTypeGeoPolygon         = reflect.TypeOf((*wkb.Polygon)(nil))
	RTypeGeoMultiPoint      = reflect.TypeOf((*wkb.MultiPoint)(nil))
	RTypeGeoMultiLineString = reflect.TypeOf((*wkb.MultiLineString)(nil))
	RTypeGeoMultiPolygon    = reflect.TypeOf((*wkb.MultiPolygon)(nil))
	RTypeGeoCollection      = reflect.TypeOf((*wkb.GeometryCollection)(nil))
)

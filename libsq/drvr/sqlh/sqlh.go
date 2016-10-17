// Package sqlh "SQL Helper" provides functionality for working with stdlib sql package.
package sqlh

import (
	"fmt"

	"time"

	"errors"

	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq-driver/hackery/database/sql/driver"
	"github.com/neilotoole/sq/libsq/drvr/datatype"
)

func ScanDest(dt datatype.Type) interface{} {

	switch dt {
	case datatype.Text:
		return &sql.NullString{}
	case datatype.Int:
		return &sql.NullInt64{}
	case datatype.Float:
		return &sql.NullFloat64{}
	case datatype.Bool:
		return &sql.NullBool{}
	case datatype.DateTime:
		// TODO: for now we return a NullString, we'll get to NullTime later
		return &sql.NullString{}
		//return &NullTime{}
	case datatype.Bytes:
		return &sql.NullString{}
		//return NullBytes{}
	case datatype.Null:
		return &sql.NullString{}
		//return &NullNull{}
	}

	return &sql.NullString{}
	//panic(fmt.Sprintf("unknown data type %q", dt))
}

func ScanDests(dts []datatype.Type) []interface{} {
	dests := make([]interface{}, len(dts))

	for i := range dests {
		dests[i] = ScanDest(dts[i])
	}

	return dests
}

func DataTypeFromCols(colTypes []driver.ColumnType) ([]datatype.Type, error) {

	dts := make([]datatype.Type, len(colTypes))

	for i := range colTypes {

		f := colTypes[i].FieldType
		switch f {
		case driver.FieldTypeDecimal, driver.FieldTypeNewDecimal:
			// decimal
			dts[i] = datatype.Decimal
		case driver.FieldTypeBit,
			driver.FieldTypeTiny,
			driver.FieldTypeShort,
			driver.FieldTypeLong,
			driver.FieldTypeLongLong,
			driver.FieldTypeInt24:
			// int
			dts[i] = datatype.Int
		case driver.FieldTypeFloat, driver.FieldTypeDouble:
			// float
			dts[i] = datatype.Float
		case driver.FieldTypeNULL:
			// null
			dts[i] = datatype.Null
		case driver.FieldTypeTimestamp,
			driver.FieldTypeDate,
			driver.FieldTypeTime,
			driver.FieldTypeDateTime,
			driver.FieldTypeYear,
			driver.FieldTypeNewDate:
			// datetime
			dts[i] = datatype.DateTime
		case driver.FieldTypeTinyBLOB,
			driver.FieldTypeMediumBLOB,
			driver.FieldTypeLongBLOB,
			driver.FieldTypeBLOB:
			dts[i] = datatype.Bytes
		case driver.FieldTypeVarChar,
			driver.FieldTypeVarString,
			driver.FieldTypeString,
			driver.FieldTypeEnum,
			driver.FieldTypeSet:
			dts[i] = datatype.Text

		default:
			// default to text
			dts[i] = datatype.Text
		}

	}

	return dts, nil
}

// REFERENCE (from sql.driver)

// Value is a value that drivers must be able to handle.
// It is either nil or an instance of one of these types:
//
//   int64
//   float64
//   bool
//   []byte
//   string   [*] everywhere except from Rows.Next.
//   time.Time

// TODO: probably don't need these NullXYZ types, they were here more to see
// if it made using the sql API easier, but prob not needed

// NullTime represents a time.Time that may be null. NullTime implements the
// sql.Scanner interface so it can be used as a scan destination, similar to
// sql.NullString.
type NullTime struct {
	Time  time.Time
	Valid bool // Valid is true if Time is not NULL
}

// Scan implements the Scanner interface.
func (n *NullTime) Scan(value interface{}) error {
	fmt.Printf("nulltime: %T: %v\n", value, value)
	n.Time, n.Valid = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (n NullTime) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Time, nil
}

// NullBytes represents a []byte that may be null. NullBytes implements the
// sql.Scanner interface so it can be used as a scan destination, similar to
// sql.NullString.
type NullBytes struct {
	Bytes []byte
	Valid bool // Valid is true if Time is not NULL
}

// Scan implements the Scanner interface.
func (n *NullBytes) Scan(value interface{}) error {

	if value == nil {
		n.Valid = false
		return nil
	}

	n.Bytes, n.Valid = value.([]byte)
	if !n.Valid {
		return fmt.Errorf("expected []byte but got %T", value)
	}
	return nil
}

// Value implements the driver Valuer interface.
func (n NullBytes) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Bytes, nil
}

// NullNull represents an always-null value. NullNull implements the
// sql.Scanner interface so it can be used as a scan destination, similar to
// sql.NullString.
type NullNull struct {
	Valid bool // Valid is true if Time is not NULL
}

// Scan implements the Scanner interface.
func (nn *NullNull) Scan(value interface{}) error {
	if value != nil {
		return errors.New("value is not null")
	}
	return nil
}

// Value implements the driver Valuer interface.
func (nn NullNull) Value() (driver.Value, error) {
	return nil, nil
}

// ExtractValue "unwraps" a field returned from the sql Rows.Scan() method. If
// val implements driver.Valuer, the underlying Value() is returned. Otherwise,
// val is returned.
func ExtractValue(val interface{}) interface{} {

	if valuer, ok := val.(driver.Valuer); ok {
		val, _ = valuer.Value()
	}

	return val
}

// ExtractValues is a convenience function that returns an array for which each
// element is the output of invoking ExtractValue() on the corresponding input element.
func ExtractValues(vals []interface{}) []interface{} {
	// TODO: do we need this? Do we need to create a new slice, or should
	// we just operate on the existing slice?
	vs := make([]interface{}, len(vals))

	for i := range vals {
		vs[i] = ExtractValue(vals[i])
	}
	return vs
}

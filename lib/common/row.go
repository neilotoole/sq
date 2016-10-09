package common

import (
	"reflect"

	"strconv"

	"fmt"
	"strings"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq-driver/hackery/database/sql/driver"
	"github.com/neilotoole/sq/lib/common/orderedmap"
	"github.com/neilotoole/sq/lib/util"
)

type ResultRow struct {
	Values []interface{}
	Fields []driver.ColumnType

	// fieldValMap is a mapping of field/alias names to value index
	fieldValMap map[string]int
}

func (r *ResultRow) ToOrderedMap() *orderedmap.Map {

	m := &orderedmap.Map{}
	for i := 0; i < len(r.Values); i++ {

		// TODO: OrderedMap currently allows duplicate keys, need to address this
		if r.Values[i] == nil {
			m.Put(r.Fields[i].Name, nil)
			//m = append(m, KeyVal{Key: r.Fields[i].Name, Val: nil})
			continue
		}

		if value, ok := r.Values[i].(driver.Valuer); ok {
			val, _ := value.Value()
			m.Put(r.Fields[i].Name, val)
			//m = append(m, KeyVal{Key: r.Fields[i].Name, Val: val})
			continue
		}

		m.Put(r.Fields[i].Name, r.Values[i])

		//m = append(m, KeyVal{Key: r.Fields[i].Name, Val: r.Values[i]})
	}
	return m
}

// String returns a string representation of the ResultRow. Note that this method
// is computationally expensive, it should generally only be used for debugging.
func (r *ResultRow) String() string {

	elements := make([]string, len(r.Fields))

	for i, field := range r.Fields {

		name := strconv.Itoa(i)

		fieldName := field.Name
		if field.AliasedName != "" {
			if fieldName == "" {
				fieldName = field.AliasedName
			} else {
				fieldName = fieldName + "|" + field.AliasedName
			}
		}
		if fieldName != "" {
			name = name + " " + fieldName
		}

		if r.Values[i] == nil {
			elements[i] = fmt.Sprintf("{%s: nil}", name)
		} else {

			valuer, ok := r.Values[i].(driver.Valuer)
			if ok {
				v, err := valuer.Value()
				if err != nil {
					lg.Warnf("falling through to default output format: unable to get valuer.Value(): %v", err)
					// fall through to default output below
				} else {
					elements[i] = fmt.Sprintf("{%s: %v}", name, v)
					continue
				}
			}

			elements[i] = fmt.Sprintf("{%s: %s}", name, r.Values[i])
		}
	}

	return "[" + strings.Join(elements, ", ") + "]"
}

// Types returns the types of the values.
func (r *ResultRow) Types() []reflect.Type {

	types := make([]reflect.Type, len(r.Values))

	for i, val := range r.Values {
		typ := reflect.TypeOf(val)
		types[i] = typ
	}
	return types
}

// NameIndices returns a map of col name/alias to its index in the Values array.
func (r *ResultRow) NameIndices() map[string]int {
	return r.fieldValMap
}

// NamedValue returns the scanned value for a column (or alias) of the given name,
// unwrapping any sql.Valuer if possible. If errIfNil is true, an error is returned
// if the (unwrapped) value is nil. An error is also returned if there is no such
// named value.
func (r *ResultRow) NamedValue(name string, errIfNil bool) (interface{}, error) {

	index, ok := r.fieldValMap[name]
	if !ok {
		return nil, util.Errorf("named value %q not found", name)
	}

	valuer, ok := r.Values[index].(driver.Valuer)
	if ok {
		val, err := valuer.Value()
		if err != nil {
			return nil, util.WrapError(err)
		}

		if errIfNil && val == nil {
			return nil, util.Errorf("named value %q is nil", name)
		}

		return val, nil
	}

	if errIfNil && r.Values[index] == nil {
		return nil, util.Errorf("named value %q is nil", name)
	}

	return r.Values[index], nil
}

// TODO: get rid of error in return
func NewResultRow(fields []driver.ColumnType) *ResultRow {

	r := &ResultRow{}
	r.fieldValMap = make(map[string]int)
	dest := make([]interface{}, len(fields))

	for i, field := range fields {

		//lg.Debugf("handling field: %v", field)
		if field.Name == "" {
			lg.Warnf("field [%d] has empty name")
		} else {
			r.fieldValMap[field.Name] = i
		}

		if field.AliasedName != "" {
			_, ok := r.fieldValMap[field.AliasedName]
			if ok {
				lg.Warnf("alias %q for field %q already exists for val index [%d]", field.AliasedName, field.Name, i)
			}

			r.fieldValMap[field.AliasedName] = i
		}

		switch field.FieldType {
		case driver.FieldTypeNULL:
			dest[i] = nil
		case
			driver.FieldTypeTiny,
			driver.FieldTypeShort,
			driver.FieldTypeYear,
			driver.FieldTypeInt24,
			driver.FieldTypeLong,
			driver.FieldTypeLongLong:
			dest[i] = new(sql.NullInt64)
		case
			driver.FieldTypeFloat,
			driver.FieldTypeDouble:
			dest[i] = new(sql.NullFloat64)

		case
			driver.FieldTypeTinyBLOB,
			driver.FieldTypeMediumBLOB,
			driver.FieldTypeLongBLOB,
			driver.FieldTypeBLOB:
			if field.Flags.IsSet(driver.FlagBinary) {
				dest[i] = &[]byte{}
			} else {
				dest[i] = new(sql.NullString)
			}
		case
			// TODO: Need to figure out time/timestamp support instead of using String for time values
			driver.FieldTypeDate,
			driver.FieldTypeNewDate, // Date YYYY-MM-DD
			driver.FieldTypeTime,    // Time [-][H]HH:MM:SS[.fractal]
			driver.FieldTypeTimestamp,
			driver.FieldTypeDateTime: // Timestamp YYYY-MM-DD HH:MM:SS[.fractal]
			dest[i] = new(sql.NullString)

		case
			driver.FieldTypeDecimal,
			driver.FieldTypeNewDecimal,
			driver.FieldTypeVarChar,
			driver.FieldTypeBit,
			driver.FieldTypeEnum,
			driver.FieldTypeSet,
			driver.FieldTypeVarString,
			driver.FieldTypeString,
			driver.FieldTypeGeometry,
			driver.FieldTypeJSON:
			dest[i] = new(sql.NullString)
		// Please report if this happens!
		default:
			// TODO: change this to warn
			lg.Warnf("unexpected field type %q, treating as text", field.FieldType)
			dest[i] = new(sql.NullString)
		}
	}

	r.Values = dest
	r.Fields = fields

	return r

}

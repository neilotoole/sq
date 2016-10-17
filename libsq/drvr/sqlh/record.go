package sqlh

import (
	"reflect"

	"strconv"

	"fmt"
	"strings"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql/driver"
	"github.com/neilotoole/sq/libsq/util"
)

type Record struct {
	Values []interface{}
	Fields []driver.ColumnType

	// fieldValMap is a mapping of field/alias names to value index
	fieldValMap map[string]int
}

// String returns a string representation of the ResultRow. Note that this method
// is computationally expensive, it should generally only be used for debugging.
func (r *Record) String() string {

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

// ReflectTypes returns the types of the values.
func (r *Record) ReflectTypes() []reflect.Type {

	types := make([]reflect.Type, len(r.Values))

	for i, val := range r.Values {
		typ := reflect.TypeOf(val)
		types[i] = typ
	}
	return types
}

// NameIndices returns a map of col name/alias to its index in the Values array.
func (r *Record) NameIndices() map[string]int {
	return r.fieldValMap
}

// NamedValue returns the scanned value for a column (or alias) of the given name,
// unwrapping any sql.Valuer if possible. If errIfNil is true, an error is returned
// if the (unwrapped) value is nil. An error is also returned if there is no such
// named value.
func (r *Record) NamedValue(name string, errIfNil bool) (interface{}, error) {

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

func NewRecord(fields []driver.ColumnType) (*Record, error) {

	r := &Record{}
	r.fieldValMap = make(map[string]int)

	dataTypes, err := DataTypeFromCols(fields)
	if err != nil {
		return nil, err
	}

	// TODO: ScanDests should be a function on the driver?
	// TODO: Or it will come from sql.ColumnType() eventually
	dest := ScanDests(dataTypes)

	for i, field := range fields {

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
	}

	r.Values = dest
	r.Fields = fields

	return r, nil

}

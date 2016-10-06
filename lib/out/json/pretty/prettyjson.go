// Package out provides JSON pretty print.
package pretty

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/common/orderedmap"
)

// Formatter is a struct to format JSON data. `color` is github.com/fatih/color: https://github.com/fatih/color
type Formatter struct {
	// JSON key color. Default is `color.New(color.FgBlue, color.Bold)`.
	KeyColor *color.Color

	// JSON string value color. Default is `color.New(color.FgGreen, color.Bold)`.
	StringColor *color.Color

	// Binary data is output as a JSON string.
	BinaryColor *color.Color

	// JSON boolean value color. Default is `color.New(color.FgYellow, color.Bold)`.
	BoolColor *color.Color

	// JSON number value color. Default is `color.New(color.FgCyan, color.Bold)`.
	NumberColor *color.Color

	// JSON null value color. Default is `color.New(color.FgBlack, color.Bold)`.
	NullColor *color.Color

	// Max length of JSON string value. When the value is 1 and over, string is truncated to length of the value. Default is 0 (not truncated).
	StringMaxLength int

	// Boolean to disable color. Default is false.
	DisabledColor bool

	// Indent space number. Default is 2.
	Indent int
}

// NewFormatter returns a new formatter with following default values.
func NewFormatter() *Formatter {
	return &Formatter{
		KeyColor:        color.New(color.FgBlue, color.Bold),
		StringColor:     color.New(color.FgGreen, color.Bold),
		BinaryColor:     color.New(color.FgCyan),
		BoolColor:       color.New(color.FgYellow, color.Bold),
		NumberColor:     color.New(color.FgCyan, color.Bold),
		NullColor:       color.New(color.FgBlack, color.Bold),
		StringMaxLength: 0,
		DisabledColor:   false,
		Indent:          2,
	}
}

// Marshals and formats JSON data.
func (f *Formatter) Marshal(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)

	if err != nil {
		return nil, err
	}

	return f.Format(data)
}

// Formats JSON string.
func (f *Formatter) Format(data []byte) ([]byte, error) {
	var v interface{}
	err := json.Unmarshal(data, &v)

	if err != nil {
		return nil, err
	}

	s := f.pretty(v, 1)

	return []byte(s), nil
}

func (f *Formatter) FormatRows(rows []*common.ResultRow) ([]byte, error) {

	items := make([]interface{}, len(rows))

	for i, row := range rows {
		items[i] = row.ToOrderedMap()
	}

	s := f.pretty(items, 1)

	return []byte(s), nil

}

func (f *Formatter) FormatOrderedMap(m orderedmap.Map) ([]byte, error) {
	//var v interface{}
	//err := json.Unmarshal(data, &v)
	//
	//if err != nil {
	//	return nil, err
	//}

	s := f.pretty(m, 1)

	return []byte(s), nil
}

func (f *Formatter) SprintfColor(c *color.Color, format string, args ...interface{}) string {
	if f.DisabledColor || c == nil {
		return fmt.Sprintf(format, args...)
	} else {
		return c.SprintfFunc()(format, args...)
	}
}

func (f *Formatter) pretty(v interface{}, depth int) string {
	switch val := v.(type) {
	case string:
		return f.processString(val)
	case *string:
		return f.processString(*val)
	case float64:
		return f.SprintfColor(f.NumberColor, strconv.FormatFloat(val, 'f', -1, 64))
	case *float64:
		return f.SprintfColor(f.NumberColor, strconv.FormatFloat(*val, 'f', -1, 64))
	case float32:
		return f.SprintfColor(f.NumberColor, strconv.FormatFloat(float64(val), 'f', -1, 32))
	case *float32:
		return f.SprintfColor(f.NumberColor, strconv.FormatFloat(float64(*val), 'f', -1, 32))
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return f.SprintfColor(f.NumberColor, fmt.Sprintf("%d", val))
	case *int:
		return f.SprintfColor(f.NumberColor, fmt.Sprintf("%d", *val))
	case *int8:
		return f.SprintfColor(f.NumberColor, fmt.Sprintf("%d", *val))
	case *int16:
		return f.SprintfColor(f.NumberColor, fmt.Sprintf("%d", *val))
	case *int32:
		return f.SprintfColor(f.NumberColor, fmt.Sprintf("%d", *val))
	case *int64:
		return f.SprintfColor(f.NumberColor, fmt.Sprintf("%d", *val))
	case *uint:
		return f.SprintfColor(f.NumberColor, fmt.Sprintf("%d", *val))
	case *uint8:
		return f.SprintfColor(f.NumberColor, fmt.Sprintf("%d", *val))
	case *uint16:
		return f.SprintfColor(f.NumberColor, fmt.Sprintf("%d", *val))
	case *uint32:
		return f.SprintfColor(f.NumberColor, fmt.Sprintf("%d", *val))
	case *uint64:
		return f.SprintfColor(f.NumberColor, fmt.Sprintf("%d", *val))
	case bool:
		return f.SprintfColor(f.BoolColor, strconv.FormatBool(val))
	case *bool:
		return f.SprintfColor(f.BoolColor, strconv.FormatBool(*val))
	case nil:
		return f.SprintfColor(f.NullColor, "null")
	case orderedmap.Map:
		return f.processOrderedMap(&val, depth)
	case *orderedmap.Map:
		return f.processOrderedMap(val, depth)
	case map[string]interface{}:
		return f.processMap(val, depth)
	case []byte:
		return f.processBinary(val)
	case *[]byte:
		if val == nil {
			return f.SprintfColor(f.NullColor, "null")
		}
		return f.processBinary(*val)
	case []interface{}:
		return f.processArray(val, depth)
	}

	return ""
}

func (f *Formatter) processString(s string) string {
	r := []rune(s)

	if f.StringMaxLength != 0 && len(r) >= f.StringMaxLength {
		s = string(r[0:f.StringMaxLength]) + "..."
	}

	b, _ := json.Marshal(s)

	return f.SprintfColor(f.StringColor, string(b))
}

func (f *Formatter) processBinary(data []byte) string {

	b, _ := json.Marshal(data)

	return f.SprintfColor(f.BinaryColor, string(b))
}

func (f *Formatter) processMap(m map[string]interface{}, depth int) string {
	currentIndent := f.generateIndent(depth - 1)
	nextIndent := f.generateIndent(depth)
	rows := []string{}
	keys := []string{}

	if len(m) == 0 {
		return "{}"
	}

	for key, _ := range m {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		val := m[key]
		k := f.SprintfColor(f.KeyColor, `"%s"`, key)
		v := f.pretty(val, depth+1)
		row := fmt.Sprintf("%s%s: %s", nextIndent, k, v)
		rows = append(rows, row)
	}

	return fmt.Sprintf("{\n%s\n%s}", strings.Join(rows, ",\n"), currentIndent)
}

func (f *Formatter) processOrderedMap(m *orderedmap.Map, depth int) string {
	currentIndent := f.generateIndent(depth - 1)
	nextIndent := f.generateIndent(depth)
	rows := []string{}
	//keys := []string{}

	if m.Len() == 0 {
		return "{}"
	}

	//for key, _ := range m {
	//	keys = append(keys, key)
	//}
	//
	//sort.Strings(keys)

	for _, kv := range m.Items() {
		//val := kv.Val
		k := f.SprintfColor(f.KeyColor, `"%s"`, kv.Key)

		//rv := reflect.ValueOf(kv.Val)
		//rt := reflect.TypeOf(kv.Val)
		////elem := rv.Elem()
		//
		//log.Printf("val: %T(%v)", kv.Val, kv.Val)
		//log.Printf("rv: %T(%v)", rv, rv)
		//log.Printf("rt: %T(%v)", rt, rt)
		////log.Printf("elem: %T(%v)", elem, elem)
		//log.Println()
		//
		//switch val := kv.Val.(type) {
		//default:
		//	v := f.pretty(val, depth+1)
		//	row := fmt.Sprintf("%s%s: %s", nextIndent, k, v)
		//	rows = append(rows, row)
		//}
		v := f.pretty(kv.Val, depth+1)

		row := fmt.Sprintf("%s%s: %s", nextIndent, k, v)
		rows = append(rows, row)
	}

	//for _, key := range keys {
	//	val := m[key]
	//	k := f.sprintfColor(f.KeyColor, `"%s"`, key)
	//	v := f.Pretty(val, depth+1)
	//	row := fmt.Sprintf("%s%s: %s", nextIndent, k, v)
	//	rows = append(rows, row)
	//}

	return fmt.Sprintf("{\n%s\n%s}", strings.Join(rows, ",\n"), currentIndent)
}

func (f *Formatter) processArray(a []interface{}, depth int) string {
	currentIndent := f.generateIndent(depth - 1)
	nextIndent := f.generateIndent(depth)
	rows := []string{}

	if len(a) == 0 {
		return "[]"
	}

	for _, val := range a {
		c := f.pretty(val, depth+1)
		row := nextIndent + c
		rows = append(rows, row)
	}

	return fmt.Sprintf("[\n%s\n%s]", strings.Join(rows, ",\n"), currentIndent)
}

func (f *Formatter) generateIndent(depth int) string {
	return strings.Join(make([]string, f.Indent*depth+1), " ")
}

// Marshal JSON data with default options.
func Marshal(v interface{}) ([]byte, error) {
	return NewFormatter().Marshal(v)
}

// Format JSON string with default options.
func Format(data []byte) ([]byte, error) {
	return NewFormatter().Format(data)
}

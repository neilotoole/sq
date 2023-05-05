//nolint:lll
package internal_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	stdjson "encoding/json"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw/internal"
	jcolorenc "github.com/neilotoole/sq/cli/output/jsonw/internal/jcolorenc"
)

// Encoder encapsulates the methods of a JSON encoder.
type Encoder interface {
	Encode(v any) error
	SetEscapeHTML(on bool)
	SetIndent(prefix, indent string)
}

var _ Encoder = (*jcolorenc.Encoder)(nil)

func TestEncode(t *testing.T) {
	testCases := []struct {
		name    string
		pretty  bool
		color   bool
		sortMap bool
		v       any
		want    string
	}{
		{
			name:   "nil",
			pretty: true,
			v:      nil,
			want:   "null\n",
		},
		{
			name:   "slice_empty",
			pretty: true,
			v:      []int{},
			want:   "[]\n",
		},
		{
			name:   "slice_1_pretty",
			pretty: true,
			v:      []any{1},
			want:   "[\n  1\n]\n",
		},
		{
			name: "slice_1_no_pretty",
			v:    []any{1},
			want: "[1]\n",
		},
		{
			name:   "slice_2_pretty",
			pretty: true,
			v:      []any{1, true},
			want:   "[\n  1,\n  true\n]\n",
		},
		{
			name: "slice_2_no_pretty",
			v:    []any{1, true},
			want: "[1,true]\n",
		},
		{
			name:   "map_int_empty",
			pretty: true,
			v:      map[string]int{},
			want:   "{}\n",
		},
		{
			name:   "map_interface_empty",
			pretty: true,
			v:      map[string]any{},
			want:   "{}\n",
		},
		{
			name:    "map_interface_empty_sorted",
			pretty:  true,
			sortMap: true,
			v:       map[string]any{},
			want:    "{}\n",
		},
		{
			name:    "map_1_pretty",
			pretty:  true,
			sortMap: true,
			v:       map[string]any{"one": 1},
			want:    "{\n  \"one\": 1\n}\n",
		},
		{
			name:    "map_1_no_pretty",
			sortMap: true,
			v:       map[string]any{"one": 1},
			want:    "{\"one\":1}\n",
		},
		{
			name:    "map_2_pretty",
			pretty:  true,
			sortMap: true,
			v:       map[string]any{"one": 1, "two": 2},
			want:    "{\n  \"one\": 1,\n  \"two\": 2\n}\n",
		},
		{
			name:    "map_2_no_pretty",
			sortMap: true,
			v:       map[string]any{"one": 1, "two": 2},
			want:    "{\"one\":1,\"two\":2}\n",
		},
		{
			name:   "tinystruct",
			pretty: true,
			v:      TinyStruct{FBool: true},
			want:   "{\n  \"f_bool\": true\n}\n",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			pr := output.NewPrinting()
			pr.Compact = !tc.pretty
			pr.EnableColor(tc.color)

			buf := &bytes.Buffer{}
			enc := jcolorenc.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			enc.SetSortMapKeys(tc.sortMap)
			enc.SetColors(internal.NewColors(pr))

			if !pr.Compact {
				enc.SetIndent("", pr.Indent)
			}

			require.NoError(t, enc.Encode(tc.v))
			require.True(t, stdjson.Valid(buf.Bytes()))
			require.Equal(t, tc.want, buf.String())
		})
	}
}

func TestEncode_Slice(t *testing.T) {
	testCases := []struct {
		name   string
		pretty bool
		color  bool
		v      []any
		want   string
	}{
		{
			name:   "nil",
			pretty: true,
			v:      nil,
			want:   "null\n",
		},
		{
			name:   "empty",
			pretty: true,
			v:      []any{},
			want:   "[]\n",
		},
		{
			name:   "one",
			pretty: true,
			v:      []any{1},
			want:   "[\n  1\n]\n",
		},
		{
			name:   "two",
			pretty: true,
			v:      []any{1, true},
			want:   "[\n  1,\n  true\n]\n",
		},
		{
			name:   "three",
			pretty: true,
			v:      []any{1, true, "hello"},
			want:   "[\n  1,\n  true,\n  \"hello\"\n]\n",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			pr := output.NewPrinting()
			pr.Compact = !tc.pretty
			pr.EnableColor(tc.color)

			buf := &bytes.Buffer{}
			enc := jcolorenc.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			enc.SetColors(internal.NewColors(pr))
			if !pr.Compact {
				enc.SetIndent("", "  ")
			}

			require.NoError(t, enc.Encode(tc.v))
			require.True(t, stdjson.Valid(buf.Bytes()))
			require.Equal(t, tc.want, buf.String())
		})
	}
}

func TestEncode_SmallStruct(t *testing.T) {
	v := SmallStruct{
		FInt:   7,
		FSlice: []any{64, true},
		FMap: map[string]any{
			"m_float64": 64.64,
			"m_string":  "hello",
		},
		FTinyStruct: TinyStruct{FBool: true},
		FString:     "hello",
	}

	testCases := []struct {
		pretty bool
		color  bool
		want   string
	}{
		{
			pretty: false,
			color:  false,
			want:   "{\"f_int\":7,\"f_slice\":[64,true],\"f_map\":{\"m_float64\":64.64,\"m_string\":\"hello\"},\"f_tinystruct\":{\"f_bool\":true},\"f_string\":\"hello\"}\n",
		},
		{
			pretty: true,
			color:  false,
			want:   "{\n  \"f_int\": 7,\n  \"f_slice\": [\n    64,\n    true\n  ],\n  \"f_map\": {\n    \"m_float64\": 64.64,\n    \"m_string\": \"hello\"\n  },\n  \"f_tinystruct\": {\n    \"f_bool\": true\n  },\n  \"f_string\": \"hello\"\n}\n",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(fmt.Sprintf("pretty_%v__color_%v", tc.pretty, tc.color), func(t *testing.T) {
			pr := output.NewPrinting()
			pr.Compact = !tc.pretty
			pr.EnableColor(tc.color)

			buf := &bytes.Buffer{}
			enc := jcolorenc.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			enc.SetSortMapKeys(true)
			enc.SetColors(internal.NewColors(pr))

			if !pr.Compact {
				enc.SetIndent("", "  ")
			}

			require.NoError(t, enc.Encode(v))
			require.True(t, stdjson.Valid(buf.Bytes()))
			require.Equal(t, tc.want, buf.String())
		})
	}
}

func TestEncode_Map_Nested(t *testing.T) {
	v := map[string]any{
		"m_bool1": true,
		"m_nest1": map[string]any{
			"m_nest1_bool": true,
			"m_nest2": map[string]any{
				"m_nest2_bool": true,
				"m_nest3": map[string]any{
					"m_nest3_bool": true,
				},
			},
		},
		"m_string1": "hello",
	}

	testCases := []struct {
		pretty bool
		color  bool
		want   string
	}{
		{
			pretty: false,
			want:   "{\"m_bool1\":true,\"m_nest1\":{\"m_nest1_bool\":true,\"m_nest2\":{\"m_nest2_bool\":true,\"m_nest3\":{\"m_nest3_bool\":true}}},\"m_string1\":\"hello\"}\n",
		},
		{
			pretty: true,
			want:   "{\n  \"m_bool1\": true,\n  \"m_nest1\": {\n    \"m_nest1_bool\": true,\n    \"m_nest2\": {\n      \"m_nest2_bool\": true,\n      \"m_nest3\": {\n        \"m_nest3_bool\": true\n      }\n    }\n  },\n  \"m_string1\": \"hello\"\n}\n",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(fmt.Sprintf("pretty_%v__color_%v", tc.pretty, tc.color), func(t *testing.T) {
			pr := output.NewPrinting()
			pr.Compact = !tc.pretty
			pr.EnableColor(tc.color)

			buf := &bytes.Buffer{}
			enc := jcolorenc.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			enc.SetSortMapKeys(true)
			enc.SetColors(internal.NewColors(pr))

			if !pr.Compact {
				enc.SetIndent("", "  ")
			}

			require.NoError(t, enc.Encode(v))
			require.True(t, stdjson.Valid(buf.Bytes()))
			require.Equal(t, tc.want, buf.String())
		})
	}
}

// TestEncode_Map_StringNotInterface tests maps with a string key
// but the value type is not any.
// For example, map[string]bool. This test is necessary because the
// encoder has a fast path for map[string]any.
func TestEncode_Map_StringNotInterface(t *testing.T) {
	testCases := []struct {
		pretty  bool
		color   bool
		sortMap bool
		v       map[string]bool
		want    string
	}{
		{
			pretty:  false,
			sortMap: true,
			v:       map[string]bool{},
			want:    "{}\n",
		},
		{
			pretty:  false,
			sortMap: false,
			v:       map[string]bool{},
			want:    "{}\n",
		},
		{
			pretty:  true,
			sortMap: true,
			v:       map[string]bool{},
			want:    "{}\n",
		},
		{
			pretty:  true,
			sortMap: false,
			v:       map[string]bool{},
			want:    "{}\n",
		},
		{
			pretty:  false,
			sortMap: true,
			v:       map[string]bool{"one": true},
			want:    "{\"one\":true}\n",
		},
		{
			pretty:  false,
			sortMap: false,
			v:       map[string]bool{"one": true},
			want:    "{\"one\":true}\n",
		},
		{
			pretty:  false,
			sortMap: true,
			v:       map[string]bool{"one": true, "two": false},
			want:    "{\"one\":true,\"two\":false}\n",
		},
		{
			pretty:  true,
			sortMap: true,
			v:       map[string]bool{"one": true, "two": false},
			want:    "{\n  \"one\": true,\n  \"two\": false\n}\n",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(fmt.Sprintf("size_%d__pretty_%v__color_%v", len(tc.v), tc.pretty, tc.color), func(t *testing.T) {
			pr := output.NewPrinting()
			pr.Compact = !tc.pretty
			pr.EnableColor(tc.color)

			buf := &bytes.Buffer{}
			enc := jcolorenc.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			enc.SetSortMapKeys(tc.sortMap)
			enc.SetColors(internal.NewColors(pr))
			if !pr.Compact {
				enc.SetIndent("", pr.Indent)
			}

			require.NoError(t, enc.Encode(tc.v))
			require.True(t, stdjson.Valid(buf.Bytes()))
			require.Equal(t, tc.want, buf.String())
		})
	}
}

func TestEncode_RawMessage(t *testing.T) {
	type RawStruct struct {
		FString string               `json:"f_string"`
		FRaw    jcolorenc.RawMessage `json:"f_raw"`
	}

	raw := jcolorenc.RawMessage(`{"one":1,"two":2}`)

	testCases := []struct {
		name   string
		pretty bool
		color  bool
		v      any
		want   string
	}{
		{
			name:   "empty",
			pretty: false,
			v:      jcolorenc.RawMessage(`{}`),
			want:   "{}\n",
		},
		{
			name:   "no_pretty",
			pretty: false,
			v:      raw,
			want:   "{\"one\":1,\"two\":2}\n",
		},
		{
			name:   "pretty",
			pretty: true,
			v:      raw,
			want:   "{\n  \"one\": 1,\n  \"two\": 2\n}\n",
		},
		{
			name:   "pretty_struct",
			pretty: true,
			v:      RawStruct{FString: "hello", FRaw: raw},
			want:   "{\n  \"f_string\": \"hello\",\n  \"f_raw\": {\n    \"one\": 1,\n    \"two\": 2\n  }\n}\n",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			pr := output.NewPrinting()
			pr.Compact = !tc.pretty
			pr.EnableColor(tc.color)

			buf := &bytes.Buffer{}
			enc := jcolorenc.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			enc.SetSortMapKeys(true)
			enc.SetColors(internal.NewColors(pr))
			if !pr.Compact {
				enc.SetIndent("", pr.Indent)
			}

			err := enc.Encode(tc.v)
			require.NoError(t, err)
			require.True(t, stdjson.Valid(buf.Bytes()))
			require.Equal(t, tc.want, buf.String())
		})
	}
}

// TestEncode_Map_StringNotInterface tests map[string]json.RawMessage.
// This test is necessary because the encoder has a fast path
// for map[string]any.
func TestEncode_Map_StringRawMessage(t *testing.T) {
	raw := jcolorenc.RawMessage(`{"one":1,"two":2}`)

	testCases := []struct {
		pretty  bool
		color   bool
		sortMap bool
		v       map[string]jcolorenc.RawMessage
		want    string
	}{
		{
			pretty:  false,
			sortMap: true,
			v:       map[string]jcolorenc.RawMessage{},
			want:    "{}\n",
		},
		{
			pretty:  false,
			sortMap: false,
			v:       map[string]jcolorenc.RawMessage{},
			want:    "{}\n",
		},
		{
			pretty:  true,
			sortMap: true,
			v:       map[string]jcolorenc.RawMessage{},
			want:    "{}\n",
		},
		{
			pretty:  true,
			sortMap: false,
			v:       map[string]jcolorenc.RawMessage{},
			want:    "{}\n",
		},
		{
			pretty:  false,
			sortMap: true,
			v:       map[string]jcolorenc.RawMessage{"msg1": raw, "msg2": raw},
			want:    "{\"msg1\":{\"one\":1,\"two\":2},\"msg2\":{\"one\":1,\"two\":2}}\n",
		},
		{
			pretty:  true,
			sortMap: true,
			v:       map[string]jcolorenc.RawMessage{"msg1": raw, "msg2": raw},
			want:    "{\n  \"msg1\": {\n    \"one\": 1,\n    \"two\": 2\n  },\n  \"msg2\": {\n    \"one\": 1,\n    \"two\": 2\n  }\n}\n",
		},
		{
			pretty:  true,
			sortMap: false,
			v:       map[string]jcolorenc.RawMessage{"msg1": raw},
			want:    "{\n  \"msg1\": {\n    \"one\": 1,\n    \"two\": 2\n  }\n}\n",
		},
	}

	for _, tc := range testCases {
		tc := tc

		name := fmt.Sprintf("size_%d__pretty_%v__color_%v__sort_%v", len(tc.v), tc.pretty, tc.color, tc.sortMap)
		t.Run(name, func(t *testing.T) {
			pr := output.NewPrinting()
			pr.Compact = !tc.pretty
			pr.EnableColor(tc.color)

			buf := &bytes.Buffer{}
			enc := jcolorenc.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			enc.SetSortMapKeys(tc.sortMap)
			enc.SetColors(internal.NewColors(pr))
			if !pr.Compact {
				enc.SetIndent("", pr.Indent)
			}

			require.NoError(t, enc.Encode(tc.v))
			require.True(t, stdjson.Valid(buf.Bytes()))
			require.Equal(t, tc.want, buf.String())
		})
	}
}

func TestEncode_BigStruct(t *testing.T) {
	v := newBigStruct()

	testCases := []struct {
		pretty bool
		color  bool
		want   string
	}{
		{
			pretty: false,
			want:   "{\"f_int\":-7,\"f_int8\":-8,\"f_int16\":-16,\"f_int32\":-32,\"f_int64\":-64,\"f_uint\":7,\"f_uint8\":8,\"f_uint16\":16,\"f_uint32\":32,\"f_uint64\":64,\"f_float32\":32.32,\"f_float64\":64.64,\"f_bool\":true,\"f_bytes\":\"aGVsbG8=\",\"f_nil\":null,\"f_string\":\"hello\",\"f_map\":{\"m_bool\":true,\"m_int64\":64,\"m_nil\":null,\"m_smallstruct\":{\"f_int\":7,\"f_slice\":[64,true],\"f_map\":{\"m_float64\":64.64,\"m_string\":\"hello\"},\"f_tinystruct\":{\"f_bool\":true},\"f_string\":\"hello\"},\"m_string\":\"hello\"},\"f_smallstruct\":{\"f_int\":7,\"f_slice\":[64,true],\"f_map\":{\"m_float64\":64.64,\"m_string\":\"hello\"},\"f_tinystruct\":{\"f_bool\":true},\"f_string\":\"hello\"},\"f_interface\":\"hello\",\"f_interfaces\":[64,\"hello\",true]}\n",
		},
		{
			pretty: true,
			want:   "{\n  \"f_int\": -7,\n  \"f_int8\": -8,\n  \"f_int16\": -16,\n  \"f_int32\": -32,\n  \"f_int64\": -64,\n  \"f_uint\": 7,\n  \"f_uint8\": 8,\n  \"f_uint16\": 16,\n  \"f_uint32\": 32,\n  \"f_uint64\": 64,\n  \"f_float32\": 32.32,\n  \"f_float64\": 64.64,\n  \"f_bool\": true,\n  \"f_bytes\": \"aGVsbG8=\",\n  \"f_nil\": null,\n  \"f_string\": \"hello\",\n  \"f_map\": {\n    \"m_bool\": true,\n    \"m_int64\": 64,\n    \"m_nil\": null,\n    \"m_smallstruct\": {\n      \"f_int\": 7,\n      \"f_slice\": [\n        64,\n        true\n      ],\n      \"f_map\": {\n        \"m_float64\": 64.64,\n        \"m_string\": \"hello\"\n      },\n      \"f_tinystruct\": {\n        \"f_bool\": true\n      },\n      \"f_string\": \"hello\"\n    },\n    \"m_string\": \"hello\"\n  },\n  \"f_smallstruct\": {\n    \"f_int\": 7,\n    \"f_slice\": [\n      64,\n      true\n    ],\n    \"f_map\": {\n      \"m_float64\": 64.64,\n      \"m_string\": \"hello\"\n    },\n    \"f_tinystruct\": {\n      \"f_bool\": true\n    },\n    \"f_string\": \"hello\"\n  },\n  \"f_interface\": \"hello\",\n  \"f_interfaces\": [\n    64,\n    \"hello\",\n    true\n  ]\n}\n",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(fmt.Sprintf("pretty_%v__color_%v", tc.pretty, tc.color), func(t *testing.T) {
			pr := output.NewPrinting()
			pr.Compact = !tc.pretty
			pr.EnableColor(tc.color)

			buf := &bytes.Buffer{}
			enc := jcolorenc.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			enc.SetSortMapKeys(true)
			enc.SetColors(internal.NewColors(pr))

			if !pr.Compact {
				enc.SetIndent("", "  ")
			}

			require.NoError(t, enc.Encode(v))
			require.True(t, stdjson.Valid(buf.Bytes()))
			require.Equal(t, tc.want, buf.String())
		})
	}
}

// TestEncode_Map_Not_StringInterface tests map encoding where
// the map is not map[string]any (for which the encoder
// has a fast path).
//
// NOTE: Currently the encoder is broken wrt colors enabled
// for non-string map keys. It's possible we don't actually need
// to address this for sq purposes.
func TestEncode_Map_Not_StringInterface(t *testing.T) {
	pr := output.NewPrinting()
	pr.Compact = false
	pr.EnableColor(true)

	buf := &bytes.Buffer{}
	enc := jcolorenc.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetSortMapKeys(true)
	enc.SetColors(internal.NewColors(pr))
	if !pr.Compact {
		enc.SetIndent("", "  ")
	}

	v := map[int32]string{
		0: "zero",
		1: "one",
		2: "two",
	}

	require.NoError(t, enc.Encode(v))
	require.False(t, stdjson.Valid(buf.Bytes()),
		"expected to be invalid JSON because the encoder currently doesn't handle maps with non-string keys")
}

// BigStruct is a big test struct.
type BigStruct struct {
	FInt         int            `json:"f_int"`
	FInt8        int8           `json:"f_int8"`
	FInt16       int16          `json:"f_int16"`
	FInt32       int32          `json:"f_int32"`
	FInt64       int64          `json:"f_int64"`
	FUint        uint           `json:"f_uint"`
	FUint8       uint8          `json:"f_uint8"`
	FUint16      uint16         `json:"f_uint16"`
	FUint32      uint32         `json:"f_uint32"`
	FUint64      uint64         `json:"f_uint64"`
	FFloat32     float32        `json:"f_float32"`
	FFloat64     float64        `json:"f_float64"`
	FBool        bool           `json:"f_bool"`
	FBytes       []byte         `json:"f_bytes"`
	FNil         any            `json:"f_nil"`
	FString      string         `json:"f_string"`
	FMap         map[string]any `json:"f_map"`
	FSmallStruct SmallStruct    `json:"f_smallstruct"`
	FInterface   any            `json:"f_interface"`
	FInterfaces  []any          `json:"f_interfaces"`
}

// SmallStruct is a small test struct.
type SmallStruct struct {
	FInt        int            `json:"f_int"`
	FSlice      []any          `json:"f_slice"`
	FMap        map[string]any `json:"f_map"`
	FTinyStruct TinyStruct     `json:"f_tinystruct"`
	FString     string         `json:"f_string"`
}

// Tiny Struct is a tiny test struct.
type TinyStruct struct {
	FBool bool `json:"f_bool"`
}

func newBigStruct() BigStruct {
	return BigStruct{
		FInt:     -7,
		FInt8:    -8,
		FInt16:   -16,
		FInt32:   -32,
		FInt64:   -64,
		FUint:    7,
		FUint8:   8,
		FUint16:  16,
		FUint32:  32,
		FUint64:  64,
		FFloat32: 32.32,
		FFloat64: 64.64,
		FBool:    true,
		FBytes:   []byte("hello"),
		FNil:     nil,
		FString:  "hello",
		FMap: map[string]any{
			"m_int64":       int64(64),
			"m_string":      "hello",
			"m_bool":        true,
			"m_nil":         nil,
			"m_smallstruct": newSmallStruct(),
		},
		FSmallStruct: newSmallStruct(),
		FInterface:   any("hello"),
		FInterfaces:  []any{int64(64), "hello", true},
	}
}

func newSmallStruct() SmallStruct {
	return SmallStruct{
		FInt:   7,
		FSlice: []any{64, true},
		FMap: map[string]any{
			"m_float64": 64.64,
			"m_string":  "hello",
		},
		FTinyStruct: TinyStruct{FBool: true},
		FString:     "hello",
	}
}

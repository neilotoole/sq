// Package vector provides the type descriptor and value type for vector
// columns (sqlite-vec vec0 and libSQL), along with formatting and decoders for
// their stored byte layouts.
package vector

import (
	"bytes"
	"math"
	"slices"
	"strconv"
)

// Elem is a vector element type.
type Elem string

// Vector element types.
const (
	F32  Elem = "f32"
	F64  Elem = "f64"
	F16  Elem = "f16"
	BF16 Elem = "bf16"
	I8   Elem = "i8"
	F8   Elem = "f8"
	Bit  Elem = "bit"
)

// Type describes a vector column.
//
//nolint:govet // Field order is the JSON/YAML output order.
type Type struct {
	Dim    int    `json:"dim" yaml:"dim"`
	Elem   Elem   `json:"elem" yaml:"elem"`
	Sparse bool   `json:"sparse,omitempty" yaml:"sparse,omitempty"`
	Metric string `json:"metric,omitempty" yaml:"metric,omitempty"`
}

// Vector is a decoded vector value.
type Vector struct {
	// Elem is the element type.
	Elem Elem

	// Values holds the decoded elements.
	Values []float64

	// Raw is the original stored bytes (or text), for raw output and copies.
	Raw []byte
}

// Dim returns the number of elements.
func (v Vector) Dim() int {
	return len(v.Values)
}

// Summary returns the short text form, such as "[384 × f32]".
func (v Vector) Summary() string {
	return "[" + strconv.Itoa(len(v.Values)) + " × " + string(v.Elem) + "]"
}

// AppendJSON appends the full text form, such as "[1,2,-3.5,0.25]", to dst.
// There are no spaces. JSON cannot carry NaN or infinity, so non-finite
// elements are written as null.
func (v Vector) AppendJSON(dst []byte) []byte {
	dst = append(dst, '[')
	for i, f := range v.Values {
		if i > 0 {
			dst = append(dst, ',')
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			dst = append(dst, "null"...)
			continue
		}
		dst = append(dst, FormatElem(v.Elem, f)...)
	}
	return append(dst, ']')
}

// String returns the same text as AppendJSON.
func (v Vector) String() string {
	return string(v.AppendJSON(nil))
}

// Equal reports whether v and o have the same element type, values and raw
// bytes.
func (v Vector) Equal(o Vector) bool {
	return v.Elem == o.Elem && slices.Equal(v.Values, o.Values) && bytes.Equal(v.Raw, o.Raw)
}

// Clone returns a deep copy of v.
func (v Vector) Clone() Vector {
	return Vector{
		Elem:   v.Elem,
		Values: slices.Clone(v.Values),
		Raw:    slices.Clone(v.Raw),
	}
}

// FormatElem formats f as the shortest text that round-trips to element type
// e. The i8 and bit types are formatted as integers.
func FormatElem(e Elem, f float64) string {
	switch e {
	case I8, Bit:
		return strconv.FormatInt(int64(f), 10)
	case F64:
		return strconv.FormatFloat(f, 'g', -1, 64)
	case F32, F16, BF16, F8:
		return strconv.FormatFloat(f, 'g', -1, 32)
	default:
		return strconv.FormatFloat(f, 'g', -1, 32)
	}
}

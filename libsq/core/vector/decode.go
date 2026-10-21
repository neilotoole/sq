package vector

import (
	"encoding/binary"
	"encoding/json"
	"math"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// libSQL trailing type tags, used when the blob length is odd.
const (
	tagF64  = 2
	tagBit  = 3
	tagF8   = 4
	tagF16  = 5
	tagBF16 = 6
)

// DecodeVec0 decodes a sqlite-vec vec0 value: f32 (4 bytes little-endian per
// element), i8 (one two's complement byte per element) or bit (8 elements per
// byte, least significant bit first). There is no header.
func DecodeVec0(b []byte, t Type) (Vector, error) {
	var vals []float64
	switch t.Elem {
	case F32:
		if len(b) != 4*t.Dim {
			return Vector{}, lenErr(b, t)
		}
		vals = decodeF32(b)
	case I8:
		if len(b) != t.Dim {
			return Vector{}, lenErr(b, t)
		}
		vals = make([]float64, len(b))
		for i, c := range b {
			vals[i] = float64(int(c) - int(c&0x80)*2)
		}
	case Bit:
		if len(b) != (t.Dim+7)/8 {
			return Vector{}, lenErr(b, t)
		}
		vals = decodeBits(b, t.Dim)
	case F64, F16, BF16, F8:
		return Vector{}, errz.Errorf("unsupported vec0 element type: %q", t.Elem)
	default:
		return Vector{}, errz.Errorf("unsupported vec0 element type: %q", t.Elem)
	}
	return Vector{Elem: t.Elem, Values: vals, Raw: cloneBytes(b)}, nil
}

// DecodeLibSQL decodes a libSQL vector blob. An even length means f32. An odd
// length means the last byte is a type tag that selects the layout.
func DecodeLibSQL(b []byte, t Type) (Vector, error) {
	if len(b)%2 == 0 {
		if len(b) != 4*t.Dim {
			return Vector{}, lenErr(b, t)
		}
		return Vector{Elem: F32, Values: decodeF32(b), Raw: cloneBytes(b)}, nil
	}

	tag := b[len(b)-1]
	body := b[:len(b)-1]
	var (
		elem Elem
		vals []float64
	)

	switch tag {
	case tagF64:
		elem = F64
		if len(body) != 8*t.Dim {
			return Vector{}, lenErr(b, t)
		}
		vals = make([]float64, t.Dim)
		for i := range vals {
			vals[i] = math.Float64frombits(binary.LittleEndian.Uint64(body[8*i:]))
		}
	case tagF16, tagBF16:
		elem = F16
		if tag == tagBF16 {
			elem = BF16
		}
		if len(body) != 2*t.Dim {
			return Vector{}, lenErr(b, t)
		}
		vals = make([]float64, t.Dim)
		for i := range vals {
			u := binary.LittleEndian.Uint16(body[2*i:])
			if tag == tagF16 {
				vals[i] = float64(halfToFloat32(u))
			} else {
				vals[i] = float64(math.Float32frombits(uint32(u) << 16))
			}
		}
	case tagBit:
		elem = Bit
		// Layout: [data][padding?][trailing_bits u8][tag].
		if len(body) < 1 {
			return Vector{}, lenErr(b, t)
		}
		trailing := int(body[len(body)-1])
		dims := 8*len(body) - trailing
		data := body[:len(body)-1]
		if dims != t.Dim || len(data) < (t.Dim+7)/8 {
			return Vector{}, lenErr(b, t)
		}
		vals = decodeBits(data, t.Dim)
	case tagF8:
		elem = F8
		// Layout: [u8 x dims][padding 0-3][alpha f32][shift f32][padding u8]
		// [trailing_bytes u8][tag].
		const footer = 10
		dataLen := len(body) - footer
		if dataLen < t.Dim || dataLen-t.Dim > 3 {
			return Vector{}, lenErr(b, t)
		}
		alpha := math.Float32frombits(binary.LittleEndian.Uint32(body[dataLen:]))
		shift := math.Float32frombits(binary.LittleEndian.Uint32(body[dataLen+4:]))
		vals = make([]float64, t.Dim)
		for i := range vals {
			vals[i] = float64(alpha*float32(body[i]) + shift)
		}
	default:
		return Vector{}, errz.Errorf("invalid libSQL vector: unknown type tag %d", tag)
	}

	return Vector{Elem: elem, Values: vals, Raw: cloneBytes(b)}, nil
}

// ParseJSON parses numeric JSON array text, which is how libSQL returns a
// vector inserted as text. The element count must equal t.Dim.
func ParseJSON(s string, t Type) (Vector, error) {
	var vals []float64
	if err := json.Unmarshal([]byte(s), &vals); err != nil {
		return Vector{}, errz.Wrapf(err, "invalid %s vector: not a numeric JSON array", t.Elem)
	}
	if len(vals) != t.Dim {
		return Vector{}, errz.Errorf("invalid %s vector: %d elements for dimension %d",
			t.Elem, len(vals), t.Dim)
	}
	if t.Elem != F64 && t.Elem != I8 && t.Elem != Bit {
		for i, f := range vals {
			vals[i] = float64(float32(f))
		}
	}
	return Vector{Elem: t.Elem, Values: vals, Raw: []byte(s)}, nil
}

func lenErr(b []byte, t Type) error {
	return errz.Errorf("invalid %s vector: %d bytes for dimension %d", t.Elem, len(b), t.Dim)
}

func cloneBytes(b []byte) []byte {
	return append([]byte(nil), b...)
}

func decodeF32(b []byte) []float64 {
	vals := make([]float64, len(b)/4)
	for i := range vals {
		vals[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:])))
	}
	return vals
}

// decodeBits unpacks n bits, least significant bit first.
func decodeBits(b []byte, n int) []float64 {
	vals := make([]float64, n)
	for i := range vals {
		if b[i/8]>>(uint(i)%8)&1 == 1 {
			vals[i] = 1
		}
	}
	return vals
}

// halfToFloat32 converts an IEEE 754 binary16 value to float32.
func halfToFloat32(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := uint32(h>>10) & 0x1f
	frac := uint32(h) & 0x3ff

	switch exp {
	case 0:
		if frac == 0 {
			return math.Float32frombits(sign)
		}
		// Subnormal: value is frac * 2^-24.
		f := float32(frac) * float32(math.Ldexp(1, -24))
		if sign != 0 {
			f = -f
		}
		return f
	case 0x1f:
		// Inf or NaN.
		return math.Float32frombits(sign | 0x7f800000 | frac<<13)
	default:
		return math.Float32frombits(sign | (exp+112)<<23 | frac<<13)
	}
}

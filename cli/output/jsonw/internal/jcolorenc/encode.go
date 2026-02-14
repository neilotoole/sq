package jcolorenc

import (
	"bytes"
	"encoding"
	"encoding/base64"
	"math"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"
	"unsafe"
)

const hex = "0123456789abcdef"

func (e encoder) encodeNull(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendNull(b), nil
}

func (e encoder) encodeBool(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendBool(b, *(*bool)(p)), nil
}

func (e encoder) encodeInt(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendInt64(b, int64(*(*int)(p))), nil
}

func (e encoder) encodeInt8(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendInt64(b, int64(*(*int8)(p))), nil
}

func (e encoder) encodeInt16(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendInt64(b, int64(*(*int16)(p))), nil
}

func (e encoder) encodeInt32(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendInt64(b, int64(*(*int32)(p))), nil
}

func (e encoder) encodeInt64(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendInt64(b, int64(*(*int64)(p))), nil
}

func (e encoder) encodeUint(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendUint64(b, uint64(*(*uint)(p))), nil
}

func (e encoder) encodeUintptr(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendUint64(b, uint64(*(*uintptr)(p))), nil
}

func (e encoder) encodeUint8(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendUint64(b, uint64(*(*uint8)(p))), nil
}

func (e encoder) encodeUint16(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendUint64(b, uint64(*(*uint16)(p))), nil
}

func (e encoder) encodeUint32(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendUint64(b, uint64(*(*uint32)(p))), nil
}

func (e encoder) encodeUint64(b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.clrs.AppendUint64(b, uint64(*(*uint64)(p))), nil
}

func (e encoder) encodeFloat32(b []byte, p unsafe.Pointer) ([]byte, error) {
	b = append(b, e.clrs.Number.Prefix...)
	var err error
	b, err = e.encodeFloat(b, float64(*(*float32)(p)), 32)
	b = append(b, e.clrs.Number.Suffix...)
	return b, err
}

func (e encoder) encodeFloat64(b []byte, p unsafe.Pointer) ([]byte, error) {
	b = append(b, e.clrs.Number.Prefix...)
	var err error
	b, err = e.encodeFloat(b, *(*float64)(p), 64)
	b = append(b, e.clrs.Number.Suffix...)
	return b, err
}

func (e encoder) encodeFloat(b []byte, f float64, bits int) ([]byte, error) {
	switch {
	case math.IsNaN(f):
		return b, &UnsupportedValueError{Value: reflect.ValueOf(f), Str: "NaN"}
	case math.IsInf(f, 0):
		return b, &UnsupportedValueError{Value: reflect.ValueOf(f), Str: "inf"}
	}

	// Convert as if by ES6 number to string conversion.
	// This matches most other JSON generators.
	// See golang.org/issue/6384 and golang.org/issue/14135.
	// Like fmt %g, but the exponent cutoffs are different
	// and exponents themselves are not padded to two digits.
	abs := math.Abs(f)
	fmt := byte('f')
	// Note: Must use float32 comparisons for underlying float32 value to get precise cutoffs right.
	if abs != 0 {
		if bits == 64 && (abs < 1e-6 || abs >= 1e21) || bits == 32 && (float32(abs) < 1e-6 || float32(abs) >= 1e21) {
			fmt = 'e'
		}
	}

	b = strconv.AppendFloat(b, f, fmt, -1, int(bits))

	if fmt == 'e' {
		// clean up e-09 to e-9
		n := len(b)
		if n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
			b[n-2] = b[n-1]
			b = b[:n-1]
		}
	}

	return b, nil
}

func (e encoder) encodeNumber(b []byte, p unsafe.Pointer) ([]byte, error) {
	n := *(*Number)(p)
	if n == "" {
		n = "0"
	}

	_, _, err := parseNumber(stringToBytes(string(n)))
	if err != nil {
		return b, err
	}

	b = append(b, e.clrs.Number.Prefix...)
	b = append(b, n...)
	b = append(b, e.clrs.Number.Suffix...)
	return b, nil
}

func (e encoder) encodeKey(b []byte, p unsafe.Pointer) ([]byte, error) {
	b = append(b, e.clrs.Key.Prefix...)
	var err error
	b, err = e.doEncodeString(b, p)
	b = append(b, e.clrs.Key.Suffix...)
	return b, err
}

func (e encoder) encodeString(b []byte, p unsafe.Pointer) ([]byte, error) {
	b = append(b, e.clrs.String.Prefix...)
	var err error
	b, err = e.doEncodeString(b, p)
	b = append(b, e.clrs.String.Suffix...)
	return b, err
}

func (e encoder) doEncodeString(b []byte, p unsafe.Pointer) ([]byte, error) {
	s := *(*string)(p)
	i := 0
	j := 0
	escapeHTML := (e.flags & EscapeHTML) != 0

	b = append(b, '"')

	for j < len(s) {
		c := s[j]

		if c >= 0x20 && c <= 0x7f && c != '\\' && c != '"' && (!escapeHTML || (c != '<' && c != '>' && c != '&')) {
			// fast path: most of the time, printable ascii characters are used
			j++
			continue
		}

		switch c {
		case '\\', '"':
			b = append(b, s[i:j]...)
			b = append(b, '\\', c)
			i = j + 1
			j = j + 1
			continue

		case '\n':
			b = append(b, s[i:j]...)
			b = append(b, '\\', 'n')
			i = j + 1
			j = j + 1
			continue

		case '\r':
			b = append(b, s[i:j]...)
			b = append(b, '\\', 'r')
			i = j + 1
			j = j + 1
			continue

		case '\t':
			b = append(b, s[i:j]...)
			b = append(b, '\\', 't')
			i = j + 1
			j = j + 1
			continue

		case '\b':
			b = append(b, s[i:j]...)
			b = append(b, '\\', 'b')
			i = j + 1
			j = j + 1
			continue

		case '\f':
			b = append(b, s[i:j]...)
			b = append(b, '\\', 'f')
			i = j + 1
			j = j + 1
			continue

		case '<', '>', '&':
			b = append(b, s[i:j]...)
			b = append(b, `\u00`...)
			b = append(b, hex[c>>4], hex[c&0xF])
			i = j + 1
			j = j + 1
			continue
		}

		// This encodes bytes < 0x20 except for \t, \n and \r.
		if c < 0x20 {
			b = append(b, s[i:j]...)
			b = append(b, `\u00`...)
			b = append(b, hex[c>>4], hex[c&0xF])
			i = j + 1
			j = j + 1
			continue
		}

		r, size := utf8.DecodeRuneInString(s[j:])

		if r == utf8.RuneError && size == 1 {
			b = append(b, s[i:j]...)
			b = append(b, `\ufffd`...)
			i = j + size
			j = j + size
			continue
		}

		switch r {
		case '\u2028', '\u2029':
			// U+2028 is LINE SEPARATOR.
			// U+2029 is PARAGRAPH SEPARATOR.
			// They are both technically valid characters in JSON strings,
			// but don't work in JSONP, which has to be evaluated as JavaScript,
			// and can lead to security holes there. It is valid JSON to
			// escape them, so we do so unconditionally.
			// See http://timelessrepo.com/json-isnt-a-javascript-subset for discussion.
			b = append(b, s[i:j]...)
			b = append(b, `\u202`...)
			b = append(b, hex[r&0xF])
			i = j + size
			j = j + size
			continue
		}

		j += size
	}

	b = append(b, s[i:]...)
	b = append(b, '"')
	return b, nil
}

func (e encoder) encodeToString(b []byte, p unsafe.Pointer, encode encodeFunc) ([]byte, error) {
	i := len(b)

	b, err := encode(e, b, p)
	if err != nil {
		return b, err
	}

	j := len(b)
	s := b[i:]

	if b, err = e.doEncodeString(b, unsafe.Pointer(&s)); err != nil {
		return b, err
	}

	n := copy(b[i:], b[j:])
	return b[:i+n], nil
}

func (e encoder) encodeBytes(b []byte, p unsafe.Pointer) ([]byte, error) {
	b = append(b, e.clrs.Bytes.Prefix...)
	var err error
	b, err = e.doEncodeBytes(b, p)
	return append(b, e.clrs.Bytes.Suffix...), err
}

func (e encoder) doEncodeBytes(b []byte, p unsafe.Pointer) ([]byte, error) {
	v := *(*[]byte)(p)
	if v == nil {
		return e.clrs.AppendNull(b), nil
	}

	n := base64.StdEncoding.EncodedLen(len(v)) + 2

	if avail := cap(b) - len(b); avail < n {
		newB := make([]byte, cap(b)+(n-avail))
		copy(newB, b)
		b = newB[:len(b)]
	}

	i := len(b)
	j := len(b) + n

	b = b[:j]
	b[i] = '"'
	base64.StdEncoding.Encode(b[i+1:j-1], v)
	b[j-1] = '"'
	return b, nil
}

func (e encoder) encodeDuration(b []byte, p unsafe.Pointer) ([]byte, error) {
	b = append(b, e.clrs.Time.Prefix...)
	b = append(b, '"')
	b = appendDuration(b, *(*time.Duration)(p))
	b = append(b, '"')
	b = append(b, e.clrs.Time.Suffix...)
	return b, nil
}

func (e encoder) encodeTime(b []byte, p unsafe.Pointer) ([]byte, error) {
	t := *(*time.Time)(p)
	b = append(b, e.clrs.Time.Prefix...)
	b = append(b, '"')
	b = t.AppendFormat(b, time.RFC3339Nano)
	b = append(b, '"')
	b = append(b, e.clrs.Time.Suffix...)
	return b, nil
}

func (e encoder) encodeArray(b []byte, p unsafe.Pointer, n int, size uintptr, t reflect.Type, encode encodeFunc) ([]byte, error) {
	start := len(b)
	var err error

	b = append(b, '[')

	if n > 0 {
		e.indenter.Push()
		for i := range n {
			if i != 0 {
				b = append(b, ',')
			}

			b = e.indenter.AppendByte(b, '\n')
			b = e.indenter.AppendIndent(b)

			if b, err = encode(e, b, unsafe.Pointer(uintptr(p)+(uintptr(i)*size))); err != nil {
				return b[:start], err
			}
		}
		e.indenter.Pop()
		b = e.indenter.AppendByte(b, '\n')
		b = e.indenter.AppendIndent(b)
	}

	b = append(b, ']')

	return b, nil
}

func (e encoder) encodeSlice(b []byte, p unsafe.Pointer, size uintptr, t reflect.Type, encode encodeFunc) ([]byte, error) {
	s := (*slice)(p)

	if s.data == nil && s.len == 0 && s.cap == 0 {
		return e.clrs.AppendNull(b), nil
	}

	return e.encodeArray(b, s.data, s.len, size, t, encode)
}

func (e encoder) encodeMap(b []byte, p unsafe.Pointer, t reflect.Type, encodeKey, encodeValue encodeFunc, sortKeys sortFunc) ([]byte, error) {
	m := reflect.NewAt(t, p).Elem()
	if m.IsNil() {
		return e.clrs.AppendNull(b), nil
	}

	keys := m.MapKeys()
	if sortKeys != nil && (e.flags&SortMapKeys) != 0 {
		sortKeys(keys)
	}

	start := len(b)
	var err error
	b = append(b, '{')

	if len(keys) != 0 {
		b = e.indenter.AppendByte(b, '\n')

		e.indenter.Push()
		for i, k := range keys {
			v := m.MapIndex(k)

			if i != 0 {
				b = append(b, ',')
				b = e.indenter.AppendByte(b, '\n')
			}

			b = e.indenter.AppendIndent(b)
			if b, err = encodeKey(e, b, (*iface)(unsafe.Pointer(&k)).ptr); err != nil {
				return b[:start], err
			}

			b = append(b, ':')
			b = e.indenter.AppendByte(b, ' ')

			if b, err = encodeValue(e, b, (*iface)(unsafe.Pointer(&v)).ptr); err != nil {
				return b[:start], err
			}
		}
		b = e.indenter.AppendByte(b, '\n')
		e.indenter.Pop()
		b = e.indenter.AppendIndent(b)
	}

	b = append(b, '}')
	return b, nil
}

type element struct {
	key string
	val any
	raw RawMessage
}

type mapslice struct {
	elements []element
}

func (m *mapslice) Len() int           { return len(m.elements) }
func (m *mapslice) Less(i, j int) bool { return m.elements[i].key < m.elements[j].key }
func (m *mapslice) Swap(i, j int)      { m.elements[i], m.elements[j] = m.elements[j], m.elements[i] }

var mapslicePool = sync.Pool{
	New: func() any { return new(mapslice) },
}

func (e encoder) encodeMapStringInterface(b []byte, p unsafe.Pointer) ([]byte, error) {
	m := *(*map[string]any)(p)
	if m == nil {
		return e.clrs.AppendNull(b), nil
	}

	if (e.flags & SortMapKeys) == 0 {
		// Optimized code path when the program does not need the map keys to be
		// sorted.
		b = append(b, '{')

		if len(m) != 0 {
			b = e.indenter.AppendByte(b, '\n')

			var err error
			i := 0

			e.indenter.Push()
			for k, v := range m {
				if i != 0 {
					b = append(b, ',')
					b = e.indenter.AppendByte(b, '\n')
				}

				b = e.indenter.AppendIndent(b)

				b, err = e.encodeKey(b, unsafe.Pointer(&k))
				if err != nil {
					return b, err
				}

				b = append(b, ':')
				b = e.indenter.AppendByte(b, ' ')

				b, err = Append(b, v, e.flags, e.clrs, e.indenter)
				if err != nil {
					return b, err
				}

				i++
			}
			b = e.indenter.AppendByte(b, '\n')
			e.indenter.Pop()
			b = e.indenter.AppendIndent(b)
		}

		b = append(b, '}')
		return b, nil
	}

	s := mapslicePool.Get().(*mapslice)
	if cap(s.elements) < len(m) {
		s.elements = make([]element, 0, align(10, uintptr(len(m))))
	}
	for key, val := range m {
		s.elements = append(s.elements, element{key: key, val: val})
	}
	sort.Sort(s)

	start := len(b)
	var err error
	b = append(b, '{')

	if len(s.elements) > 0 {
		b = e.indenter.AppendByte(b, '\n')

		e.indenter.Push()
		for i, elem := range s.elements {
			if i != 0 {
				b = append(b, ',')
				b = e.indenter.AppendByte(b, '\n')
			}

			b = e.indenter.AppendIndent(b)

			b, _ = e.encodeKey(b, unsafe.Pointer(&elem.key))
			b = append(b, ':')
			b = e.indenter.AppendByte(b, ' ')

			b, err = Append(b, elem.val, e.flags, e.clrs, e.indenter)
			if err != nil {
				break
			}
		}
		b = e.indenter.AppendByte(b, '\n')
		e.indenter.Pop()
		b = e.indenter.AppendIndent(b)
	}

	for i := range s.elements {
		s.elements[i] = element{}
	}

	s.elements = s.elements[:0]
	mapslicePool.Put(s)

	if err != nil {
		return b[:start], err
	}

	b = append(b, '}')
	return b, nil
}

func (e encoder) encodeMapStringRawMessage(b []byte, p unsafe.Pointer) ([]byte, error) {
	m := *(*map[string]RawMessage)(p)
	if m == nil {
		return e.clrs.AppendNull(b), nil
	}

	if (e.flags & SortMapKeys) == 0 {
		// Optimized code path when the program does not need the map keys to be
		// sorted.
		b = append(b, '{')

		if len(m) != 0 {
			b = e.indenter.AppendByte(b, '\n')

			var err error
			i := 0

			e.indenter.Push()
			for k, v := range m {
				if i != 0 {
					b = append(b, ',')
					b = e.indenter.AppendByte(b, '\n')
				}

				b = e.indenter.AppendIndent(b)

				b, _ = e.encodeKey(b, unsafe.Pointer(&k))

				b = append(b, ':')
				b = e.indenter.AppendByte(b, ' ')

				b, err = e.encodeRawMessage(b, unsafe.Pointer(&v))
				if err != nil {
					break
				}

				i++
			}
			b = e.indenter.AppendByte(b, '\n')
			e.indenter.Pop()
			b = e.indenter.AppendIndent(b)
		}

		b = append(b, '}')
		return b, nil
	}

	s := mapslicePool.Get().(*mapslice)
	if cap(s.elements) < len(m) {
		s.elements = make([]element, 0, align(10, uintptr(len(m))))
	}
	for key, raw := range m {
		s.elements = append(s.elements, element{key: key, raw: raw})
	}
	sort.Sort(s)

	start := len(b)
	var err error
	b = append(b, '{')

	if len(s.elements) > 0 {
		b = e.indenter.AppendByte(b, '\n')

		e.indenter.Push()

		for i, elem := range s.elements {
			if i != 0 {
				b = append(b, ',')
				b = e.indenter.AppendByte(b, '\n')
			}

			b = e.indenter.AppendIndent(b)

			b, _ = e.encodeKey(b, unsafe.Pointer(&elem.key))
			b = append(b, ':')
			b = e.indenter.AppendByte(b, ' ')

			b, err = e.encodeRawMessage(b, unsafe.Pointer(&elem.raw))
			if err != nil {
				break
			}
		}
		b = e.indenter.AppendByte(b, '\n')
		e.indenter.Pop()
		b = e.indenter.AppendIndent(b)
	}

	for i := range s.elements {
		s.elements[i] = element{}
	}

	s.elements = s.elements[:0]
	mapslicePool.Put(s)

	if err != nil {
		return b[:start], err
	}

	b = append(b, '}')
	return b, nil
}

func (e encoder) encodeStruct(b []byte, p unsafe.Pointer, st *structType) ([]byte, error) {
	start := len(b)
	var err error
	var k string
	var n int

	b = append(b, '{')

	if len(st.fields) > 0 {
		b = e.indenter.AppendByte(b, '\n')
	}

	e.indenter.Push()

	for i := range st.fields {
		f := &st.fields[i]
		v := unsafe.Pointer(uintptr(p) + f.offset)

		if f.omitempty && f.empty(v) {
			continue
		}

		if n != 0 {
			b = append(b, ',')
			b = e.indenter.AppendByte(b, '\n')
		}

		if (e.flags & EscapeHTML) != 0 {
			k = f.html
		} else {
			k = f.json
		}

		lengthBeforeKey := len(b)
		b = e.indenter.AppendIndent(b)

		b = append(b, e.clrs.Key.Prefix...)
		b = append(b, k...)
		b = append(b, e.clrs.Key.Suffix...)
		b = append(b, ':')

		b = e.indenter.AppendByte(b, ' ')

		if b, err = f.codec.encode(e, b, v); err != nil {
			if err == (rollback{}) {
				b = b[:lengthBeforeKey]
				continue
			}
			return b[:start], err
		}

		n++
	}

	if n > 0 {
		b = e.indenter.AppendByte(b, '\n')
	}

	e.indenter.Pop()
	b = e.indenter.AppendIndent(b)

	b = append(b, '}')
	return b, nil
}

type rollback struct{}

func (rollback) Error() string { return "rollback" }

func (e encoder) encodeEmbeddedStructPointer(b []byte, p unsafe.Pointer, t reflect.Type, unexported bool, offset uintptr, encode encodeFunc) ([]byte, error) {
	p = *(*unsafe.Pointer)(p)
	if p == nil {
		return b, rollback{}
	}
	return encode(e, b, unsafe.Pointer(uintptr(p)+offset))
}

func (e encoder) encodePointer(b []byte, p unsafe.Pointer, t reflect.Type, encode encodeFunc) ([]byte, error) {
	if p = *(*unsafe.Pointer)(p); p != nil {
		return encode(e, b, p)
	}
	return e.encodeNull(b, nil)
}

func (e encoder) encodeInterface(b []byte, p unsafe.Pointer) ([]byte, error) {
	return Append(b, *(*any)(p), e.flags, e.clrs, e.indenter)
}

func (e encoder) encodeMaybeEmptyInterface(b []byte, p unsafe.Pointer, t reflect.Type) ([]byte, error) {
	return Append(b, reflect.NewAt(t, p).Elem().Interface(), e.flags, e.clrs, e.indenter)
}

func (e encoder) encodeUnsupportedTypeError(b []byte, p unsafe.Pointer, t reflect.Type) ([]byte, error) {
	return b, &UnsupportedTypeError{Type: t}
}

func (e encoder) encodeRawMessage(b []byte, p unsafe.Pointer) ([]byte, error) {
	v := *(*RawMessage)(p)

	if v == nil {
		return e.clrs.AppendNull(b), nil
	}

	var s []byte

	if (e.flags & TrustRawMessage) != 0 {
		s = v
	} else {
		var err error
		s, _, err = parseValue(v)
		if err != nil {
			return b, &UnsupportedValueError{Value: reflect.ValueOf(v), Str: err.Error()}
		}
	}

	if e.indenter == nil {
		if (e.flags & EscapeHTML) != 0 {
			return appendCompactEscapeHTML(b, s), nil
		}

		return append(b, s...), nil
	}

	// In order to get the tests inherited from the original segmentio
	// encoder to work, we need to support indentation. However, due to
	// the complexity of parsing and then colorizing, we're not going to
	// go to the effort of adding color support for JSONMarshaler right
	// now. Possibly revisit this in future if needed.

	// This below is sloppy, but seems to work.
	if (e.flags & EscapeHTML) != 0 {
		s = appendCompactEscapeHTML(nil, s)
	}

	// The "prefix" arg to Indent is the current indentation.
	pre := e.indenter.AppendIndent(nil)

	buf := &bytes.Buffer{}
	// And now we just make use of the existing Indent function.
	err := Indent(buf, s, string(pre), e.indenter.Indent)
	if err != nil {
		return b, err
	}

	s = buf.Bytes()

	return append(b, s...), nil
}

func (e encoder) encodeJSONMarshaler(b []byte, p unsafe.Pointer, t reflect.Type, pointer bool) ([]byte, error) {
	v := reflect.NewAt(t, p)

	if !pointer {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return e.clrs.AppendNull(b), nil
		}
	}

	j, err := v.Interface().(Marshaler).MarshalJSON()
	if err != nil {
		return b, err
	}

	s, _, err := parseValue(j)
	if err != nil {
		return b, &MarshalerError{Type: t, Err: err}
	}

	if e.indenter == nil {
		if (e.flags & EscapeHTML) != 0 {
			return appendCompactEscapeHTML(b, s), nil
		}

		return append(b, s...), nil
	}

	// In order to get the tests inherited from the original segmentio
	// encoder to work, we need to support indentation. However, due to
	// the complexity of parsing and then colorizing, we're not going to
	// go to the effort of supporting color for JSONMarshaler.
	// Possibly revisit this in future if needed.

	// This below is sloppy, but seems to work.
	if (e.flags & EscapeHTML) != 0 {
		s = appendCompactEscapeHTML(nil, s)
	}

	// The "prefix" arg to Indent is the current indentation.
	pre := e.indenter.AppendIndent(nil)

	buf := &bytes.Buffer{}
	// And now we just make use of the existing Indent function.
	err = Indent(buf, s, string(pre), e.indenter.Indent)
	if err != nil {
		return b, err
	}

	s = buf.Bytes()

	return append(b, s...), nil
}

func (e encoder) encodeTextMarshaler(b []byte, p unsafe.Pointer, t reflect.Type, pointer bool) ([]byte, error) {
	v := reflect.NewAt(t, p)

	if !pointer {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return append(b, `null`...), nil
		}
	}

	s, err := v.Interface().(encoding.TextMarshaler).MarshalText()
	if err != nil {
		return b, err
	}

	return e.doEncodeString(b, unsafe.Pointer(&s))
}

func appendCompactEscapeHTML(dst, src []byte) []byte {
	start := 0
	escape := false
	inString := false

	for i, c := range src {
		if !inString {
			switch c {
			case '"': // enter string
				inString = true
			case ' ', '\n', '\r', '\t': // skip space
				if start < i {
					dst = append(dst, src[start:i]...)
				}
				start = i + 1
			}
			continue
		}

		if escape {
			escape = false
			continue
		}

		if c == '\\' {
			escape = true
			continue
		}

		if c == '"' {
			inString = false
			continue
		}

		if c == '<' || c == '>' || c == '&' {
			if start < i {
				dst = append(dst, src[start:i]...)
			}
			dst = append(dst, `\u00`...)
			dst = append(dst, hex[c>>4], hex[c&0xF])
			start = i + 1
			continue
		}

		// Convert U+2028 and U+2029 (E2 80 A8 and E2 80 A9).
		if c == 0xE2 && i+2 < len(src) && src[i+1] == 0x80 && src[i+2]&^1 == 0xA8 {
			if start < i {
				dst = append(dst, src[start:i]...)
			}
			dst = append(dst, `\u202`...)
			dst = append(dst, hex[src[i+2]&0xF])
			start = i + 3
			continue
		}
	}

	if start < len(src) {
		dst = append(dst, src[start:]...)
	}

	return dst
}

// Indenter is used to indent JSON. The Push and Pop methods
// change indentation level. The AppendIndent method appends the
// computed indentation. The AppendByte method appends a byte. All
// methods are safe to use with a nil receiver.
type Indenter struct {
	Prefix   string
	Indent   string
	depth    int
	disabled bool
}

// NewIndenter returns a new Indenter instance. If prefix and
// indent are both empty, the indenter is effectively disabled,
// and the AppendIndent and AppendByte methods are no-op.
func NewIndenter(prefix, indent string) *Indenter {
	return &Indenter{
		disabled: prefix == "" && indent == "",
		Prefix:   prefix,
		Indent:   indent,
	}
}

// Push increases the indentation level.
func (in *Indenter) Push() {
	if in != nil {
		in.depth++
	}
}

// Pop decreases the indentation level.
func (in *Indenter) Pop() {
	if in != nil {
		in.depth--
	}
}

// AppendByte appends a to b if the indenter is non-nil and enabled.
// Otherwise b is returned unmodified.
func (in *Indenter) AppendByte(b []byte, a byte) []byte {
	if in == nil || in.disabled {
		return b
	}

	return append(b, a)
}

// AppendIndent writes indentation to b, returning the resulting slice.
// If the indenter is nil or disabled b is returned unchanged.
func (in *Indenter) AppendIndent(b []byte) []byte {
	if in == nil || in.disabled {
		return b
	}

	b = append(b, in.Prefix...)
	for i := 0; i < in.depth; i++ {
		b = append(b, in.Indent...)
	}
	return b
}

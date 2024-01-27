package jsonw

import (
	"encoding/base64"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw/internal"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// monoEncoder provides methods for encoding JSON values
// without colorization (that is, in monochrome).
type monoEncoder struct {
	formatDatetime         func(time.Time) string
	formatDate             func(time.Time) string
	formatTime             func(time.Time) string
	formatDatetimeAsNumber bool
	formatDateAsNumber     bool
	formatTimeAsNumber     bool
}

func (e monoEncoder) encodeTime(b []byte, v any) ([]byte, error) {
	return e.doEncodeTime(b, v, e.formatTime, e.formatTimeAsNumber)
}

func (e monoEncoder) encodeDatetime(b []byte, v any) ([]byte, error) {
	return e.doEncodeTime(b, v, e.formatDatetime, e.formatDatetimeAsNumber)
}

func (e monoEncoder) encodeDate(b []byte, v any) ([]byte, error) {
	return e.doEncodeTime(b, v, e.formatDate, e.formatDateAsNumber)
}

func (e monoEncoder) doEncodeTime(b []byte, v any, fn func(time.Time) string, asNumber bool) ([]byte, error) {
	switch v := v.(type) {
	case nil:
		return append(b, "null"...), nil
	case time.Time:
		s := fn(v)
		if asNumber {
			if i, err := strconv.ParseInt(s, 10, 64); err == nil {
				b = strconv.AppendInt(b, i, 10)
				return b, nil
			}
		}
		b = append(b, '"')
		b = append(b, []byte(s)...)
		b = append(b, '"')
		return b, nil
	case string:
		// If we've got a string, assume it's in the correct format
		return encodeString(b, v, false)
	default:
		return b, errz.Errorf("unsupported time type %T: %v", v, v)
	}
}

func (e monoEncoder) encodeAny(b []byte, v any) ([]byte, error) {
	switch v := v.(type) {
	default:
		return b, errz.Errorf("unexpected record field type %T: %#v", v, v)

	case nil:
		return append(b, "null"...), nil

	case int64:
		return strconv.AppendInt(b, v, 10), nil

	case float64:
		return append(b, stringz.FormatFloat(v)...), nil

	case bool:
		return strconv.AppendBool(b, v), nil

	case []byte:
		var err error
		b, err = encodeBytes(b, v)
		if err != nil {
			return b, errz.Err(err)
		}
		return b, nil

	case string:
		var err error
		b, err = encodeString(b, v, false)
		if err != nil {
			return b, errz.Err(err)
		}
		return b, nil

	case decimal.Decimal:
		var err error
		b, err = encodeString(b, stringz.FormatDecimal(v), false)
		if err != nil {
			return b, errz.Err(err)
		}
		return b, nil

	case time.Time:
		// We really shouldn't be hitting this path? Instead should
		// hit encodeTime.
		return e.doEncodeTime(b, v, e.formatDatetime, e.formatDatetimeAsNumber)
	}
}

// colorEncoder provides methods for encoding JSON values
// with color.
type colorEncoder struct {
	formatDatetime         func(time.Time) string
	formatDate             func(time.Time) string
	formatTime             func(time.Time) string
	clrs                   internal.Colors
	formatDatetimeAsNumber bool
	formatDateAsNumber     bool
	formatTimeAsNumber     bool
}

func (e *colorEncoder) encodeTime(b []byte, v any) ([]byte, error) {
	return e.doEncodeTime(b, v, e.formatTime, e.formatTimeAsNumber)
}

func (e *colorEncoder) encodeDatetime(b []byte, v any) ([]byte, error) {
	return e.doEncodeTime(b, v, e.formatDatetime, e.formatDatetimeAsNumber)
}

func (e *colorEncoder) encodeDate(b []byte, v any) ([]byte, error) {
	return e.doEncodeTime(b, v, e.formatDate, e.formatDateAsNumber)
}

func (e *colorEncoder) doEncodeTime(b []byte, v any, fn func(time.Time) string, asNumber bool) ([]byte, error) {
	start := len(b)

	switch v := v.(type) {
	case nil:
		return e.clrs.AppendNull(b), nil
	case time.Time:
		b = append(b, e.clrs.Time.Prefix...)
		s := fn(v)

		if asNumber {
			if i, err := strconv.ParseInt(s, 10, 64); err == nil {
				b = strconv.AppendInt(b, i, 10)
				b = append(b, e.clrs.Time.Suffix...)
				return b, nil
			}
		}

		b = append(b, '"')
		b = append(b, []byte(s)...)
		b = append(b, '"')
		b = append(b, e.clrs.Time.Suffix...)
		return b, nil

	case string:
		// If we've got a string, assume it's in the correct format
		b = append(b, e.clrs.Time.Prefix...)
		var err error
		b, err = encodeString(b, v, false)
		if err != nil {
			return b[0:start], err
		}
		b = append(b, e.clrs.Time.Suffix...)
		return b, nil
	default:
		return b, errz.Errorf("unsupported time type %T: %v", v, v)
	}
}

func (e *colorEncoder) encodeAny(b []byte, v any) ([]byte, error) {
	switch v := v.(type) {
	default:
		return b, errz.Errorf("unexpected record field type %T: %#v", v, v)

	case nil:
		return e.clrs.AppendNull(b), nil

	case int64:
		b = append(b, e.clrs.Number.Prefix...)
		b = strconv.AppendInt(b, v, 10)
		return append(b, e.clrs.Number.Suffix...), nil

	case float64:
		b = append(b, e.clrs.Number.Prefix...)
		b = append(b, stringz.FormatFloat(v)...)
		return append(b, e.clrs.Number.Suffix...), nil

	case bool:
		b = append(b, e.clrs.Bool.Prefix...)
		b = strconv.AppendBool(b, v)
		return append(b, e.clrs.Bool.Suffix...), nil

	case []byte:
		var err error
		b = append(b, e.clrs.Bytes.Prefix...)
		b, err = encodeBytes(b, v)
		if err != nil {
			return b, errz.Err(err)
		}
		b = append(b, e.clrs.Bytes.Suffix...)
		return b, nil

	case string:
		b = append(b, e.clrs.String.Prefix...)
		var err error
		b, err = encodeString(b, v, false)
		if err != nil {
			return b, errz.Err(err)
		}
		return append(b, e.clrs.String.Suffix...), nil

	case decimal.Decimal:
		b = append(b, e.clrs.Number.Prefix...)
		var err error
		b, err = encodeString(b, stringz.FormatDecimal(v), false)
		if err != nil {
			return b, errz.Err(err)
		}
		return append(b, e.clrs.Number.Suffix...), nil

	case time.Time:
		// We really shouldn't be hitting this path? Instead should
		// hit encodeTime.
		return e.doEncodeTime(b, v, e.formatDatetime, e.formatDatetimeAsNumber)
	}
}

// punc holds the byte values of JSON punctuation chars
// like left bracket "[", right brace "}" etc. When
// colorizing, these values will include the terminal color codes.
type punc struct {
	comma    []byte
	colon    []byte
	lBrace   []byte
	rBrace   []byte
	lBracket []byte
	rBracket []byte
	// null is also included in punc just for convenience
	null []byte
}

func newPunc(pr *output.Printing) punc {
	var p punc

	if pr == nil || pr.IsMonochrome() || pr.Compact {
		p.comma = append(p.comma, ',')
		p.colon = append(p.colon, ':')
		p.lBrace = append(p.lBrace, '{')
		p.rBrace = append(p.rBrace, '}')
		p.lBracket = append(p.lBracket, '[')
		p.rBracket = append(p.rBracket, ']')
		p.null = append(p.null, "null"...)
		return p
	}

	clrs := internal.NewColors(pr)
	p.comma = clrs.AppendPunc(p.comma, ',')
	p.colon = clrs.AppendPunc(p.colon, ':')
	p.lBrace = clrs.AppendPunc(p.lBrace, '{')
	p.rBrace = clrs.AppendPunc(p.rBrace, '}')
	p.lBracket = clrs.AppendPunc(p.lBracket, '[')
	p.rBracket = clrs.AppendPunc(p.rBracket, ']')
	p.null = clrs.AppendNull(p.null)
	return p
}

func getFieldEncoders(recMeta record.Meta, pr *output.Printing) []func(b []byte, v any) ([]byte, error) {
	encodeFns := make([]func(b []byte, v any) ([]byte, error), len(recMeta))

	if pr.IsMonochrome() {
		enc := monoEncoder{
			formatDatetime:         pr.FormatDatetime,
			formatDatetimeAsNumber: pr.FormatDatetimeAsNumber,
			formatDate:             pr.FormatDate,
			formatDateAsNumber:     pr.FormatDateAsNumber,
			formatTime:             pr.FormatTime,
			formatTimeAsNumber:     pr.FormatTimeAsNumber,
		}

		for i := 0; i < len(recMeta); i++ {
			switch recMeta[i].Kind() { //nolint:exhaustive
			case kind.Time:
				encodeFns[i] = enc.encodeTime
			case kind.Date:
				encodeFns[i] = enc.encodeDate
			case kind.Datetime:
				encodeFns[i] = enc.encodeDatetime
			default:
				encodeFns[i] = enc.encodeAny
			}
		}

		return encodeFns
	}

	clrs := internal.NewColors(pr)

	// Else, we want color encoders
	enc := &colorEncoder{
		clrs:                   clrs,
		formatDatetime:         pr.FormatDatetime,
		formatDatetimeAsNumber: pr.FormatDatetimeAsNumber,
		formatDate:             pr.FormatDate,
		formatDateAsNumber:     pr.FormatDateAsNumber,
		formatTime:             pr.FormatTime,
		formatTimeAsNumber:     pr.FormatTimeAsNumber,
	}
	for i := 0; i < len(recMeta); i++ {
		switch recMeta[i].Kind() { //nolint:exhaustive
		case kind.Time:
			encodeFns[i] = enc.encodeTime
		case kind.Date:
			encodeFns[i] = enc.encodeDate
		case kind.Datetime:
			encodeFns[i] = enc.encodeDatetime
		default:
			encodeFns[i] = enc.encodeAny
		}
	}

	return encodeFns
}

// encodeString encodes s, appending to b and returning
// the resulting []byte.
func encodeString(b []byte, s string, escapeHTML bool) ([]byte, error) { //nolint:unparam
	// This function is copied from the segment.io JSON encoder.
	const hex = "0123456789abcdef"

	i := 0
	j := 0

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
			j++
			continue

		case '\n':
			b = append(b, s[i:j]...)
			b = append(b, '\\', 'n')
			i = j + 1
			j++
			continue

		case '\r':
			b = append(b, s[i:j]...)
			b = append(b, '\\', 'r')
			i = j + 1
			j++
			continue

		case '\t':
			b = append(b, s[i:j]...)
			b = append(b, '\\', 't')
			i = j + 1
			j++
			continue

		case '<', '>', '&':
			b = append(b, s[i:j]...)
			b = append(b, `\u00`...)
			b = append(b, hex[c>>4], hex[c&0xF])
			i = j + 1
			j++
			continue
		}

		// This encodes bytes < 0x20 except for \t, \n and \r.
		if c < 0x20 {
			b = append(b, s[i:j]...)
			b = append(b, `\u00`...)
			b = append(b, hex[c>>4], hex[c&0xF])
			i = j + 1
			j++
			continue
		}

		r, size := utf8.DecodeRuneInString(s[j:])

		if r == utf8.RuneError && size == 1 {
			b = append(b, s[i:j]...)
			b = append(b, `\ufffd`...)
			i = j + size
			j += size
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
			j += size
			continue
		}

		j += size
	}

	b = append(b, s[i:]...)
	b = append(b, '"')
	return b, nil
}

// encodeBytes encodes v in base64 and appends to b, returning
// the resulting slice.
func encodeBytes(b, v []byte) ([]byte, error) { //nolint:unparam
	// This function is copied from the segment.io JSON encoder.

	if v == nil {
		return append(b, "null"...), nil
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

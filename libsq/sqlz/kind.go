package sqlz

import (
	"strings"

	"github.com/neilotoole/sq/libsq/errz"
)

const (
	// KindUnknown indicates an unknown kind.
	KindUnknown Kind = iota

	// KindNull indicates a NULL kind.
	KindNull

	// KindText indicates a text kind.
	KindText

	// KindInt indicates an integer kind.
	KindInt

	// KindFloat indicates a float kind.
	KindFloat

	// KindDecimal indicates a decimal kind.
	KindDecimal

	// KindBool indicates a boolean kind.
	KindBool

	// KindBytes indicates a bytes or blob kind.
	KindBytes

	// KindDatetime indicates a date-time kind.
	KindDatetime

	// KindDate indicates a date-only kind.
	KindDate

	// KindTime indicates a time-only kind.
	KindTime
)

// Kind models a generic data kind, which ultimately maps
// to some more specific implementation data type,
// such as a SQL VARCHAR or JSON boolean.
type Kind int

func (d Kind) String() string {
	t, err := d.MarshalText()
	if err != nil {
		return "<err>"
	}

	return string(t)
}

// MarshalJSON implements json.Marshaler.
func (d Kind) MarshalJSON() ([]byte, error) {
	t, err := d.MarshalText()
	if err != nil {
		return nil, err
	}

	return []byte(`"` + string(t) + `"`), nil
}

// MarshalText implements encoding.TextMarshaler.
func (d Kind) MarshalText() ([]byte, error) {
	var name string
	switch d {
	case KindUnknown:
		name = "unknown"
	case KindNull:
		name = "null"
	case KindText:
		name = "text"
	case KindInt:
		name = "int"
	case KindFloat:
		name = "float"
	case KindDecimal:
		name = "decimal"
	case KindBool:
		name = "bool"
	case KindDatetime:
		name = "datetime"
	case KindDate:
		name = "date"
	case KindTime:
		name = "time"
	case KindBytes:
		name = "bytes"
	default:
		return nil, errz.Errorf("invalid data kind '%d'", d)
	}

	return []byte(name), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Kind) UnmarshalText(text []byte) error {
	kind, err := parse(string(text))
	if err != nil {
		return err
	}

	*d = kind
	return nil
}

// parse parses text and returns the appropriate kind, or
// an error.
func parse(text string) (Kind, error) {
	switch strings.ToLower(text) {
	default:
		return KindUnknown, errz.Errorf("unrecognized kind name %q", text)
	case "unknown":
		return KindUnknown, nil
	case "text":
		return KindText, nil
	case "int":
		return KindInt, nil
	case "float":
		return KindFloat, nil
	case "decimal":
		return KindDecimal, nil
	case "bool":
		return KindBool, nil
	case "datetime":
		return KindDatetime, nil
	case "date":
		return KindDate, nil
	case "time":
		return KindTime, nil
	case "bytes":
		return KindBytes, nil
	case "null":
		return KindNull, nil
	}
}

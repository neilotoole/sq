// Package kind encapsulates the notion of data "kind": that is, it
// is an abstraction over data types across implementations.
package kind

import (
	"slices"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
)

const (
	// Unknown indicates an unknown kind.
	Unknown Kind = iota

	// Null indicates a NULL kind.
	Null

	// Text indicates a text kind.
	Text

	// Int indicates an integer kind.
	Int

	// Float indicates a float kind.
	Float

	// Decimal indicates a decimal kind.
	Decimal

	// Bool indicates a boolean kind.
	Bool

	// Bytes indicates a bytes or blob kind.
	Bytes

	// Datetime indicates a date-time kind.
	Datetime

	// Date indicates a date-only kind. For example "2022-12-31".
	Date

	// Time indicates a time-only kind.
	Time
)

// Kind models a generic data kind, which ultimately maps
// to some more specific implementation data type,
// such as a SQL VARCHAR or JSON boolean.
type Kind int

// String returns a log/debug-friendly representation.
func (k Kind) String() string {
	t, err := k.MarshalText()
	if err != nil {
		return "<err>"
	}

	return string(t)
}

// MarshalJSON implements json.Marshaler.
func (k Kind) MarshalJSON() ([]byte, error) {
	t, err := k.MarshalText()
	if err != nil {
		return nil, err
	}

	return []byte(`"` + string(t) + `"`), nil
}

// MarshalText implements encoding.TextMarshaler.
func (k Kind) MarshalText() ([]byte, error) {
	var name string
	switch k {
	case Unknown:
		name = "unknown"
	case Null:
		name = "null"
	case Text:
		name = "text"
	case Int:
		name = "int"
	case Float:
		name = "float"
	case Decimal:
		name = "decimal"
	case Bool:
		name = "bool"
	case Datetime:
		name = "datetime"
	case Date:
		name = "date"
	case Time:
		name = "time"
	case Bytes:
		name = "bytes"
	default:
		return nil, errz.Errorf("invalid data kind '%d'", k)
	}

	return []byte(name), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (k *Kind) UnmarshalText(text []byte) error {
	kind, err := parse(string(text))
	if err != nil {
		return err
	}

	*k = kind
	return nil
}

// parse parses text and returns the appropriate kind, or
// an error.
func parse(text string) (Kind, error) {
	switch strings.ToLower(text) {
	default:
		return Unknown, errz.Errorf("unrecognized kind name {%s}", text)
	case "unknown":
		return Unknown, nil
	case "text":
		return Text, nil
	case "int":
		return Int, nil
	case "float":
		return Float, nil
	case "decimal":
		return Decimal, nil
	case "bool":
		return Bool, nil
	case "datetime":
		return Datetime, nil
	case "date":
		return Date, nil
	case "time":
		return Time, nil
	case "bytes":
		return Bytes, nil
	case "null":
		return Null, nil
	}
}

func containsKind(needle Kind, haystack ...Kind) bool {
	return slices.Contains(haystack, needle)
}

// Package kind encapsulates the notion of data "kind": that is, it
// is an abstraction over data types across implementations.
package kind

import (
	stdj "encoding/json"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
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

	// Date indicates a date-only kind.
	Date

	// Time indicates a time-only kind.
	Time
)

// Kind models a generic data kind, which ultimately maps
// to some more specific implementation data type,
// such as a SQL VARCHAR or JSON boolean.
type Kind int

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
		return Unknown, errz.Errorf("unrecognized kind name %q", text)
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

// Detector is used to detect the kind of a stream of values.
// The caller adds values via Sample and then invokes Detect.
type Detector struct {
	kinds       map[Kind]struct{}
	mungeFns    map[Kind]MungeFunc
	dirty       bool
	foundString bool
}

// MungeFunc is a function that accepts a value and returns a munged
// value with the appropriate Kind. For example, a Datetime MungeFunc
// would accept string "2020-06-11T02:50:54Z" and return a time.Time,
type MungeFunc func(interface{}) (interface{}, error)

// NewDetector returns a new instance.
func NewDetector() *Detector {
	return &Detector{
		kinds: map[Kind]struct{}{
			Int:      {},
			Float:    {},
			Decimal:  {},
			Bool:     {},
			Time:     {},
			Date:     {},
			Datetime: {},
		},
		mungeFns: map[Kind]MungeFunc{},
	}
}

// Sample adds a sample to the detector.
func (d *Detector) Sample(v interface{}) {
	switch v.(type) {
	case nil:
		// Can't glean any info from nil
		return
	default:
		// Don't know what this, so delete all kinds
		d.retain()
		return
	case float32, float64:
		d.retain(Float, Decimal)
		return
	case int, int8, int16, int32, int64:
		d.retain(Int, Float, Decimal)
		return
	case bool:
		d.retain(Bool)
		return
	case time.Time:
		d.retain(Time, Date, Datetime)
		return
	case stdj.Number:
		// JSON number
		d.foundString = true
		d.retain(Decimal)
		return
	case string:
		// We need to do more work to figure out the kind when
		// we're getting string values
		d.foundString = true
	}

	// We're dealing with a string value, which could a variety
	// of things, such as: "1", "1.0", "true", "11:30".
	s := v.(string)

	if s == "" {
		// Can't really do anything useful with this
		return
	}

	var err error

	if d.has(Decimal) {
		// If Decimal is still a candidate, check that we can parse it
		if _, _, err = big.ParseFloat(s, 10, 64, 0); err != nil {
			// If s cannot be parsed as a decimal, it also can't
			// be int or float
			d.delete(Decimal, Int, Float)
		} else {
			// s can be parsed as decimal, can't be time
			d.delete(Time, Date, Datetime)
		}
	}

	if d.has(Int) {
		if _, err = strconv.ParseInt(s, 10, 64); err != nil {
			d.delete(Int)
		} else {
			// s can be parsed as int, can't be time
			d.delete(Time, Date, Datetime)
		}
	}

	if d.has(Float) {
		if _, err = strconv.ParseFloat(s, 64); err != nil {
			d.delete(Float)
		} else {
			// s can be parsed as float, can't be time
			d.delete(Time, Date, Datetime)
		}
	}

	if d.has(Bool) {
		if _, err = stringz.ParseBool(s); err != nil {
			d.delete(Bool)
		} else {
			// s can be parsed as bool, can't be time,
			// but still could be int ("1" == true)
			d.delete(Float, Time, Date, Datetime)
		}
	}

	if d.has(Time) {
		ok, format := detectKindTime(s)
		if !ok {
			// It's not a recognized time format
			d.delete(Time)
		} else {
			// If it's kind.Time, it can't be anything else
			d.retain(Time)

			d.mungeFns[Time] = func(val interface{}) (interface{}, error) {
				if val == nil {
					return nil, nil
				}

				s, ok = val.(string)
				if !ok {
					return nil, errz.Errorf("expected %T to be string", val)
				}

				if s == "" {
					return nil, nil
				}

				var t time.Time
				t, err = time.Parse(format, s)
				if err != nil {
					return nil, errz.Err(err)
				}

				return t.Format(format), nil
			}
		}
	}

	if d.has(Date) {
		ok, format := detectKindDate(s)
		if !ok {
			// It's not a recognized date format
			d.delete(Date)
		} else {
			// If it's kind.Date, it can't be anything else
			d.retain(Date)

			d.mungeFns[Date] = func(val interface{}) (interface{}, error) {
				if val == nil {
					return nil, nil
				}

				s, ok = val.(string)
				if !ok {
					return nil, errz.Errorf("expected %T to be string", val)
				}

				if s == "" {
					return nil, nil
				}

				var t time.Time
				t, err = time.Parse(format, s)
				if err != nil {
					return nil, errz.Err(err)
				}

				return t.Format(format), nil
			}
		}
	}

	if d.has(Datetime) {
		ok, format := detectKindDatetime(s)
		if !ok {
			// It's not a recognized datetime format
			d.delete(Datetime)
		} else {
			// If it's kind.Datetime, it can't be anything else
			d.retain(Datetime)

			// This mungeFn differs from kind.Date and kind.Time in that
			// it returns a time.Time instead of a string
			d.mungeFns[Datetime] = func(val interface{}) (interface{}, error) {
				if val == nil {
					return nil, nil
				}

				s, ok := val.(string)
				if !ok {
					return nil, errz.Errorf("expected %T to be string", val)
				}

				if s == "" {
					return nil, nil
				}

				t, err := time.Parse(format, s)
				if err != nil {
					return nil, errz.Err(err)
				}

				return t, nil
			}
		}
	}
}

// Detect returns the detected Kind. If ambiguous, Text is returned,
// unless all sampled values were nil, in which case Null is returned.
// If the returned mungeFn is non-nil, it can be used to convert input
// values to their canonical form. For example, for Datetime the MungeFunc
// would accept string "2020-06-11T02:50:54Z" and return a time.Time,
// while for Date, the MungeFunc would accept "1970-01-01" or "01 Jan 1970"
// and always return a string in the canonicalized form "1970-01-01".
func (d *Detector) Detect() (kind Kind, mungeFn MungeFunc, err error) {
	if !d.dirty {
		if d.foundString {
			return Text, nil, nil
		}

		// If we haven't filtered any kinds, default to Text.
		return Null, nil, nil
	}

	switch len(d.kinds) {
	case 0:
		return Text, nil, nil
	case 1:
		for k := range d.kinds {
			return k, d.mungeFns[k], nil
		}
	}

	// NOTE: this logic below about detecting the remaining type
	//  is a bit sketchy. If you're debugging this code, it's
	//  probably the case that the code below is faulty.
	if d.has(Time, Date, Datetime) {
		// If all three time types are left, use the most
		// general, i.e. Datetime.
		return Datetime, d.mungeFns[Datetime], nil
	}

	if d.has(Time) {
		return Time, d.mungeFns[Time], nil
	}

	if d.has(Date) {
		return Date, d.mungeFns[Date], nil
	}

	if d.has(Datetime) {
		return Datetime, d.mungeFns[Datetime], nil
	}

	if d.foundString && d.has(Decimal) {
		return Decimal, nil, nil
	}

	if d.has(Int) {
		return Int, nil, nil
	}

	if d.has(Float) {
		return Float, nil, nil
	}

	if d.has(Bool) {
		return Bool, nil, nil
	}

	return Text, nil, nil
}

// delete deletes each of kinds from kd.kinds
func (d *Detector) delete(kinds ...Kind) {
	d.dirty = true
	for _, k := range kinds {
		delete(d.kinds, k)
	}
}

// retain deletes everything from kd.kinds except items
// contains in the kinds arg. If kinds is empty, kd.kinds is
// be emptied.
func (d *Detector) retain(kinds ...Kind) {
	d.dirty = true
	for k := range d.kinds {
		if !containsKind(k, kinds...) {
			delete(d.kinds, k)
		}
	}
}

// has returns true if kd.kinds contains each of k.
func (d *Detector) has(kinds ...Kind) bool {
	var ok bool
	for _, k := range kinds {
		if _, ok = d.kinds[k]; !ok {
			return false
		}
	}

	return true
}

func detectKindTime(s string) (ok bool, format string) {
	if s == "" {
		return false, ""
	}

	const timeNoSecsFormat = "15:04"
	formats := []string{stringz.TimeFormat, timeNoSecsFormat, time.Kitchen}
	var err error

	for _, f := range formats {
		if _, err = time.Parse(f, s); err == nil {
			return true, f
		}
	}

	return false, ""
}

func detectKindDate(s string) (ok bool, format string) {
	if s == "" {
		return false, ""
	}

	const (
		format1 = "02 Jan 2006"
		format2 = "2006/01/02"
		format3 = "2006-01-02"
	)

	formats := []string{stringz.DateFormat, format1, format2, format3}
	var err error

	for _, f := range formats {
		if _, err = time.Parse(f, s); err == nil {
			return true, f
		}
	}

	return false, ""
}

func detectKindDatetime(s string) (ok bool, format string) {
	if s == "" {
		return false, ""
	}

	formats := []string{
		stringz.DatetimeFormat, // RFC3339Nano
		time.ANSIC,
		time.UnixDate,
		time.RubyDate,
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339,
		time.Stamp,
		time.StampMilli,
		time.StampMicro,
		time.StampNano,
	}
	var err error

	for _, f := range formats {
		if _, err = time.Parse(f, s); err == nil {
			return true, f
		}
	}

	return false, ""
}

func containsKind(needle Kind, haystack ...Kind) bool {
	for _, k := range haystack {
		if k == needle {
			return true
		}
	}

	return false
}

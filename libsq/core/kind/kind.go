// Package kind encapsulate data kind, that is data types.
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
	case Unknown:
		name = "unknown"
	case Null:
		name = "null"
	case Text:
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
		return Unknown, errz.Errorf("unrecognized kind name %q", text)
	case "unknown":
		return Unknown, nil
	case "text":
		return Text, nil
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
		return Null, nil
	}
}

// Detector is used to detect the kind of a stream of values.
// The caller adds values via Sample and then invokes Detect.
type Detector struct {
	kinds       map[Kind]struct{}
	mungeFns    map[Kind]func(interface{}) (interface{}, error)
	dirty       bool
	foundString bool
}

// NewDetector returns a new instance.
func NewDetector() *Detector {
	return &Detector{
		kinds: map[Kind]struct{}{
			KindInt:      {},
			KindFloat:    {},
			KindDecimal:  {},
			KindBool:     {},
			KindTime:     {},
			KindDate:     {},
			KindDatetime: {},
		},
		mungeFns: map[Kind]func(interface{}) (interface{}, error){},
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
		d.retain(KindFloat, KindDecimal)
		return
	case int, int8, int16, int32, int64:
		d.retain(KindInt, KindFloat, KindDecimal)
		return
	case bool:
		d.retain(KindBool)
		return
	case time.Time:
		d.retain(KindTime, KindDate, KindDatetime)
	case stdj.Number:
		// JSON number
		d.foundString = true
		d.retain(KindDecimal)
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

	if d.has(KindDecimal) {
		// If KindDecimal is still a candidate, check that we can parse it
		if _, _, err = big.ParseFloat(s, 10, 64, 0); err != nil {
			// If s cannot be parsed as a decimal, it also can't
			// be int or float
			d.delete(KindDecimal, KindInt, KindFloat)
		} else {
			// s can be parsed as decimal, can't be time
			d.delete(KindTime, KindDate, KindDatetime)
		}
	}

	if d.has(KindInt) {
		if _, err = strconv.ParseInt(s, 10, 64); err != nil {
			d.delete(KindInt)
		} else {
			// s can be parsed as int, can't be time
			d.delete(KindTime, KindDate, KindDatetime)
		}
	}

	if d.has(KindFloat) {
		if _, err = strconv.ParseFloat(s, 64); err != nil {
			d.delete(KindFloat)
		} else {
			// s can be parsed as float, can't be time
			d.delete(KindTime, KindDate, KindDatetime)
		}
	}

	if d.has(KindBool) {
		if _, err = stringz.ParseBool(s); err != nil {
			d.delete(KindBool)
		} else {
			// s can be parsed as bool, can't be time,
			// but still could be int ("1" == true)
			d.delete(KindFloat, KindTime, KindDate, KindDatetime)
		}
	}

	if d.has(KindTime) {
		ok, format := detectKindTime(s)
		if !ok {
			// It's not a recognized time format
			d.delete(KindTime)
		} else {
			// If it's KindTime, it can't be anything else
			d.retain(KindTime)

			d.mungeFns[KindTime] = func(val interface{}) (interface{}, error) {
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

				return t.Format(format), nil
			}
		}
	}

	if d.has(KindDate) {
		ok, format := detectKindDate(s)
		if !ok {
			// It's not a recognized date format
			d.delete(KindDate)
		} else {
			// If it's KindDate, it can't be anything else
			d.retain(KindDate)

			d.mungeFns[KindDate] = func(val interface{}) (interface{}, error) {
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

				return t.Format(format), nil
			}
		}
	}

	if d.has(KindDatetime) {
		ok, format := detectKindDatetime(s)
		if !ok {
			// It's not a recognized datetime format
			d.delete(KindDatetime)
		} else {
			// If it's KindDatetime, it can't be anything else
			d.retain(KindDatetime)

			// This mungeFn differs from KindDate and KindTime in that
			// it returns a time.Time instead of a string
			d.mungeFns[KindDatetime] = func(val interface{}) (interface{}, error) {
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

// Detect returns the detected Kind. If ambiguous, Text is returned.
// If the returned mungeFn is non-nil, it can be used to convert input
// values to their canonical form. For example for Datetime mungeFn
// would accept string "2020-06-11T02:50:54Z" and return a time.Time,
// while for Date, mungeFn would accept "1970-01-01" or "01 Jan 1970"
// and always return a string in the canonicalized form "1970-01-01".
func (d *Detector) Detect() (kind Kind, mungeFn func(interface{}) (interface{}, error), err error) {
	if !d.dirty {
		// If we haven't filtered any kinds, default to Text.
		return Text, nil, nil
	}

	switch len(d.kinds) {
	case 0:
		return Text, nil, nil
	case 1:
		for k := range d.kinds {
			return k, d.mungeFns[k], nil
		}
	default:
	}

	if d.has(KindTime) {
		return KindTime, d.mungeFns[KindTime], nil
	}

	if d.has(KindDate) {
		return KindDate, d.mungeFns[KindDate], nil
	}

	if d.has(KindDatetime) {
		return KindDatetime, d.mungeFns[KindDatetime], nil
	}

	if d.foundString && d.has(KindDecimal) {
		return KindDecimal, nil, nil
	}

	if d.has(KindInt) {
		return KindInt, nil, nil
	}

	if d.has(KindFloat) {
		return KindFloat, nil, nil
	}

	if d.has(KindBool) {
		return KindBool, nil, nil
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

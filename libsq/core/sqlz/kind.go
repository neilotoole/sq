package sqlz

import (
	stdj "encoding/json"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
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

// KindDetector is used to detect the kind of a stream of values.
// The caller adds values via Sample and then invokes Detect.
type KindDetector struct {
	kinds       map[Kind]struct{}
	mungeFns    map[Kind]func(interface{}) (interface{}, error)
	dirty       bool
	foundString bool
}

// NewKindDetector returns a new instance.
func NewKindDetector() *KindDetector {
	return &KindDetector{
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
func (kd *KindDetector) Sample(v interface{}) {
	switch v.(type) {
	case nil:
		// Can't glean any info from nil
		return
	default:
		// Don't know what this, so delete all kinds
		kd.retain()
		return
	case float32, float64:
		kd.retain(KindFloat, KindDecimal)
		return
	case int, int8, int16, int32, int64:
		kd.retain(KindInt, KindFloat, KindDecimal)
		return
	case bool:
		kd.retain(KindBool)
		return
	case time.Time:
		kd.retain(KindTime, KindDate, KindDatetime)
	case stdj.Number:
		// JSON number
		kd.foundString = true
		kd.retain(KindDecimal)
		return
	case string:
		// We need to do more work to figure out the kind when
		// we're getting string values
		kd.foundString = true
	}

	// We're dealing with a string value, which could a variety
	// of things, such as: "1", "1.0", "true", "11:30".
	s := v.(string)

	if s == "" {
		// Can't really do anything useful with this
		return
	}

	var err error

	if kd.has(KindDecimal) {
		// If KindDecimal is still a candidate, check that we can parse it
		if _, _, err = big.ParseFloat(s, 10, 64, 0); err != nil {
			// If s cannot be parsed as a decimal, it also can't
			// be int or float
			kd.delete(KindDecimal, KindInt, KindFloat)
		} else {
			// s can be parsed as decimal, can't be time
			kd.delete(KindTime, KindDate, KindDatetime)
		}
	}

	if kd.has(KindInt) {
		if _, err = strconv.ParseInt(s, 10, 64); err != nil {
			kd.delete(KindInt)
		} else {
			// s can be parsed as int, can't be time
			kd.delete(KindTime, KindDate, KindDatetime)
		}
	}

	if kd.has(KindFloat) {
		if _, err = strconv.ParseFloat(s, 64); err != nil {
			kd.delete(KindFloat)
		} else {
			// s can be parsed as float, can't be time
			kd.delete(KindTime, KindDate, KindDatetime)
		}
	}

	if kd.has(KindBool) {
		if _, err = stringz.ParseBool(s); err != nil {
			kd.delete(KindBool)
		} else {
			// s can be parsed as bool, can't be time,
			// but still could be int ("1" == true)
			kd.delete(KindFloat, KindTime, KindDate, KindDatetime)
		}
	}

	if kd.has(KindTime) {
		ok, format := detectKindTime(s)
		if !ok {
			// It's not a recognized time format
			kd.delete(KindTime)
		} else {
			// If it's KindTime, it can't be anything else
			kd.retain(KindTime)

			kd.mungeFns[KindTime] = func(val interface{}) (interface{}, error) {
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

	if kd.has(KindDate) {
		ok, format := detectKindDate(s)
		if !ok {
			// It's not a recognized date format
			kd.delete(KindDate)
		} else {
			// If it's KindDate, it can't be anything else
			kd.retain(KindDate)

			kd.mungeFns[KindDate] = func(val interface{}) (interface{}, error) {
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

	if kd.has(KindDatetime) {
		ok, format := detectKindDatetime(s)
		if !ok {
			// It's not a recognized datetime format
			kd.delete(KindDatetime)
		} else {
			// If it's KindDatetime, it can't be anything else
			kd.retain(KindDatetime)

			// This mungeFn differs from KindDate and KindTime in that
			// it returns a time.Time instead of a string
			kd.mungeFns[KindDatetime] = func(val interface{}) (interface{}, error) {
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

// Detect returns the detected Kind. If ambiguous, KindText is returned.
// If the returned mungeFn is non-nil, it can be used to convert input
// values to their canonical form. For example for KindDatetime mungeFn
// would accept string "2020-06-11T02:50:54Z" and return a time.Time,
// while for KindDate, mungeFn would accept "1970-01-01" or "01 Jan 1970"
// and always return a string in the canonicalized form "1970-01-01".
func (kd *KindDetector) Detect() (kind Kind, mungeFn func(interface{}) (interface{}, error), err error) {
	if !kd.dirty {
		// If we haven't filtered any kinds, default to KindText.
		return KindText, nil, nil
	}

	switch len(kd.kinds) {
	case 0:
		return KindText, nil, nil
	case 1:
		for k := range kd.kinds {
			return k, kd.mungeFns[k], nil
		}
	default:
	}

	if kd.has(KindTime) {
		return KindTime, kd.mungeFns[KindTime], nil
	}

	if kd.has(KindDate) {
		return KindDate, kd.mungeFns[KindDate], nil
	}

	if kd.has(KindDatetime) {
		return KindDatetime, kd.mungeFns[KindDatetime], nil
	}

	if kd.foundString && kd.has(KindDecimal) {
		return KindDecimal, nil, nil
	}

	if kd.has(KindInt) {
		return KindInt, nil, nil
	}

	if kd.has(KindFloat) {
		return KindFloat, nil, nil
	}

	if kd.has(KindBool) {
		return KindBool, nil, nil
	}

	return KindText, nil, nil
}

// delete deletes each of kinds from kd.kinds
func (kd *KindDetector) delete(kinds ...Kind) {
	kd.dirty = true
	for _, k := range kinds {
		delete(kd.kinds, k)
	}
}

// retain deletes everything from kd.kinds except items
// contains in the kinds arg. If kinds is empty, kd.kinds is
// be emptied.
func (kd *KindDetector) retain(kinds ...Kind) {
	kd.dirty = true
	for k := range kd.kinds {
		if !containsKind(k, kinds...) {
			delete(kd.kinds, k)
		}
	}
}

// has returns true if kd.kinds contains each of k.
func (kd *KindDetector) has(kinds ...Kind) bool {
	var ok bool
	for _, k := range kinds {
		if _, ok = kd.kinds[k]; !ok {
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

// KindScanType returns the default scan type for kind. The returned
// type is typically a sql.NullType.
func KindScanType(knd Kind) reflect.Type {
	switch knd {
	default:
		return RTypeNullString

	case KindText, KindDecimal:
		return RTypeNullString

	case KindInt:
		return RTypeNullInt64

	case KindBool:
		return RTypeNullBool

	case KindFloat:
		return RTypeNullFloat64

	case KindBytes:
		return RTypeBytes

	case KindDatetime:
		return RTypeNullTime

	case KindDate:
		return RTypeNullTime

	case KindTime:
		return RTypeNullTime
	}
}

package kind

// MungeFunc is a function that accepts a value and returns a munged
// value with the appropriate Kind. For example, a Datetime MungeFunc
// would accept string "2020-06-11T02:50:54Z" and return a time.Time.
type MungeFunc func(any) (any, error)

var _ MungeFunc = MungeEmptyStringAsNil

// MungeEmptyStringAsNil munges v to nil if v
// is an empty string.
func MungeEmptyStringAsNil(v any) (any, error) {
	switch v := v.(type) {
	case nil:
		return nil, nil
	case *string:
		if len(*v) == 0 {
			return nil, nil
		}
	case string:
		if len(v) == 0 {
			return nil, nil
		}
	}

	return v, nil
}

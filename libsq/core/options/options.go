// Package options is the home of the Options type, used to control
// optional behavior of core types such as Source.
//
// Deprecated: use config/options instead.
package options

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// Options are optional values akin to url.Values.
type Options url.Values

// Clone returns a deep copy of o. If o is nil, nil is returned.
func (o Options) Clone() Options {
	if o == nil {
		return nil
	}

	n := Options{}
	for key, vals := range o {
		if vals == nil {
			n[key] = nil
			continue
		}

		nvals := make([]string, len(vals))
		copy(nvals, vals)
		n[key] = nvals
	}

	return n
}

// ParseOptions parses the URL-encoded options string. If allowedOpts is
// non-empty, the options are tested for basic correctness.
// See url.ParseQuery.
func ParseOptions(options string, allowedOpts ...string) (Options, error) {
	vals, err := url.ParseQuery(options)
	if err != nil {
		return nil, errz.Wrap(err, "unable to parse --opts flag: value should be in URL-encoded query format")
	}

	opts := Options(vals)

	if len(allowedOpts) > 0 {
		err := verifyOptions(opts, allowedOpts...)
		if err != nil {
			return nil, err
		}
	}

	return opts, nil
}

// Add is documented by url.Values.Add.
func (o Options) Add(key, value string) {
	url.Values(o).Add(key, value)
}

// Get is documented by url.Values.Get.
func (o Options) Get(key string) string {
	return url.Values(o).Get(key)
}

// Encode is documented by url.Values.Encode.
func (o Options) Encode() string {
	return url.Values(o).Encode()
}

const (
	// OptHasHeader is the key for a header option.
	OptHasHeader = "header"

	// OptCols is the key for a cols option.
	OptCols = "cols"

	// OptDelim the key for a delimiter option.
	OptDelim = "delim"
)

// verifyOptions returns an error if opts contains any
// illegal or unknown values. If opts or allowedOpts are empty,
// nil is returned.
func verifyOptions(opts Options, allowedOpts ...string) error {
	if len(opts) == 0 || len(allowedOpts) == 0 {
		return nil
	}

	for key := range opts {
		if !stringz.InSlice(allowedOpts, key) {
			return errz.Errorf("illegal option name %s", key)
		}
	}

	// OptHasHeader must be a bool, we can verify that here.
	if vals, ok := opts[OptHasHeader]; ok {
		if len(vals) != 1 {
			return errz.Errorf("illegal value for opt %s: only 1 value permitted", OptHasHeader)
		}

		if _, err := strconv.ParseBool(vals[0]); err != nil {
			return errz.Wrapf(err, "illegal value for opt %s", OptHasHeader)
		}
	}

	return nil
}

// HasHeader checks if src.Options has "header=true".
func HasHeader(opts Options) (header, ok bool, err error) {
	if len(opts) == 0 {
		return false, false, nil
	}

	if _, ok = opts[OptHasHeader]; !ok {
		return false, false, nil
	}
	val := opts.Get(OptHasHeader)
	if val == "" {
		return false, false, nil
	}

	header, err = strconv.ParseBool(val)
	if err != nil {
		return false, false, errz.Errorf(`option {%s}: %v`, OptHasHeader, err)
	}

	return header, true, nil
}

// GetColNames returns column names specified like "--opts=cols=A,B,C".
func GetColNames(o Options) (colNames []string, err error) {
	if len(o) == 0 {
		return nil, nil
	}

	_, ok := o[OptCols]
	if !ok {
		return nil, nil
	}

	val := strings.TrimSpace(o.Get(OptCols))
	colNames = strings.Split(val, ",")
	if val == "" || len(colNames) == 0 {
		err = errz.Errorf("option {%s}: cannot be empty", OptCols)
		return nil, err
	}

	for i := range colNames {
		colNames[i] = strings.TrimSpace(colNames[i])
		if colNames[i] == "" {
			err = errz.Errorf("option {%s}: column [%d] cannot be empty", OptCols, i)
			return nil, err
		}
	}

	return colNames, nil
}

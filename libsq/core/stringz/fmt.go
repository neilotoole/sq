package stringz

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// ParseBool is an expansion of strconv.ParseBool that also
// accepts variants of "yes" and "no" (which are bool
// representations returned by some data sources).
func ParseBool(s string) (bool, error) {
	switch s {
	default:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return b, errz.Err(err)
		}
		return b, nil
	case "1", "yes", "Yes", "YES", "y", "Y", "on", "ON":
		return true, nil
	case "0", "no", "No", "NO", "n", "N", "off", "OFF":
		return false, nil
	}
}

// FormatFloat formats f. This method exists to provide a standard
// float formatting across the codebase.
func FormatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// FormatDecimal formats d with the appropriate number of decimal
// places as defined by d's exponent.
func FormatDecimal(d decimal.Decimal) string {
	exp := d.Exponent()
	var places int32
	if exp < 0 {
		places = -exp
	}
	return d.StringFixed(places)
}

// DecimalPlaces returns the count of decimal places in d. That is to
// say, it returns the number of digits after the decimal point.
func DecimalPlaces(d decimal.Decimal) int32 {
	var places int32
	exp := d.Exponent()
	if exp < 0 {
		places = -exp
	}
	return places
}

// DecimalFloatOK returns true if d can be stored as a float64
// without losing precision.
func DecimalFloatOK(d decimal.Decimal) bool {
	sDec := d.String()
	sF := FormatFloat(d.InexactFloat64())
	return sDec == sF
}

// Plu handles the most common (English language) case of
// pluralization. With arg s being "row(s) col(s)", Plu
// returns "row col" if arg i is 1, otherwise returns "rows cols".
func Plu(s string, i int) string {
	if i == 1 {
		return strings.ReplaceAll(s, "(s)", "")
	}
	return strings.ReplaceAll(s, "(s)", "s")
}

// ByteSized returns a human-readable byte size, e.g. "2.1 MB", "3.0 TB", etc.
// TODO: replace this usage with "github.com/c2h5oh/datasize",
// or maybe https://github.com/docker/go-units/.
func ByteSized(size int64, precision int, sep string) string {
	f := float64(size)
	tpl := "%." + strconv.Itoa(precision) + "f" + sep

	switch {
	case f >= yb:
		return fmt.Sprintf(tpl+"YB", f/yb)
	case f >= zb:
		return fmt.Sprintf(tpl+"ZB", f/zb)
	case f >= eb:
		return fmt.Sprintf(tpl+"EB", f/eb)
	case f >= pb:
		return fmt.Sprintf(tpl+"PB", f/pb)
	case f >= tb:
		return fmt.Sprintf(tpl+"TB", f/tb)
	case f >= gb:
		return fmt.Sprintf(tpl+"GB", f/gb)
	case f >= mb:
		return fmt.Sprintf(tpl+"MB", f/mb)
	case f >= kb:
		return fmt.Sprintf(tpl+"KB", f/kb)
	}
	return fmt.Sprintf(tpl+"B", f)
}

const (
	_          = iota // ignore first value by assigning to blank identifier
	kb float64 = 1 << (10 * iota)
	mb
	gb
	tb
	pb
	eb
	zb
	yb
)

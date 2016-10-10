package util

import (
	"encoding/json"
	"os"

	"fmt"

	"strconv"

	"github.com/neilotoole/go-lg/lg"
)

// Errorf returns a generic error, logging the fact of its creation.
func Errorf(format string, v ...interface{}) error {
	return ErrorfN(1, format, v...)
}

// Errorf is similar to Errorf, but allows specification of calldepth.
func ErrorfN(calldepth int, format string, v ...interface{}) error {
	err := fmt.Errorf(format, v...)
	lg.Depth(1+calldepth).Warnf("error created: %s", err.Error())
	return err
}

func WrapError(err error) error {

	if err == nil {
		return nil
	}
	err2 := fmt.Errorf(err.Error())
	lg.Depth(1).Warnf("wrapping error (%T): %s", err, err2.Error())
	return err
}

// InArray returns true if the needle is present in the haystack.
func InArray(haystack []string, needle string) bool {

	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

// ByteSized returns a human-readable byte size, e.g. "2.1 MB", "3.0 TB", etc.
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

func ReverseString(input string) string {
	n := 0
	runes := make([]rune, len(input))
	for _, r := range input {
		runes[n] = r
		n++
	}
	runes = runes[0:n]
	// Reverse
	for i := 0; i < n/2; i++ {
		runes[i], runes[n-1-i] = runes[n-1-i], runes[i]
	}
	// Convert back to UTF-8.
	return string(runes)
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

func SprintJSON(value interface{}) string {

	j, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(j)
}

// FileExists return true if the file at path exists.
func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, WrapError(err)
}

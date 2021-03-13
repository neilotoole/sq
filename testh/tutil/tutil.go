// Package tutil contains basic test utilities.
package tutil

import (
	"fmt"
	"strings"
)

// Name is a convenience function for building a test name to
// pass to t.Run.
//
//  t.Run(tutil.Name("my_test", 1), func(t *testing.T) {
//
// The most common usage is with test names that are file
// paths.
//
//   tutil.Name("path/to/file") --> "path_to_file"
//
// Any element of arg that prints to empty string is skipped.
func Name(args ...interface{}) string {
	var parts []string
	var s string
	for _, a := range args {
		s = fmt.Sprintf("%v", a)
		if s == "" {
			continue
		}

		s = strings.Replace(s, "/", "_", -1)
		parts = append(parts, s)
	}

	s = strings.Join(parts, "_")
	if s == "" {
		return "empty"
	}

	return s
}

package stringz

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// TrimHeadLines trims the first n lines from s. It panics
// if n is negative. If n is zero, s is returned unchanged.
func TrimHeadLines(s string, n int) string {
	switch {
	case n < 0:
		panic(fmt.Sprintf("n must be >= 0 but was %d", n))
	case n == 0, s == "":
		return s
	}

	var (
		sc  = bufio.NewScanner(strings.NewReader(s))
		buf bytes.Buffer
		i   = -1
	)

	for sc.Scan() {
		i++
		if i < n {
			continue
		}

		if buf.Len() > 0 {
			buf.WriteRune('\n')
		}

		buf.Write(sc.Bytes())
	}

	if buf.Len() > 0 && s[len(s)-1] == '\n' {
		buf.WriteRune('\n')
	}

	return buf.String()
}

// VisitLines visits the lines of s, returning a new string built from
// applying fn to each line.
func VisitLines(s string, fn func(i int, line string) string) string {
	var sb strings.Builder

	sc := bufio.NewScanner(strings.NewReader(s))
	var line string
	for i := 0; sc.Scan(); i++ {
		line = sc.Text()
		line = fn(i, line)
		if i > 0 {
			sb.WriteRune('\n')
		}
		sb.WriteString(line)
	}

	return sb.String()
}

// IndentLines returns a new string built from indenting each line of s.
func IndentLines(s, indent string) string {
	return VisitLines(s, func(_ int, line string) string {
		return indent + line
	})
}

// LineCount returns the number of lines in r. If skipEmpty is
// true, empty lines are skipped (a whitespace-only line is not
// considered empty). If r is nil or any error occurs, -1 is returned.
func LineCount(r io.Reader, skipEmpty bool) int {
	if r == nil {
		return -1
	}

	sc := bufio.NewScanner(r)
	var i int

	if skipEmpty {
		for sc.Scan() {
			if len(sc.Bytes()) > 0 {
				i++
			}
		}

		if sc.Err() != nil {
			return -1
		}

		return i
	}

	for i = 0; sc.Scan(); i++ { //nolint:revive
		// no-op
	}

	return i
}

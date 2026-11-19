package typetest

import (
	"strings"
	"time"
)

// SpacePadded returns s padded with spaces to length n, as a fixed-length
// character column (CHAR(n), NCHAR(n)) stores it. A string already n long or
// longer is returned unchanged.
func SpacePadded(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// ZeroPadded returns b padded with zero bytes to length n, as a fixed-length
// binary column (BINARY(n)) stores it.
func ZeroPadded(b []byte, n int) []byte {
	out := make([]byte, n)
	copy(out, b)
	return out
}

// TimeOfDay returns the time.Time that several drivers read a TIME column
// holding the given time of day back as: that time on 0001-01-01 UTC.
func TimeOfDay(hour, minute, sec, nsec int) time.Time {
	return time.Date(1, time.January, 1, hour, minute, sec, nsec, time.UTC)
}

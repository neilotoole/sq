package bytez

const newline = '\n'

// TerminateNewline returns a slice whose last byte is newline, if b is
// non-empty. If b is empty or already newline-terminated, b is returned as-is.
func TerminateNewline(b []byte) []byte {
	if len(b) == 0 || b[len(b)-1] == newline {
		return b
	}

	s := make([]byte, len(b)+1)
	copy(s, b)
	s[len(b)] = newline
	return s
}

package ioz

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// Checksum is a checksum of a file.
type Checksum string

// FileChecksum returns a checksum of the file at path.
// The checksum is based on the file's name, size, mode, and
// modification time. File contents are not read.
func FileChecksum(path string) (Checksum, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", errz.Wrap(err, "calculate file checksum")
	}

	buf := bytes.Buffer{}
	buf.WriteString(fi.Name())
	buf.WriteString(strconv.FormatInt(fi.ModTime().UnixNano(), 10))
	buf.WriteString(strconv.FormatInt(fi.Size(), 10))
	buf.WriteString(strconv.FormatUint(uint64(fi.Mode()), 10))
	buf.WriteString(strconv.FormatBool(fi.IsDir()))

	sum := sha256.Sum256(buf.Bytes())
	return Checksum(fmt.Sprintf("%x", sum)), nil
}

// WriteChecksum appends a checksum line to w, including
// a newline. The format is:
//
//		<checksum>  <name>
//	 da1f14c16c09bebbc452108d9ab193541f2e96515aefcb7745fee5197c343106  file.txt
//
// Use FileChecksum to calculate a checksum, and ReadChecksums
// to read this format.
func WriteChecksum(w io.Writer, sum Checksum, name string) error {
	_, err := fmt.Fprintf(w, "%s  %s\n", sum, name)
	return errz.Err(err)
}

// WriteChecksumFile writes a single {checksum,name} to path, overwriting
// the previous contents.
//
// See: WriteChecksum.
func WriteChecksumFile(path string, sum Checksum, name string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return errz.Wrap(err, "write checksum file")
	}
	defer func() { _ = f.Close() }()
	return WriteChecksum(f, sum, name)
}

// ReadChecksumsFile reads a checksum file from path.
//
// See ReadChecksums for details.
func ReadChecksumsFile(path string) (map[string]Checksum, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errz.Err(err)
	}

	defer func() { _ = f.Close() }()

	return ReadChecksums(f)
}

// ReadChecksums reads checksums lines from r, returning a map
// of checksums keyed by name. Empty lines, and lines beginning
// with "#" (comments) are ignored. This function is the
// inverse of WriteChecksum.
func ReadChecksums(r io.Reader) (map[string]Checksum, error) {
	sums := map[string]Checksum{}

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.Contains(line, "INTEGER") { // FIXME: delete
			x := true
			_ = x
		}

		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			return nil, errz.Errorf("invalid checksum line: %q", line)
		}

		sums[parts[1]] = Checksum(parts[0])
	}

	return sums, errz.Wrap(sc.Err(), "read checksums")
}

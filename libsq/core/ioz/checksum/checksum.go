package checksum

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// Hash returns the hash of b as a hex string.
func Hash(b []byte) string {
	sum := crc32.ChecksumIEEE(b)
	return fmt.Sprintf("%x", sum)
}

// Rand returns a random checksum.
func Rand() string {
	b := make([]byte, 128)
	_, _ = rand.Read(b)
	return Hash(b)
}

// Checksum is a checksum of a file.
type Checksum string

// Write appends a checksum line to w, including
// a newline. The typical format is:
//
//	<sum>     <name>
//	3610a686  file.txt
//
// However, the checksum be any string value. Use ForFile to calculate
// a checksum, and Read to read this format.
func Write(w io.Writer, sum Checksum, name string) error {
	_, err := fmt.Fprintf(w, "%s  %s\n", sum, name)
	return errz.Err(err)
}

// WriteFile writes a single {checksum,name} to path, overwriting
// the previous contents.
//
// See: Write.
func WriteFile(path string, sum Checksum, name string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return errz.Wrap(err, "write checksum file")
	}
	err = Write(f, sum, name)
	if err == nil {
		return errz.Err(f.Close())
	}

	_ = f.Close()
	return err
}

// ReadFile reads a checksum file from path.
//
// See Read for details.
func ReadFile(path string) (map[string]Checksum, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errz.Err(err)
	}

	defer func() { _ = f.Close() }()

	return Read(f)
}

// Read reads checksums lines from r, returning a map
// of checksums keyed by name. Empty lines, and lines beginning
// with "#" (comments) are ignored. This function is the
// inverse of Write.
func Read(r io.Reader) (map[string]Checksum, error) {
	sums := map[string]Checksum{}

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			return nil, errz.Errorf("invalid checksum line: %q", line)
		}

		sums[parts[1]] = Checksum(parts[0])
	}

	return sums, errz.Wrap(sc.Err(), "read checksums")
}

// ForFile returns a checksum of the file at path.
// The checksum is based on the file's name, size, mode, and
// modification time. File contents are not read.
func ForFile(path string) (Checksum, error) {
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

	return Checksum(Hash(buf.Bytes())), nil
}

// ForHTTPHeader returns a checksum generated from URL u and
// the contents of header. If the header contains an Etag,
// that is used as the primary element. Otherwise, other
// values such as Content-Length and Last-Modified are
// considered.
//
// Deprecated: use ForHTTPResponse instead.
func ForHTTPHeader(u string, header http.Header) Checksum {
	buf := bytes.Buffer{}
	buf.WriteString(u)
	if header != nil {
		etag := header.Get("Etag")
		if etag != "" {
			buf.WriteString(etag)
		} else {
			buf.WriteString(header.Get("Content-Type"))
			buf.WriteString(header.Get("Content-Disposition"))
			buf.WriteString(header.Get("Content-Length"))
			buf.WriteString(header.Get("Last-Modified"))
		}
	}

	return Checksum(Hash(buf.Bytes()))
}

// ForHTTPResponse returns a checksum generated from the response's
// request URL and the contents of the response's header. If the header
// contains an Etag, that is used as the primary element. Otherwise,
// other values such as Content-Length and Last-Modified are considered.
func ForHTTPResponse(resp *http.Response) Checksum {
	if resp == nil {
		return ""
	}

	buf := bytes.Buffer{}
	if resp.Request != nil && resp.Request.URL != nil {
		buf.WriteString(resp.Request.URL.String())
	}
	header := resp.Header
	if header != nil {
		etag := header.Get("Etag")
		if etag != "" {
			buf.WriteString(etag)
		} else {
			buf.WriteString(header.Get("Content-Type"))
			buf.WriteString(header.Get("Content-Disposition"))
			buf.WriteString(header.Get("Content-Length"))
			buf.WriteString(header.Get("Last-Modified"))
		}
	}

	return Checksum(Hash(buf.Bytes()))
}

// Package checksum provides functions for working with checksums.
// It uses crc32 for the checksum algorithm, resulting in checksum
// values like "3af3aaad".
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
	"github.com/neilotoole/sq/libsq/core/ioz"
)

// Sum returns the hash of b as a hex string.
// If b is empty, empty string is returned.
func Sum(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	sum := crc32.ChecksumIEEE(b)
	return fmt.Sprintf("%x", sum)
}

// SumAll returns the hash of a, and all the elements
// of b, as a hex string.
func SumAll[T ~string](a T, b ...T) string {
	h := crc32.NewIEEE()
	_, _ = h.Write([]byte(a))
	for _, col := range b {
		_, _ = h.Write([]byte(col))
	}
	return fmt.Sprintf("%x", h.Sum32())
}

// Rand returns a random checksum.
func Rand() string {
	b := make([]byte, 128)
	_, _ = rand.Read(b)
	return Sum(b)
}

// Checksum is a checksum value.
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
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, ioz.RWPerms)
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

	return Checksum(Sum(buf.Bytes())), nil
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

	return Checksum(Sum(buf.Bytes()))
}

// ForHTTPResponse returns a checksum generated from the response's
// request URL and the contents of the response's header. If the header
// contains an Etag, that is used as the primary element. Otherwise,
// other values such as Content-Length and Last-Modified are considered.
//
// There's some trickiness with Etag. Note that by default, stdlib http.Client
// will set sneakily set the "Accept-Encoding: gzip" header on GET requests.
// However, this doesn't happen for HEAD requests. So, comparing a GET and HEAD
// response for the same URL may result in different checksums, because the
// server will likely return a different Etag for the gzipped response.
//
//	# With gzip
//	Etag: "069dbf690a12d5eb853feb8e04aeb49e-ssl-df"
//
//	# Without gzip
//	Etag: "069dbf690a12d5eb853feb8e04aeb49e-ssl"
//
// Note the "-ssl-df" suffix on the gzipped response. The "df" suffix is
// for "deflate".
//
// The solution here might be to always explicitly set the gzip header on all
// requests. However, when gzip is not explicitly set, the stdlib client
// transparently handles gzip compression, including on the body read end. So,
// ideally, we wouldn't change that part, so that we don't have to code for
// both compressed and uncompressed responses.
//
// Our hack for now it to trim the "-df" suffix from the Etag.
//
// REVISIT: ForHTTPResponse is no longer used. It should be removed.
func ForHTTPResponse(resp *http.Response) Checksum {
	if resp == nil {
		return ""
	}

	buf := bytes.Buffer{}
	if resp.Request != nil && resp.Request.URL != nil {
		buf.WriteString(resp.Request.URL.String() + "\n")
	}
	buf.WriteString(strconv.Itoa(int(resp.ContentLength)) + "\n")
	header := resp.Header
	if header != nil {
		buf.WriteString(header.Get("Content-Encoding") + "\n")
		etag := strings.TrimSpace(header.Get("Etag"))
		if etag != "" {
			etag = strings.TrimSuffix(etag, "-df")
			buf.WriteString(etag + "\n")
		} else {
			buf.WriteString(header.Get("Content-Type") + "\n")
			buf.WriteString(header.Get("Content-Disposition") + "\n")
			buf.WriteString(header.Get("Content-Length") + "\n")
			buf.WriteString(header.Get("Last-Modified") + "\n")
		}
	}

	return Checksum(Sum(buf.Bytes()))
}

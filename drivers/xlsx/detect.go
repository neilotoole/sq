package xlsx

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"slices"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

var _ files.TypeDetectFunc = DetectXLSX

// DetectXLSX implements files.TypeDetectFunc, returning
// TypeXLSX and a score of 1.0 if valid XLSX.
//
// Detection works by scanning for ZIP local file headers and checking
// if any entry's filename starts with "xl/", which is the hallmark of
// an XLSX (Office Open XML Spreadsheet) file. This approach is more
// reliable than magic number detection because XLSX files are ZIP
// archives, and different tools create them with varying internal
// structures that can confuse magic-number-based detection.
//
// Unlike parsing the full ZIP (which requires reading to the end for
// the central directory), this scans only the first portion of the file.
func DetectXLSX(ctx context.Context, newRdrFn files.NewReaderFunc) (detected drivertype.Type, score float32,
	err error,
) {
	// Read enough bytes to find ZIP local file headers with "xl/" entries.
	// Most XLSX files have "xl/" entries within the first 8KB, but we use
	// a larger buffer to handle files with bigger metadata or different
	// entry ordering.
	const detectBufSize = 64 * 1024

	log := lg.FromContext(ctx)
	var r io.ReadCloser
	r, err = newRdrFn(ctx)
	if err != nil {
		return drivertype.None, 0, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	buf := make([]byte, detectBufSize)
	n, err := io.ReadFull(r, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return drivertype.None, 0, errz.Err(err)
	}
	buf = buf[:n]

	if hasXLSXEntry(buf) {
		return drivertype.XLSX, 1.0, nil
	}

	return drivertype.None, 0, nil
}

// zipLocalFileHeaderSig is the ZIP local file header signature.
var zipLocalFileHeaderSig = []byte{'P', 'K', 0x03, 0x04}

// hasXLSXEntry scans buf for ZIP local file headers and returns true
// if any entry's filename starts with "xl/".
//
// ZIP local file header format:
//
//	Offset  Length  Description
//	0       4       Signature (PK\x03\x04)
//	4       2       Version needed
//	6       2       General purpose bit flag
//	8       2       Compression method
//	10      2       Last mod file time
//	12      2       Last mod file date
//	14      4       CRC-32
//	18      4       Compressed size
//	22      4       Uncompressed size
//	26      2       Filename length (n)
//	28      2       Extra field length (m)
//	30      n       Filename
//	30+n    m       Extra field
func hasXLSXEntry(buf []byte) bool {
	if len(buf) < 4 {
		return false
	}

	// Quick check: must start with ZIP signature.
	if !bytes.HasPrefix(buf, zipLocalFileHeaderSig) {
		return false
	}

	xlPrefix := []byte("xl/")
	pos := 0

	for pos+30 <= len(buf) {
		// Look for next ZIP local file header signature.
		idx := bytes.Index(buf[pos:], zipLocalFileHeaderSig)
		if idx == -1 {
			break
		}
		pos += idx

		// Need at least 30 bytes for the fixed-size header fields.
		if pos+30 > len(buf) {
			break
		}

		// Read filename length from offset 26-27 (little-endian uint16).
		filenameLen := int(binary.LittleEndian.Uint16(buf[pos+26 : pos+28]))

		// Check if we have enough bytes for the filename.
		filenameStart := pos + 30
		if filenameStart+len(xlPrefix) > len(buf) {
			break
		}

		// Check if filename starts with "xl/". We only need to examine the
		// prefix bytes, not the entire filename.
		filenamePrefix := buf[filenameStart : filenameStart+len(xlPrefix)]
		if bytes.Equal(filenamePrefix, xlPrefix) {
			return true
		}

		// Skip past the fixed header (30 bytes) and filename to efficiently
		// search for the next ZIP local file header. We could also skip the
		// extra field and compressed data, but that adds complexity for
		// data descriptor handling. Skipping header+filename is sufficient
		// for efficient detection.
		pos += 30 + filenameLen
	}

	return false
}

func detectHeaderRow(ctx context.Context, sheet *xSheet) (hasHeader bool, err error) {
	if len(sheet.sampleRows) < 2 {
		// If zero records, obviously no header row.
		// If one record... well, is there any way of determining if
		// it's a header row or not? Probably best to treat it as a data row.
		return false, nil
	}

	kinds1, _, err := detectSheetColumnKinds(sheet, 0)
	if err != nil {
		return false, err
	}
	kinds2, _, err := detectSheetColumnKinds(sheet, 1)
	if err != nil {
		return false, err
	}

	if len(kinds1) == len(kinds2) {
		return !slices.Equal(kinds1, kinds2), nil
	}

	// The rows differ in length (ragged edges). Unfortunately this does
	// happen in the real world, so we must deal with it.
	lg.FromContext(ctx).Warn("Excel sheet has ragged edges", laSheet, sheet.name)

	length := min(len(kinds1), len(kinds2))
	kinds1 = kinds1[:length]
	kinds2 = kinds2[:length]

	return !slices.Equal(kinds1, kinds2), nil
}

// detectSheetColumnKinds calculates the lowest-common-denominator kind
// for the columns of sheet. It also returns munge funcs for ingesting
// each column's data (the munge func may be nil for any column).
func detectSheetColumnKinds(sheet *xSheet, rangeStart int) ([]kind.Kind, []kind.MungeFunc, error) {
	rows := sheet.sampleRows

	if rangeStart > len(rows) {
		// Shouldn't happen
		return nil, nil, errz.Errorf("excel: sheet {%s} is empty", sheet.name)
	}

	var detectors []*kind.Detector

	for i := rangeStart; i < len(rows); i++ {
		if langz.IsSliceZeroed(rows[i]) {
			continue
		}

		for j := len(detectors); j < len(rows[i]); j++ {
			detectors = append(detectors, kind.NewDetector())
		}

		for j := range rows[i] {
			val := rows[i][j]
			detectors[j].Sample(val)
		}
	}

	kinds := make([]kind.Kind, len(detectors))
	mungeFns := make([]kind.MungeFunc, len(detectors))
	var err error

	for j := range detectors {
		if kinds[j], mungeFns[j], err = detectors[j].Detect(); err != nil {
			return nil, nil, err
		}
	}

	return kinds, mungeFns, nil
}

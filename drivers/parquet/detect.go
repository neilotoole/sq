package parquet

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// parquetMagic is the 4-byte magic marker present at the start and end of
// every valid Parquet file (the "PAR1" footer marker, per the Apache Parquet
// file format spec).
var parquetMagic = []byte{'P', 'A', 'R', '1'}

const (
	scoreHeadOnly float32 = 0.7
	scoreFull     float32 = 1.0
)

var _ files.TypeDetectFunc = DetectParquet

// DetectParquet implements files.TypeDetectFunc. It returns drivertype.Parquet
// with score 1.0 when the input has the "PAR1" magic at both byte 0 and the
// last four bytes, 0.7 when only the head matches (probably a truncated
// Parquet file; DuckDB will produce a clearer error on first query), and
// drivertype.None otherwise.
func DetectParquet(ctx context.Context, newRdrFn files.NewReaderFunc) (
	detected drivertype.Type, score float32, err error,
) {
	log := lg.FromContext(ctx)

	r1, err := newRdrFn(ctx)
	if err != nil {
		return drivertype.None, 0, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r1)

	head := make([]byte, 4)
	_, err = io.ReadFull(r1, head)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return drivertype.None, 0, nil
		}
		return drivertype.None, 0, errz.Err(err)
	}
	if !isParquetHead(head) {
		return drivertype.None, 0, nil
	}

	// Head matched. Open a fresh reader for the tail check.
	r2, err := newRdrFn(ctx)
	if err != nil {
		return drivertype.None, 0, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r2)

	tail, ok := readLastFour(r2)
	if !ok {
		return drivertype.Parquet, scoreHeadOnly, nil
	}
	if isParquetFooter(tail) {
		return drivertype.Parquet, scoreFull, nil
	}
	return drivertype.Parquet, scoreHeadOnly, nil
}

// isParquetHead reports whether b starts with the Parquet magic bytes "PAR1".
func isParquetHead(b []byte) bool {
	return len(b) >= 4 && bytes.Equal(b[:4], parquetMagic)
}

// isParquetFooter reports whether b's last four bytes are the Parquet magic
// "PAR1".
func isParquetFooter(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	return bytes.Equal(b[len(b)-4:], parquetMagic)
}

// maxNonSeekDrain caps how many bytes readLastFour reads from a non-seekable
// reader while scanning for the footer marker. Above this, readLastFour
// returns (nil, false) and the caller falls back to a head-only detection
// score. The cap exists so that detecting a multi-GB Parquet over a
// streaming HTTP body does not silently download the entire object during
// `sq add`; we'd rather report "probably parquet, tail not confirmed" than
// pay that cost.
const maxNonSeekDrain = 1 << 20 // 1 MiB.

// readLastFour returns the last four bytes of r. If r implements io.Seeker,
// it seeks to (-4, end). Otherwise it drains r, retaining only the most
// recent four bytes seen, up to maxNonSeekDrain bytes. Returns (nil, false)
// on error, when r has fewer than four bytes total, or when the non-seekable
// drain hits the cap before EOF.
func readLastFour(r io.Reader) ([]byte, bool) {
	if seeker, ok := r.(io.Seeker); ok {
		if _, err := seeker.Seek(-4, io.SeekEnd); err == nil {
			tail := make([]byte, 4)
			if _, err := io.ReadFull(r, tail); err == nil {
				return tail, true
			}
		}
		// Seek or read failed (e.g. stream not seekable from end); fall through
		// to the sliding-window path.
	}

	// Sliding 4-byte window across the stream, capped at maxNonSeekDrain.
	var window [4]byte
	buf := make([]byte, 4096)
	have, total := 0, 0
	for {
		n, err := r.Read(buf)
		if n > 0 {
			total += n
			// Append to window: keep the most recent 4 bytes.
			combined := append(window[:have], buf[:n]...)
			if len(combined) >= 4 {
				copy(window[:], combined[len(combined)-4:])
				have = 4
			} else {
				copy(window[:], combined)
				have = len(combined)
			}
			if total > maxNonSeekDrain {
				// Cap reached before EOF: caller falls back to head-only.
				return nil, false
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, false
		}
	}
	if have < 4 {
		return nil, false
	}
	return window[:], true
}

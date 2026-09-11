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

	tail, ok, err := readLastFour(r2)
	if err != nil {
		return drivertype.None, 0, errz.Err(err)
	}
	if !ok {
		log.Debug("parquet: tail not confirmed (short file or drain cap); using head-only score")
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
// reports ok=false and the caller falls back to a head-only detection
// score. The cap exists so that detecting a multi-GB Parquet over a
// streaming HTTP body does not silently download the entire object during
// `sq add`; we'd rather report "probably parquet, tail not confirmed" than
// pay that cost.
const maxNonSeekDrain = 1 << 20 // 1 MiB.

// readLastFour returns the last four bytes of r. If r implements io.Seeker,
// it seeks to (-4, end). Otherwise it drains r, retaining only the most
// recent four bytes seen, up to maxNonSeekDrain bytes. It returns ok=false
// (with nil err) when r has fewer than four bytes total or when the
// non-seekable drain hits the cap before EOF; a genuine read error is
// returned in err so the caller can distinguish an I/O failure from a short
// file.
func readLastFour(r io.Reader) (tail []byte, ok bool, err error) {
	if seeker, isSeeker := r.(io.Seeker); isSeeker {
		if _, err := seeker.Seek(-4, io.SeekEnd); err == nil {
			tail := make([]byte, 4)
			if _, err := io.ReadFull(r, tail); err == nil {
				return tail, true, nil
			}
		}
		// Seek or read failed (e.g. stream not seekable from end); fall through
		// to the sliding-window path.
	}

	// Sliding 4-byte window across the stream, capped at maxNonSeekDrain.
	// The window is updated in place: per iteration we never allocate.
	var window [4]byte
	buf := make([]byte, 4096)
	have, total := 0, 0
	for {
		n, err := r.Read(buf)
		if n > 0 {
			total += n
			updateSlidingWindow(&window, &have, buf[:n])
			if total > maxNonSeekDrain {
				// Cap reached before EOF: caller falls back to head-only.
				return nil, false, nil
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, false, err
		}
	}
	if have < 4 {
		return nil, false, nil
	}
	return window[:], true, nil
}

// updateSlidingWindow folds the next chunk into a fixed 4-byte tail window.
// The window holds the most recent min(have+len(chunk), 4) bytes of the
// cumulative stream. *have is the count of valid bytes in *window before
// the call (0..4); the call updates *window and *have in place. No
// allocation.
func updateSlidingWindow(window *[4]byte, have *int, chunk []byte) {
	n := len(chunk)
	switch {
	case n == 0:
		// nothing to fold in.
	case *have+n <= 4:
		// Still filling: append.
		copy(window[*have:*have+n], chunk)
		*have += n
	case n >= 4:
		// Chunk alone overwrites the window.
		copy(window[:], chunk[n-4:])
		*have = 4
	default:
		// Cross-boundary case: shift the kept portion of window left,
		// then write chunk at the right. copy() handles the in-window
		// overlap correctly.
		keep := 4 - n
		copy(window[:keep], window[*have-keep:*have])
		copy(window[keep:], chunk)
		*have = 4
	}
}

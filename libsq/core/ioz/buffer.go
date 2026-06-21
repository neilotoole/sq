package ioz

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"

	"github.com/djherbis/buffer"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// errBufferClosed is returned by [Buffer.Write] after the buffer is closed.
var errBufferClosed = errors.New("buffer is closed")

// Buffer extracts the methods of [bytes.Buffer] to allow for alternative
// buffering strategies, such as file-backed buffers for large files. It also
// adds a [Buffer.Close] method that the caller MUST invoke when done.
//
// A Buffer is not safe for concurrent use: a single goroutine should write to,
// read from, and close it. After Close is called the Buffer must not be used.
// For safety, post-close calls do not panic (Read returns [io.EOF], Write
// returns an error, Len and Cap return 0, Reset is a no-op), but callers must
// not rely on any particular post-close behavior.
type Buffer interface {
	io.Reader
	io.Writer

	// Len returns the number of bytes of the unread portion of the buffer;
	Len() int64

	// Cap returns the capacity of the buffer.
	Cap() int64

	// Reset resets the buffer to be empty.
	Reset()

	// Close MUST be invoked when done with the buffer, or resources may leak.
	Close() error
}

// NewDefaultBuffer returns a [Buffer] backed by a [bytes.Buffer].
var NewDefaultBuffer = func() Buffer {
	return &bytesBuffer{buf: &bytes.Buffer{}}
}

var _ Buffer = (*bytesBuffer)(nil)

// bytesBuffer adapts [bytes.Buffer] to the [Buffer] interface used by
// pkg djherbis/buffer, as see in the [Buffers] type. After Close, buf is nil
// and all methods are no-ops returning zero values (or an error from Write).
type bytesBuffer struct {
	buf *bytes.Buffer
}

func (b *bytesBuffer) Read(p []byte) (n int, err error) {
	if b.buf == nil {
		return 0, io.EOF
	}
	return b.buf.Read(p)
}

func (b *bytesBuffer) Write(p []byte) (n int, err error) {
	if b.buf == nil {
		return 0, errz.Err(errBufferClosed)
	}
	return b.buf.Write(p)
}

func (b *bytesBuffer) Reset() {
	if b.buf != nil {
		b.buf.Reset()
	}
}

func (b *bytesBuffer) Close() error {
	if b.buf != nil {
		b.buf.Reset()
		b.buf = nil
	}
	return nil
}

func (b *bytesBuffer) Cap() int64 {
	if b.buf == nil {
		return 0
	}
	return int64(b.buf.Cap())
}

func (b *bytesBuffer) Len() int64 {
	if b.buf == nil {
		return 0
	}
	return int64(b.buf.Len())
}

// NewBuffers returns a new [Buffers] instance. The caller must invoke
// [Buffers.Close] when done. Arg dir is the directory in which file-backed
// buffers will be stored; the dir will be created if necessary, and is deleted
// by [Buffers.Close]. Arg memBufSize is the maximum size of in-memory buffers;
// if a buffer exceeds this size, it will be backed by a file.
func NewBuffers(dir string, memBufSize int) (*Buffers, error) {
	if err := RequireDir(dir); err != nil {
		return nil, err
	}

	bf := &Buffers{
		dir:            dir,
		spillThreshold: int64(memBufSize),
		fileBufPool:    buffer.NewFilePool(math.MaxInt, dir),
	}

	return bf, nil
}

// Buffers is a factory for creating buffers that overflow to disk. This is
// useful when dealing with large data that may not fit in memory.
type Buffers struct {
	fileBufPool    buffer.Pool
	dir            string
	spillThreshold int64
}

// Close removes the directory used for file-backed buffers.
func (bs *Buffers) Close() error {
	return errz.Wrap(os.RemoveAll(bs.dir), "failed to remove file buffer dir")
}

// NewMem2Disk returns a [Buffer] whose head is in-memory, but overflows to disk
// when it reaches a threshold. No file is created unless and until the data
// exceeds the in-memory threshold. The caller MUST invoke [Buffer.Close] when
// done, or resources may be leaked.
func (bs *Buffers) NewMem2Disk() Buffer {
	lz := &lazyFileBuffer{pool: bs.fileBufPool}
	chain := buffer.NewMulti(buffer.New(bs.spillThreshold), lz)
	return &mem2DiskBuffer{fileBuf: lz, chain: chain}
}

var _ Buffer = (*mem2DiskBuffer)(nil)

// mem2DiskBuffer is a [Buffer] backed by an in-memory head that overflows to a
// lazily-created file. It is not safe for concurrent use.
type mem2DiskBuffer struct {
	fileBuf *lazyFileBuffer
	chain   buffer.Buffer
	closed  bool
}

func (m *mem2DiskBuffer) Read(p []byte) (n int, err error) {
	if m.closed {
		return 0, io.EOF
	}
	return m.chain.Read(p)
}

func (m *mem2DiskBuffer) Write(p []byte) (n int, err error) {
	if m.closed {
		return 0, errz.Err(errBufferClosed)
	}
	return m.chain.Write(p)
}

func (m *mem2DiskBuffer) Len() int64 {
	if m.closed {
		return 0
	}
	return m.chain.Len()
}

func (m *mem2DiskBuffer) Cap() int64 {
	if m.closed {
		return 0
	}
	return m.chain.Cap()
}

func (m *mem2DiskBuffer) Reset() {
	if !m.closed {
		m.chain.Reset()
	}
}

// Close returns the file-backed buffer (if any was created) to the pool. It is
// idempotent.
func (m *mem2DiskBuffer) Close() error {
	if m.closed {
		return nil
	}
	m.closed = true

	if m.fileBuf.buf == nil {
		return nil
	}
	err := m.fileBuf.pool.Put(m.fileBuf.buf)
	m.fileBuf.buf = nil
	return errz.Err(err)
}

var _ buffer.Buffer = (*lazyFileBuffer)(nil)

// lazyFileBuffer is a [buffer.Buffer] that lazily acquires a file-backed buffer
// from pool on first write. Until then it behaves as an empty buffer, so that
// measuring (Len/Cap) or reading an unspilled buffer never creates a file on
// disk. It is not safe for concurrent use; see [Buffer].
type lazyFileBuffer struct {
	pool    buffer.Pool
	buf     buffer.Buffer
	initErr error
}

// ensure acquires the file buffer from the pool on first call. After a failed
// acquisition it returns the same error on every subsequent call without
// retrying. It is invoked only from Write, so measuring or reading an unspilled
// buffer never creates a file.
func (lz *lazyFileBuffer) ensure() error {
	if lz.buf != nil || lz.initErr != nil {
		return lz.initErr
	}
	lz.buf, lz.initErr = lz.pool.Get()
	return lz.initErr
}

func (lz *lazyFileBuffer) Len() int64 {
	if lz.buf == nil {
		return 0
	}
	return lz.buf.Len()
}

func (lz *lazyFileBuffer) Cap() int64 {
	if lz.buf == nil {
		// Not yet spilled: report the capacity it would have (the file pool is
		// sized at math.MaxInt) without forcing the file to be created.
		return math.MaxInt64
	}
	return lz.buf.Cap()
}

func (lz *lazyFileBuffer) Read(p []byte) (n int, err error) {
	if lz.buf == nil {
		return 0, io.EOF
	}
	return lz.buf.Read(p)
}

func (lz *lazyFileBuffer) Write(p []byte) (n int, err error) {
	if err = lz.ensure(); err != nil {
		return 0, errz.Err(err)
	}
	return lz.buf.Write(p)
}

func (lz *lazyFileBuffer) Reset() {
	if lz.buf != nil {
		lz.buf.Reset()
	}
}

package ioz

import (
	"bytes"
	"io"
	"math"
	"os"
	"sync"

	"github.com/djherbis/buffer"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// Buffer extracts the methods of [bytes.Buffer] to allow for alternative
// buffering strategies, such as file-backed buffers for large files. It also
// adds a [Buffer.Close] method that the caller MUST invoke when done.
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
// pkg djherbis/buffer, as see in the [Buffers] type.
type bytesBuffer struct {
	buf *bytes.Buffer
}

func (b *bytesBuffer) Read(p []byte) (n int, err error) {
	return b.buf.Read(p)
}

func (b *bytesBuffer) Write(p []byte) (n int, err error) {
	return b.buf.Write(p)
}

func (b *bytesBuffer) Reset() {
	b.buf.Reset()
}

func (b *bytesBuffer) Close() error {
	b.buf.Reset()
	b.buf = nil
	return nil
}

func (b *bytesBuffer) Cap() int64 {
	return int64(b.buf.Cap())
}

func (b *bytesBuffer) Len() int64 {
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

	// Now is a good time to test if the pool works.
	if f, err := bf.fileBufPool.Get(); err != nil {
		return nil, errz.Wrapf(err, "failed to get file buffer from pool")
	} else {
		f.Reset()
	}

	b2 := bf.NewMem2Disk()
	_, err := io.Copy(b2, LimitRandReader(100000)) // FIXME: delete
	if err != nil {
		return nil, errz.Err(err)
	}
	b2.Reset()

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
// when it reaches a threshold. The caller MUST invoke [Buffer.Close] when done,
// or resources may be leaked.
func (bs *Buffers) NewMem2Disk() Buffer {
	lz := &lazyFileBuffer{pool: bs.fileBufPool}
	multi := buffer.NewMulti(buffer.New(bs.spillThreshold), lz)
	return &mem2DiskBuffer{fileBuf: lz, Buffer: multi}
}

var _ Buffer = (*mem2DiskBuffer)(nil)

type mem2DiskBuffer struct {
	fileBuf *lazyFileBuffer
	buffer.Buffer
}

func (m *mem2DiskBuffer) Close() error {
	if m.fileBuf.buf != nil {
		return errz.Err(m.fileBuf.pool.Put(m.fileBuf.buf))
	}
	return nil
}

var _ buffer.Buffer = (*lazyFileBuffer)(nil)

// lazyFileBuffer is a [Buffer] that lazily initializes a file-backed buffer.
type lazyFileBuffer struct {
	pool buffer.Pool
	buf  buffer.Buffer
	once sync.Once
}

func (lz *lazyFileBuffer) init() {
	lz.once.Do(func() {
		var err error
		if lz.buf, err = lz.pool.Get(); err != nil {
			// Shouldn't happen
			panic(errz.Wrap(err, "failed to get file buffer from pool"))
		}
	})
}

func (lz *lazyFileBuffer) Len() int64 {
	lz.init()
	return lz.buf.Len()
}

func (lz *lazyFileBuffer) Cap() int64 {
	lz.init()
	return lz.buf.Cap()
}

func (lz *lazyFileBuffer) Read(p []byte) (n int, err error) {
	lz.init()
	return lz.buf.Read(p)
}

func (lz *lazyFileBuffer) Write(p []byte) (n int, err error) {
	lz.init()
	return lz.buf.Write(p)
}

func (lz *lazyFileBuffer) Reset() {
	if lz.buf != nil {
		lz.buf.Reset()
	}
}

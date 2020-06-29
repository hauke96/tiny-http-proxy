package stream

import (
	"io"
	"sync"
)

// Buffer is an in-memory stream which supports one writer and multiple readers.
// The stream is backed by a byte slice and concurrency is handled by an RWLock.
type Buffer struct {
	buf []byte
	rw  *sync.RWMutex
}

// bufferReader represents a single reader of a Buffer. It essentialy is an
// index into the Buffer which indicates the current position from read data
// from. Multiple instances of bufferReader can read concurrently from the
// underlying Buffer
type bufferReader struct {
	buffer *Buffer
	i      int64
}

// NewBuffer returns an empty Buffer backed by a byte slice of initial length 0
// and default capacity
func NewBuffer() Source {
	return &Buffer{
		buf: make([]byte, 0),
		rw:  new(sync.RWMutex),
	}
}

// Open returns a new reader into Buffer. Open may be called multiple times to
// retrieve multiple readers. All readers may read concurrently
func (b *Buffer) Open() (io.Reader, error) {
	return &bufferReader{
		buffer: b,
		i:      0,
	}, nil
}

func (b *bufferReader) Read(dst []byte) (n int, err error) {
	b.buffer.rw.RLock()
	defer b.buffer.rw.RUnlock()
	if b.i >= int64(len(b.buffer.buf)) {
		// No more data. Return EOF, Stream will wait for next write op
		return 0, io.EOF
	}
	n = copy(dst, b.buffer.buf[b.i:])
	b.i += int64(n)
	return n, nil
}

func (b *Buffer) Write(src []byte) (n int, err error) {
	b.rw.Lock()
	defer b.rw.Unlock()
	b.buf = append(b.buf, src...)
	return len(src), nil
}

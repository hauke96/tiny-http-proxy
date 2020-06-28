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
func (s *Buffer) Open() (io.Reader, error) {
	return &bufferReader{
		buffer: s,
		i:      0,
	}, nil
}

func (s *bufferReader) Read(b []byte) (n int, err error) {
	s.buffer.rw.RLock()
	defer s.buffer.rw.RUnlock()
	if s.i >= int64(len(s.buffer.buf)) {
		// No more data. Return EOF, Stream will wait for next write op
		return 0, io.EOF
	}
	n = copy(b, s.buffer.buf[s.i:])
	s.i += int64(n)
	return
}

func (s *Buffer) Write(b []byte) (n int, err error) {
	s.rw.Lock()
	defer s.rw.Unlock()
	s.buf = append(s.buf, b...)
	return len(b), nil
}

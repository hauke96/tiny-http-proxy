// Package stream implements data streams which support one writer and multiple
// concurrent readers. The Stream struct governs execution and suspension of
// processed when data is available/unavailable. The Source struct handles
// concurrent reads and writes to some data source. Streams are backed by such
// a data Source implementation which is supplied as an in-memory and file
// backed implementation
package stream

import (
	"errors"
	"io"
	"sync"
)

// New returns a new stream. A stream will have one writer and multiple readers
// which can read from the stream concurrently. The stream is backed by a source
// which is typically an in-memory slice of bytes or a file on disk. When no
// data is available for readers, readers will be suspended from execution. When
// the writer writes new data to the stream all readers are notified. Only when
// the writer explicitly declares the end of the stream (no more data to write),
// readers will begin to receive EOF on following reads.
func New(source Source) *Stream {
	return &Stream{
		Source: source,
		writer: source,
		cond:   sync.NewCond(new(sync.Mutex)),
		closed: false,
	}
}

// Source is the underlying data store for a stream. Multiple implementations
// of sources may exist. Typical implementations are in-memory and file backed
// sources. Implementation must as a bare minimum provide one writer, multiple
// readers and concurrency.
type Source interface {
	io.Writer
	Open() (io.Reader, error)
}

// Stream handles access to some concurrency safe data store (source) and
// handles suspension of execution when no data is available through a
// sync.Condition. When the stream is closed, the closed bool will be true
type Stream struct {
	Source
	writer io.Writer
	cond   *sync.Cond
	closed bool
}

// streamReader is a single instance of a reader from a stream
type streamReader struct {
	stream *Stream
	reader io.Reader
}

// NewReader returns a new reader for the stream. May be called multiple times
// and each reader may read from the stream concurrently
func (s *Stream) NewReader() (io.Reader, error) {
	r, err := s.Open()

	if err != nil {
		return nil, err
	}

	return &streamReader{
		stream: s,
		reader: r,
	}, nil
}

// Read reads data from the stream. If no data is currently available and the
// stream has not closed, the read method will block until more data is
// available. Only when the writer has declared no more data (closed = true)
// will the reader receive EOF.
func (s *streamReader) Read(p []byte) (n int, err error) {
	s.stream.cond.L.Lock()
	defer s.stream.cond.L.Unlock()
	n, err = s.reader.Read(p)

	// No errors, base case, return data
	if err == nil {
		return
	}

	// End of file reached
	for err == io.EOF {
		// If we are done writing, handle this as a normal case
		if s.stream.closed {
			return
		}
		// If partial data, return that data, mask the EOF since the writer may
		// write additional data in the future
		if n > 0 {
			return n, nil
		}
		// Else, no data was read, wait until more data is available
		s.stream.cond.Wait()
		n, err = s.reader.Read(p)
	}
	return
}

func (s *Stream) Write(data []byte) (int, error) {
	defer s.cond.Broadcast()
	return s.writer.Write(data)
}

// CloseWrite is called when the writer declares the end of the stream.
// This is a clear indication for readers, that no more data will be written to
// the stream. Following this call, readers will begin receiving EOF on calls to
// read. Readers may still join the stream after the stream is closed
func (s *Stream) CloseWrite() error {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	defer s.cond.Broadcast()
	if s.closed {
		return errors.New("Multireader closed multiple times")
	}
	s.closed = true
	return nil
}

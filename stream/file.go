package stream

import (
	"io"
	"os"
)

// File is a source for streams which is backed by a file on the file system.
// Concurrency and safety is provided by the file system, hence no need for
// mutex/locks. The write method is provided for free by *os.File.
type File struct {
	*os.File
}

// NewFile creates a new file in path which is used as the source for a stream.
// If the file cannot be created it panics.
func NewFile(path string) Source {
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	return &File{file}
}

// Open returns a new reader for the file by opening the file in read only mode.
func (f File) Open() (io.Reader, error) {
	return os.Open(f.Name())
}

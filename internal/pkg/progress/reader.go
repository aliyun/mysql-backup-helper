package progress

import "io"

// Reader wraps an io.Reader to track progress
type Reader struct {
	reader  io.Reader
	tracker *Tracker
	bufSize int
}

// NewReader creates a new progress reader
func NewReader(reader io.Reader, tracker *Tracker, bufSize int) *Reader {
	if bufSize <= 0 {
		bufSize = 64 * 1024 // 64KB default
	}
	return &Reader{
		reader:  reader,
		tracker: tracker,
		bufSize: bufSize,
	}
}

// Read implements io.Reader
func (pr *Reader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	if n > 0 {
		pr.tracker.Update(int64(n))
	}
	return n, err
}

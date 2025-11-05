package progress

import "io"

// Writer wraps an io.Writer to track progress
type Writer struct {
	writer  io.Writer
	tracker *Tracker
}

// NewWriter creates a new progress writer
func NewWriter(writer io.Writer, tracker *Tracker) *Writer {
	return &Writer{
		writer:  writer,
		tracker: tracker,
	}
}

// Write implements io.Writer
func (pw *Writer) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	if n > 0 {
		pw.tracker.Update(int64(n))
	}
	return n, err
}

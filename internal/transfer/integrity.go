package transfer

import (
	"hash/crc32"
	"io"
	"sync"
	"sync/atomic"
)

type CountingWriter struct {
	writer io.Writer
	bytes  atomic.Int64
	mu     sync.Mutex
	crc    uint32
}

func NewCountingWriter(writer io.Writer) *CountingWriter {
	return &CountingWriter{writer: writer}
}

func (w *CountingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n > 0 {
		w.bytes.Add(int64(n))
		w.mu.Lock()
		w.crc = crc32.Update(w.crc, crc32.IEEETable, p[:n])
		w.mu.Unlock()
	}
	return n, err
}

func (w *CountingWriter) BytesWritten() int64 {
	return w.bytes.Load()
}

func (w *CountingWriter) CRC32() uint32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.crc
}

type CountingReader struct {
	reader io.Reader
	bytes  atomic.Int64
}

func NewCountingReader(reader io.Reader) *CountingReader {
	return &CountingReader{reader: reader}
}

func (r *CountingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.bytes.Add(int64(n))
	}
	return n, err
}

func (r *CountingReader) BytesRead() int64 {
	return r.bytes.Load()
}

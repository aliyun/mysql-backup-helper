package ratelimit

import (
	"io"
	"sync"
	"time"
)

// Writer wraps an io.Writer with rate limiting
type Writer struct {
	writer       io.Writer
	rateLimit    int64 // bytes per second
	lastWrite    time.Time
	bytesWritten int64
	mu           sync.Mutex
}

// NewWriter creates a new rate-limited writer
func NewWriter(writer io.Writer, rateLimit int64) *Writer {
	return &Writer{
		writer:    writer,
		rateLimit: rateLimit,
		lastWrite: time.Now(),
	}
}

// UpdateRateLimit updates the rate limit dynamically
func (rlw *Writer) UpdateRateLimit(newLimit int64) {
	rlw.mu.Lock()
	rlw.rateLimit = newLimit
	rlw.mu.Unlock()
}

// GetCurrentLimit returns the current rate limit
func (rlw *Writer) GetCurrentLimit() int64 {
	rlw.mu.Lock()
	defer rlw.mu.Unlock()
	return rlw.rateLimit
}

// Write implements io.Writer with rate limiting
func (rlw *Writer) Write(p []byte) (n int, err error) {
	rlw.mu.Lock()
	rateLimit := rlw.rateLimit
	rlw.mu.Unlock()

	if rateLimit <= 0 {
		// No rate limit, write directly
		return rlw.writer.Write(p)
	}

	// Use token bucket algorithm for rate limiting
	totalWritten := 0
	for totalWritten < len(p) {
		now := time.Now()
		rlw.mu.Lock()
		elapsed := now.Sub(rlw.lastWrite).Seconds()

		// Calculate how many bytes we can write
		if elapsed > 1.0 {
			// Reset counter every second
			rlw.bytesWritten = 0
			rlw.lastWrite = now
			elapsed = 0
		}

		allowedBytes := int64(float64(rateLimit) * elapsed)
		available := allowedBytes - rlw.bytesWritten

		if available <= 0 {
			// Need to wait
			waitTime := time.Duration(-float64(available) / float64(rateLimit) * float64(time.Second))
			rlw.mu.Unlock()
			time.Sleep(waitTime)
			rlw.mu.Lock()
			now = time.Now()
			rlw.lastWrite = now
			rlw.bytesWritten = 0
			available = rateLimit
		}

		// Write as much as we can
		writeSize := len(p) - totalWritten
		if int64(writeSize) > available {
			writeSize = int(available)
		}

		rlw.mu.Unlock()

		written, writeErr := rlw.writer.Write(p[totalWritten : totalWritten+writeSize])
		totalWritten += written

		if writeErr != nil {
			return totalWritten, writeErr
		}

		rlw.mu.Lock()
		rlw.bytesWritten += int64(written)
		rlw.mu.Unlock()
	}

	return totalWritten, nil
}

// Close implements io.Closer - delegates to underlying writer if it's a Closer
func (rlw *Writer) Close() error {
	if closer, ok := rlw.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// Reader wraps an io.Reader with rate limiting
type Reader struct {
	reader    io.Reader
	rateLimit int64 // bytes per second
	lastRead  time.Time
	bytesRead int64
	mu        sync.Mutex
}

// NewReader creates a new rate-limited reader
func NewReader(reader io.Reader, rateLimit int64) *Reader {
	return &Reader{
		reader:    reader,
		rateLimit: rateLimit,
		lastRead:  time.Now(),
	}
}

// Read implements io.Reader with rate limiting
func (rlr *Reader) Read(p []byte) (n int, err error) {
	rlr.mu.Lock()
	rateLimit := rlr.rateLimit
	rlr.mu.Unlock()

	if rateLimit <= 0 {
		// No rate limit, read directly
		return rlr.reader.Read(p)
	}

	// Use token bucket algorithm for rate limiting
	totalRead := 0
	for totalRead < len(p) {
		now := time.Now()
		rlr.mu.Lock()
		elapsed := now.Sub(rlr.lastRead).Seconds()

		// Calculate how many bytes we can read
		if elapsed > 1.0 {
			// Reset counter every second
			rlr.bytesRead = 0
			rlr.lastRead = now
			elapsed = 0
		}

		allowedBytes := int64(float64(rateLimit) * elapsed)
		available := allowedBytes - rlr.bytesRead

		if available <= 0 {
			// Need to wait
			waitTime := time.Duration(-float64(available) / float64(rateLimit) * float64(time.Second))
			rlr.mu.Unlock()
			time.Sleep(waitTime)
			rlr.mu.Lock()
			now = time.Now()
			rlr.lastRead = now
			rlr.bytesRead = 0
			available = rateLimit
		}

		// Read as much as we can
		readSize := len(p) - totalRead
		if int64(readSize) > available {
			readSize = int(available)
		}

		rlr.mu.Unlock()

		read, readErr := rlr.reader.Read(p[totalRead : totalRead+readSize])
		totalRead += read

		if readErr != nil {
			return totalRead, readErr
		}

		rlr.mu.Lock()
		rlr.bytesRead += int64(read)
		rlr.mu.Unlock()

		// If we read less than requested, we're done
		if read < readSize {
			break
		}
	}

	return totalRead, nil
}

// Close implements io.Closer - delegates to underlying reader if it's a Closer
func (rlr *Reader) Close() error {
	if closer, ok := rlr.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

package rate

import (
	"io"
	"sync"
	"time"
)

// RateLimitedWriter wraps an io.Writer with rate limiting using a proper token bucket
type RateLimitedWriter struct {
	writer     io.Writer
	rateLimit  int64   // bytes per second
	tokens     float64 // Current tokens in bucket
	capacity   float64 // Bucket capacity (allow some burst for smoothness)
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewRateLimitedWriter creates a new rate-limited writer
func NewRateLimitedWriter(writer io.Writer, rateLimit int64) *RateLimitedWriter {
	// Allow burst up to 2x rate limit for smoothness
	capacity := float64(rateLimit) * 2
	return &RateLimitedWriter{
		writer:     writer,
		rateLimit:  rateLimit,
		tokens:     capacity, // Start with full bucket
		capacity:   capacity,
		lastUpdate: time.Now(),
	}
}

// UpdateRateLimit updates the rate limit dynamically
func (rlw *RateLimitedWriter) UpdateRateLimit(newLimit int64) {
	rlw.mu.Lock()
	rlw.rateLimit = newLimit
	rlw.capacity = float64(newLimit) * 2
	// Adjust tokens proportionally if needed
	if rlw.tokens > rlw.capacity {
		rlw.tokens = rlw.capacity
	}
	rlw.mu.Unlock()
}

// GetCurrentLimit returns the current rate limit
func (rlw *RateLimitedWriter) GetCurrentLimit() int64 {
	rlw.mu.Lock()
	defer rlw.mu.Unlock()
	return rlw.rateLimit
}

// Write implements io.Writer with rate limiting
func (rlw *RateLimitedWriter) Write(p []byte) (n int, err error) {
	rlw.mu.Lock()
	rateLimit := rlw.rateLimit
	rlw.mu.Unlock()

	if rateLimit <= 0 {
		// No rate limit, write directly
		return rlw.writer.Write(p)
	}

	totalWritten := 0
	for totalWritten < len(p) {
		rlw.mu.Lock()
		now := time.Now()

		// Refill tokens based on elapsed time
		elapsed := now.Sub(rlw.lastUpdate).Seconds()
		if elapsed > 0 {
			// Add tokens at rateLimit bytes per second
			rlw.tokens += float64(rateLimit) * elapsed
			if rlw.tokens > rlw.capacity {
				rlw.tokens = rlw.capacity
			}
			rlw.lastUpdate = now
		}

		// Calculate how many bytes we can write
		available := int64(rlw.tokens)
		rlw.mu.Unlock()

		if available <= 0 {
			// Need to wait for tokens
			// Calculate wait time: tokens needed / rate
			needed := float64(len(p) - totalWritten)
			if needed > rlw.capacity {
				needed = rlw.capacity
			}
			waitTime := time.Duration((needed - rlw.tokens) / float64(rateLimit) * float64(time.Second))
			if waitTime > 0 {
				// Use smaller sleep increments for better precision
				if waitTime > 100*time.Millisecond {
					time.Sleep(100 * time.Millisecond)
				} else {
					time.Sleep(waitTime)
				}
			}
			continue
		}

		// Write as much as we can
		writeSize := len(p) - totalWritten
		if int64(writeSize) > available {
			writeSize = int(available)
		}

		written, writeErr := rlw.writer.Write(p[totalWritten : totalWritten+writeSize])
		totalWritten += written

		if writeErr != nil {
			return totalWritten, writeErr
		}

		// Consume tokens
		rlw.mu.Lock()
		rlw.tokens -= float64(written)
		if rlw.tokens < 0 {
			rlw.tokens = 0
		}
		rlw.mu.Unlock()
	}

	return totalWritten, nil
}

// Close implements io.Closer - delegates to underlying writer if it's a Closer
func (rlw *RateLimitedWriter) Close() error {
	if closer, ok := rlw.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// RateLimitedReader wraps an io.Reader with rate limiting using a proper token bucket
type RateLimitedReader struct {
	reader     io.Reader
	rateLimit  int64   // bytes per second
	tokens     float64 // Current tokens in bucket
	capacity   float64 // Bucket capacity (allow some burst for smoothness)
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewRateLimitedReader creates a new rate-limited reader
func NewRateLimitedReader(reader io.Reader, rateLimit int64) *RateLimitedReader {
	// Allow burst up to 2x rate limit for smoothness
	capacity := float64(rateLimit) * 2
	return &RateLimitedReader{
		reader:     reader,
		rateLimit:  rateLimit,
		tokens:     capacity, // Start with full bucket
		capacity:   capacity,
		lastUpdate: time.Now(),
	}
}

// Read implements io.Reader with rate limiting
func (rlr *RateLimitedReader) Read(p []byte) (n int, err error) {
	rlr.mu.Lock()
	rateLimit := rlr.rateLimit
	rlr.mu.Unlock()

	if rateLimit <= 0 {
		// No rate limit, read directly
		return rlr.reader.Read(p)
	}

	totalRead := 0
	for totalRead < len(p) {
		rlr.mu.Lock()
		now := time.Now()

		// Refill tokens based on elapsed time
		elapsed := now.Sub(rlr.lastUpdate).Seconds()
		if elapsed > 0 {
			// Add tokens at rateLimit bytes per second
			rlr.tokens += float64(rateLimit) * elapsed
			if rlr.tokens > rlr.capacity {
				rlr.tokens = rlr.capacity
			}
			rlr.lastUpdate = now
		}

		// Calculate how many bytes we can read
		available := int64(rlr.tokens)
		rlr.mu.Unlock()

		if available <= 0 {
			// Need to wait for tokens
			// Calculate wait time: tokens needed / rate
			needed := float64(len(p) - totalRead)
			if needed > rlr.capacity {
				needed = rlr.capacity
			}
			waitTime := time.Duration((needed - rlr.tokens) / float64(rateLimit) * float64(time.Second))
			if waitTime > 0 {
				// Use smaller sleep increments for better precision
				if waitTime > 100*time.Millisecond {
					time.Sleep(100 * time.Millisecond)
				} else {
					time.Sleep(waitTime)
				}
			}
			continue
		}

		// Read as much as we can
		readSize := len(p) - totalRead
		if int64(readSize) > available {
			readSize = int(available)
		}

		read, readErr := rlr.reader.Read(p[totalRead : totalRead+readSize])
		totalRead += read

		if readErr != nil {
			return totalRead, readErr
		}

		// Consume tokens
		rlr.mu.Lock()
		rlr.tokens -= float64(read)
		if rlr.tokens < 0 {
			rlr.tokens = 0
		}
		rlr.mu.Unlock()

		// If we read less than requested, we're done
		if read < readSize {
			break
		}
	}

	return totalRead, nil
}

// Close implements io.Closer - delegates to underlying reader if it's a Closer
func (rlr *RateLimitedReader) Close() error {
	if closer, ok := rlr.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

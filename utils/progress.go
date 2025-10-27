package utils

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/gioco-play/easy-i18n/i18n"
)

// ProgressTracker tracks upload progress and displays real-time information
type ProgressTracker struct {
	totalBytes    int64
	uploadedBytes int64
	startTime     time.Time
	lastUpdate    time.Time
	lastBytes     int64
	isComplete    bool
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(totalBytes int64) *ProgressTracker {
	now := time.Now()
	return &ProgressTracker{
		totalBytes:    totalBytes,
		uploadedBytes: 0,
		startTime:     now,
		lastUpdate:    now,
		lastBytes:     0,
		isComplete:    false,
	}
}

// Update updates the uploaded bytes and displays progress
func (pt *ProgressTracker) Update(bytes int64) {
	atomic.AddInt64(&pt.uploadedBytes, bytes)
	pt.displayProgress()
}

// Complete marks the upload as complete and displays final statistics
func (pt *ProgressTracker) Complete() {
	pt.isComplete = true
	totalUploaded := atomic.LoadInt64(&pt.uploadedBytes)
	duration := time.Since(pt.startTime)
	avgSpeed := float64(totalUploaded) / duration.Seconds()

	fmt.Printf("\n")
	i18n.Printf("[backup-helper] Upload completed!\n")
	i18n.Printf("  Total uploaded: %s\n", formatBytes(totalUploaded))
	i18n.Printf("  Duration: %s\n", formatDuration(duration))
	i18n.Printf("  Average speed: %s/s\n", formatBytes(int64(avgSpeed)))
}

// displayProgress displays current progress
func (pt *ProgressTracker) displayProgress() {
	now := time.Now()
	uploaded := atomic.LoadInt64(&pt.uploadedBytes)

	// Only update every 500ms to avoid flooding the console
	if now.Sub(pt.lastUpdate) < 500*time.Millisecond {
		return
	}

	// Calculate speed based on last interval
	duration := now.Sub(pt.lastUpdate)
	bytesDiff := uploaded - pt.lastBytes
	speed := float64(bytesDiff) / duration.Seconds()

	// Display progress
	var progressLine string
	if pt.totalBytes > 0 {
		percentage := float64(uploaded) * 100.0 / float64(pt.totalBytes)
		progressLine = fmt.Sprintf("\rProgress: %s / %s (%.1f%%) - %s/s - Duration: %s",
			formatBytes(uploaded),
			formatBytes(pt.totalBytes),
			percentage,
			formatBytes(int64(speed)),
			formatDuration(now.Sub(pt.startTime)),
		)
	} else {
		// Unknown total size
		progressLine = fmt.Sprintf("\rProgress: %s - %s/s - Duration: %s",
			formatBytes(uploaded),
			formatBytes(int64(speed)),
			formatDuration(now.Sub(pt.startTime)),
		)
	}

	fmt.Print(progressLine)

	pt.lastUpdate = now
	pt.lastBytes = uploaded
}

// ProgressReader wraps an io.Reader to track progress
type ProgressReader struct {
	reader  io.Reader
	tracker *ProgressTracker
	bufSize int
}

// NewProgressReader creates a new progress reader
func NewProgressReader(reader io.Reader, tracker *ProgressTracker, bufSize int) *ProgressReader {
	if bufSize <= 0 {
		bufSize = 64 * 1024 // 64KB default
	}
	return &ProgressReader{
		reader:  reader,
		tracker: tracker,
		bufSize: bufSize,
	}
}

// Read implements io.Reader
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	if n > 0 {
		pr.tracker.Update(int64(n))
	}
	return n, err
}

// ProgressWriter wraps an io.Writer to track progress
type ProgressWriter struct {
	writer  io.Writer
	tracker *ProgressTracker
}

// NewProgressWriter creates a new progress writer
func NewProgressWriter(writer io.Writer, tracker *ProgressTracker) *ProgressWriter {
	return &ProgressWriter{
		writer:  writer,
		tracker: tracker,
	}
}

// Write implements io.Writer
func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	if n > 0 {
		pw.tracker.Update(int64(n))
	}
	return n, err
}

// formatBytes formats bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatDuration formats duration to human-readable format
func formatDuration(d time.Duration) string {
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	s := (d % time.Minute) / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

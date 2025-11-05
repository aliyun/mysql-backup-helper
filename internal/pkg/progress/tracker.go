package progress

import (
	"backup-helper/internal/pkg/format"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gioco-play/easy-i18n/i18n"
)

// Tracker tracks upload/download progress and displays real-time information
type Tracker struct {
	totalBytes       int64
	transferredBytes int64
	startTime        time.Time
	lastUpdate       time.Time
	lastBytes        int64
	isComplete       bool
	startOnce        sync.Once
	mode             string // "upload" or "download"
	outputToStderr   bool   // If true, output progress to stderr instead of stdout
}

// NewTracker creates a new progress tracker
func NewTracker(totalBytes int64) *Tracker {
	return &Tracker{
		totalBytes:       totalBytes,
		transferredBytes: 0,
		startTime:        time.Time{}, // Zero time, will be set on first Update
		lastUpdate:       time.Time{},
		lastBytes:        0,
		isComplete:       false,
		mode:             "upload", // default to upload
	}
}

// NewDownloadTracker creates a new progress tracker for download mode
func NewDownloadTracker(totalBytes int64) *Tracker {
	pt := NewTracker(totalBytes)
	pt.mode = "download"
	return pt
}

// SetOutputToStderr sets whether progress should be output to stderr instead of stdout
func (pt *Tracker) SetOutputToStderr(outputToStderr bool) {
	pt.outputToStderr = outputToStderr
}

// Update updates the transferred bytes and displays progress
func (pt *Tracker) Update(bytes int64) {
	// Start timer on first data transfer
	pt.startOnce.Do(func() {
		now := time.Now()
		pt.startTime = now
		pt.lastUpdate = now
	})

	atomic.AddInt64(&pt.transferredBytes, bytes)
	pt.displayProgress()
}

// Complete marks the transfer as complete and displays final statistics
func (pt *Tracker) Complete() {
	pt.isComplete = true
	totalBytes := atomic.LoadInt64(&pt.transferredBytes)

	// Use stderr if outputToStderr is true, otherwise use stdout (via fmt.Print/i18n.Printf)
	outputWriter := os.Stderr
	if !pt.outputToStderr {
		outputWriter = os.Stdout
	}

	// Clear the progress line and add a newline
	fmt.Fprint(outputWriter, "\r"+strings.Repeat(" ", 100)+"\r\n")

	// Only calculate duration if we actually started (startTime is not zero)
	if pt.startTime.IsZero() {
		// No data was transferred
		if pt.mode == "download" {
			i18n.Fprintf(outputWriter, "[backup-helper] Download completed!\n")
			i18n.Fprintf(outputWriter, "  Total downloaded: %s\n", format.Bytes(totalBytes))
		} else {
			i18n.Fprintf(outputWriter, "[backup-helper] Upload completed!\n")
			i18n.Fprintf(outputWriter, "  Total uploaded: %s\n", format.Bytes(totalBytes))
		}
		return
	}

	duration := time.Since(pt.startTime)
	avgSpeed := float64(totalBytes) / duration.Seconds()

	if pt.mode == "download" {
		i18n.Fprintf(outputWriter, "[backup-helper] Download completed!\n")
		i18n.Fprintf(outputWriter, "  Total downloaded: %s\n", format.Bytes(totalBytes))
	} else {
		i18n.Fprintf(outputWriter, "[backup-helper] Upload completed!\n")
		i18n.Fprintf(outputWriter, "  Total uploaded: %s\n", format.Bytes(totalBytes))
	}
	i18n.Fprintf(outputWriter, "  Duration: %s\n", format.Duration(duration))
	i18n.Fprintf(outputWriter, "  Average speed: %s/s\n", format.Bytes(int64(avgSpeed)))
}

// displayProgress displays current progress
func (pt *Tracker) displayProgress() {
	// Don't display if startTime hasn't been set yet
	if pt.startTime.IsZero() {
		return
	}

	now := time.Now()
	transferred := atomic.LoadInt64(&pt.transferredBytes)

	// Only update every 500ms to avoid flooding the console
	if now.Sub(pt.lastUpdate) < 500*time.Millisecond {
		return
	}

	// Calculate speed based on last interval
	duration := now.Sub(pt.lastUpdate)
	bytesDiff := transferred - pt.lastBytes
	speed := float64(bytesDiff) / duration.Seconds()

	// Display progress
	var progressLine string
	if pt.totalBytes > 0 {
		percentage := float64(transferred) * 100.0 / float64(pt.totalBytes)
		progressLine = fmt.Sprintf("\rProgress: %s / %s (%.1f%%) - %s/s - Duration: %s",
			format.Bytes(transferred),
			format.Bytes(pt.totalBytes),
			percentage,
			format.Bytes(int64(speed)),
			format.Duration(now.Sub(pt.startTime)),
		)
	} else {
		// Unknown total size
		progressLine = fmt.Sprintf("\rProgress: %s - %s/s - Duration: %s",
			format.Bytes(transferred),
			format.Bytes(int64(speed)),
			format.Duration(now.Sub(pt.startTime)),
		)
	}

	// Output to stderr if outputToStderr is true, otherwise stdout
	if pt.outputToStderr {
		fmt.Fprint(os.Stderr, progressLine)
	} else {
		fmt.Print(progressLine)
	}

	pt.lastUpdate = now
	pt.lastBytes = transferred
}

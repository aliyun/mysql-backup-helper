package utils

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gioco-play/easy-i18n/i18n"
)

// IOStats represents current IO statistics
type IOStats struct {
	UtilPercent float64 // IO utilization percentage (0-100)
	ReadIOPS    float64 // Read IOPS
	WriteIOPS   float64 // Write IOPS
	ReadBW      float64 // Read bandwidth (MB/s)
	WriteBW     float64 // Write bandwidth (MB/s)
}

// IOMonitor monitors system IO in real-time during transfer
type IOMonitor struct {
	stats          *IOStats
	isRunning      int32
	stopChan       chan struct{}
	threshold      float64              // Warning threshold for IO utilization (0-100)
	onHighIO       func(stats *IOStats) // Callback when IO is high
	currentLimit   int64                // Current dynamic rate limit (atomic)
	originalLimit  int64                // Original rate limit set by user
	adjustmentStep float64              // Step size for rate adjustment (as percentage)
}

// NewIOMonitor creates a new IO monitor with dynamic rate limiting
func NewIOMonitor(threshold float64, originalLimit int64, onHighIO func(stats *IOStats)) *IOMonitor {
	return &IOMonitor{
		stats:          &IOStats{},
		isRunning:      0,
		stopChan:       make(chan struct{}),
		threshold:      threshold,
		onHighIO:       onHighIO,
		currentLimit:   originalLimit,
		originalLimit:  originalLimit,
		adjustmentStep: 0.1, // 10% adjustment step
	}
}

// Start begins monitoring IO in the background
func (m *IOMonitor) Start(ctx context.Context, interval time.Duration) {
	if !atomic.CompareAndSwapInt32(&m.isRunning, 0, 1) {
		return // Already running
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				atomic.StoreInt32(&m.isRunning, 0)
				return
			case <-m.stopChan:
				atomic.StoreInt32(&m.isRunning, 0)
				return
			case <-ticker.C:
				stats, err := GetCurrentIOStats()
				if err != nil {
					// Silently fail, don't spam errors
					continue
				}
				m.stats = stats

				currentLimit := atomic.LoadInt64(&m.currentLimit)

				// Dynamic rate adjustment based on IO utilization
				if stats.UtilPercent > m.threshold {
					// IO is high, reduce rate limit
					newLimit := int64(float64(currentLimit) * (1.0 - m.adjustmentStep))
					// Minimum limit: 10% of original
					minLimit := int64(float64(m.originalLimit) * 0.1)
					if newLimit < minLimit {
						newLimit = minLimit
					}

					if newLimit < currentLimit {
						atomic.StoreInt64(&m.currentLimit, newLimit)
						if m.onHighIO != nil {
							m.onHighIO(stats)
						}
						i18n.Printf("\n[backup-helper] IO utilization high (%.1f%%), reducing rate limit to %s/s\n",
							stats.UtilPercent, FormatBytes(newLimit))
					}
				} else if stats.UtilPercent < m.threshold*0.7 {
					// IO is low (< 70% of threshold), gradually increase rate limit
					if currentLimit < m.originalLimit {
						newLimit := int64(float64(currentLimit) * (1.0 + m.adjustmentStep))
						if newLimit > m.originalLimit {
							newLimit = m.originalLimit
						}
						atomic.StoreInt64(&m.currentLimit, newLimit)
						if newLimit > currentLimit {
							i18n.Printf("\n[backup-helper] IO utilization low (%.1f%%), increasing rate limit to %s/s\n",
								stats.UtilPercent, FormatBytes(newLimit))
						}
					}
				}
			}
		}
	}()
}

// Stop stops the monitoring
func (m *IOMonitor) Stop() {
	if atomic.LoadInt32(&m.isRunning) == 1 {
		close(m.stopChan)
	}
}

// GetStats returns current IO statistics
func (m *IOMonitor) GetStats() *IOStats {
	return m.stats
}

// GetCurrentLimit returns the current dynamic rate limit
func (m *IOMonitor) GetCurrentLimit() int64 {
	return atomic.LoadInt64(&m.currentLimit)
}

// GetCurrentIOStats reads current IO statistics using iostat
func GetCurrentIOStats() (*IOStats, error) {
	// Try iostat -x 1 1 (Linux) or iostat -w 1 2 (macOS)
	// Linux format: device r/s w/s rkB/s wkB/s %util
	// macOS format: device r/s w/s KB/s %util

	var cmd *exec.Cmd
	var parseFunc func([]byte) (*IOStats, error)

	// Try Linux iostat first
	cmd = exec.Command("iostat", "-x", "1", "1")
	parseFunc = parseLinuxIostat

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try macOS iostat (second sample, skip first averaged since boot)
		cmd = exec.Command("iostat", "-w", "1", "2")
		parseFunc = parseMacOSIostat
		output, err = cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("iostat command failed: %v", err)
		}
	}

	return parseFunc(output)
}

func parseLinuxIostat(output []byte) (*IOStats, error) {
	lines := strings.Split(string(output), "\n")
	stats := &IOStats{}

	// Linux iostat -x output format:
	// Device r/s w/s rkB/s wkB/s rrqm/s wrqm/s r_await w_await aqu-sz rareq-sz wareq-sz svctm %util
	// Skip header lines, find device lines (usually sda, sdb, etc.)
	foundHeader := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Find the header line
		if strings.Contains(line, "Device") && strings.Contains(line, "%util") {
			foundHeader = true
			continue
		}

		if foundHeader {
			parts := strings.Fields(line)
			if len(parts) < 14 {
				continue
			}

			// Skip loop devices and special devices
			device := parts[0]
			if strings.HasPrefix(device, "loop") || strings.HasPrefix(device, "dm-") {
				continue
			}

			// Parse %util (last column)
			utilStr := parts[len(parts)-1]
			util, err := strconv.ParseFloat(utilStr, 64)
			if err == nil && util > stats.UtilPercent {
				stats.UtilPercent = util
			}

			// Parse r/s and w/s (columns 2 and 3)
			if len(parts) > 3 {
				if rIOPS, err := strconv.ParseFloat(parts[1], 64); err == nil {
					stats.ReadIOPS += rIOPS
				}
				if wIOPS, err := strconv.ParseFloat(parts[2], 64); err == nil {
					stats.WriteIOPS += wIOPS
				}
			}

			// Parse rkB/s and wkB/s (columns 4 and 5)
			if len(parts) > 5 {
				if rkB, err := strconv.ParseFloat(parts[3], 64); err == nil {
					stats.ReadBW += rkB / 1024 // Convert KB/s to MB/s
				}
				if wkB, err := strconv.ParseFloat(parts[4], 64); err == nil {
					stats.WriteBW += wkB / 1024 // Convert KB/s to MB/s
				}
			}
		}
	}

	return stats, nil
}

func parseMacOSIostat(output []byte) (*IOStats, error) {
	lines := strings.Split(string(output), "\n")
	stats := &IOStats{}

	// macOS iostat -w output format:
	// device r/s w/s KB/s ms/r ms/w %util
	// First output is average since boot, second is current activity
	// We want the last sample (current activity)

	foundHeader := false
	sampleCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is a header line
		if strings.Contains(strings.ToLower(line), "device") && strings.Contains(line, "%util") {
			foundHeader = true
			sampleCount++
			continue
		}

		// After finding header, collect data from the second sample
		if foundHeader && sampleCount >= 2 {
			parts := strings.Fields(line)
			if len(parts) < 7 {
				continue
			}

			// Skip disk0 (which is often system disk and not relevant for database)
			device := parts[0]
			if device == "disk0" || strings.HasPrefix(device, "/dev/") {
				continue
			}

			// Parse %util (last column, column 7)
			utilStr := parts[6]
			util, err := strconv.ParseFloat(utilStr, 64)
			if err == nil && util > stats.UtilPercent {
				stats.UtilPercent = util
			}

			// Parse r/s and w/s (columns 2 and 3)
			if rIOPS, err := strconv.ParseFloat(parts[1], 64); err == nil {
				stats.ReadIOPS += rIOPS
			}
			if wIOPS, err := strconv.ParseFloat(parts[2], 64); err == nil {
				stats.WriteIOPS += wIOPS
			}

			// Parse KB/s (column 4) - on macOS this is total KB/s
			// We'll add it to WriteBW as backup operations are mostly writes
			if kb, err := strconv.ParseFloat(parts[3], 64); err == nil {
				stats.WriteBW += kb / 1024 // Convert KB/s to MB/s
			}
		}
	}

	return stats, nil
}

// DetectIOBandwidth is deprecated - no longer performs dd benchmark tests
// Returns a default safe limit to avoid impacting production systems
func DetectIOBandwidth() (int64, error) {
	i18n.Printf("[backup-helper] Auto rate limiting enabled (using default safe limit)\n")
	i18n.Printf("  Note: Real-time IO monitoring will be active during transfer\n")
	i18n.Printf("  Default limit: 300 MB/s\n")
	i18n.Printf("  Use --io-limit to manually set bandwidth limit if needed\n")

	// Return default safe limit - no destructive testing
	return 300 * 1024 * 1024, nil
}

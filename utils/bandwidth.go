package utils

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gioco-play/easy-i18n/i18n"
)

func parseDDOutput(output []byte) (int64, error) {
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	// Try parsing format with speed unit: "10485760 bytes (10 MB) copied, 0.5 s, 20.9 MB/s"
	// or "10485760 bytes (10 MB) copied, 0.00976458 s, 1.1 GB/s"
	for _, line := range lines {
		if strings.Contains(line, "copied,") {
			// Check for speed units: GB/s, MB/s, KB/s
			units := []string{"GB/s", "MB/s", "KB/s"}
			for _, unit := range units {
				if strings.Contains(line, unit) {
					i18n.Printf("    Found speed format line with %s: %s\n", unit, line)
					parts := strings.Fields(line)
					unitIndex := -1
					for i, part := range parts {
						if part == unit {
							unitIndex = i
							break
						}
					}
					if unitIndex > 0 {
						valueStr := parts[unitIndex-1]
						i18n.Printf("    Parsing %s value: %s\n", unit, valueStr)
						if value, err := strconv.ParseFloat(valueStr, 64); err == nil && value > 0 {
							var bandwidth int64
							switch unit {
							case "GB/s":
								bandwidth = int64(value * 1024 * 1024 * 1024)
							case "MB/s":
								bandwidth = int64(value * 1024 * 1024)
							case "KB/s":
								bandwidth = int64(value * 1024)
							}
							i18n.Printf("    Parsed bandwidth: %.2f %s = %d bytes/s\n", value, unit, bandwidth)
							return bandwidth, nil
						} else if err != nil {
							i18n.Printf("    Failed to parse %s value '%s': %v\n", unit, valueStr, err)
						}
					} else {
						i18n.Printf("    Could not find %s in line: %s\n", unit, line)
					}
				}
			}
		}
	}

	// Try parsing macOS format: "10485760 bytes transferred in 0.5 secs (20971520 bytes/sec)"
	for _, line := range lines {
		if strings.Contains(line, "bytes/sec)") {
			i18n.Printf("    Found macOS format line: %s\n", line)
			startIdx := strings.Index(line, "(")
			endIdx := strings.Index(line, "bytes/sec)")
			if startIdx > 0 && endIdx > startIdx {
				bytesStr := line[startIdx+1 : endIdx]
				bytesStr = strings.ReplaceAll(bytesStr, ",", "")
				bytesStr = strings.TrimSpace(bytesStr)
				i18n.Printf("    Parsing bytes/sec value: %s\n", bytesStr)
				if bytes, err := strconv.ParseInt(bytesStr, 10, 64); err == nil && bytes > 0 {
					i18n.Printf("    Parsed bandwidth: %d bytes/s\n", bytes)
					return bytes, nil
				} else if err != nil {
					i18n.Printf("    Failed to parse bytes/sec value '%s': %v\n", bytesStr, err)
				}
			} else {
				i18n.Printf("    Could not find bytes/sec pattern in line: %s\n", line)
			}
		}
	}

	// If we get here, parsing failed - log all lines for debugging
	i18n.Printf("    Failed to parse dd output. All output lines:\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			i18n.Printf("      [%d] %s\n", i+1, line)
		}
	}

	return 0, fmt.Errorf("failed to parse dd output: no recognized format found")
}

func runDDTest() (int64, error) {
	// Try Linux-style first (with direct I/O)
	ddCmd := exec.Command("dd", "if=/dev/zero", "of=/tmp/backup-helper-iobench.tmp", "bs=1M", "count=10", "oflag=direct", "conv=fdatasync")
	output, err := ddCmd.CombinedOutput()

	if err != nil {
		i18n.Printf("    Linux-style dd failed: %v\n", err)
		if len(output) > 0 {
			i18n.Printf("    Linux dd output: %s\n", string(output))
		}
		i18n.Printf("    Trying macOS-style dd...\n")
		// Try macOS-style (without special flags)
		ddCmd = exec.Command("dd", "if=/dev/zero", "of=/tmp/backup-helper-iobench.tmp", "bs=1M", "count=10")
		output, err = ddCmd.CombinedOutput()
		if err != nil {
			i18n.Printf("    macOS-style dd also failed: %v\n", err)
			if len(output) > 0 {
				i18n.Printf("    macOS dd output: %s\n", string(output))
			}
		}
	}

	// Clean up
	os.Remove("/tmp/backup-helper-iobench.tmp")

	if err != nil {
		return 0, fmt.Errorf("dd command failed: %v, output: %s", err, string(output))
	}

	// Log output for debugging
	if len(output) > 0 {
		outputLines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(outputLines) > 0 {
			lastLine := outputLines[len(outputLines)-1]
			i18n.Printf("    dd output (last line): %s\n", lastLine)
		}
	}

	return parseDDOutput(output)
}

func DetectIOBandwidth() (int64, error) {
	const numTests = 3
	var bandwidths []int64
	var sum int64

	i18n.Printf("[backup-helper] Detecting IO bandwidth...\n")

	for i := 0; i < numTests; i++ {
		if i > 0 {
			i18n.Printf("  Test %d/%d...\n", i+1, numTests)
		} else {
			i18n.Printf("  Test 1/%d...\n", numTests)
		}

		bandwidth, err := runDDTest()
		if err != nil {
			i18n.Printf("    Test failed: %v\n", err)
			continue
		}

		if bandwidth <= 0 {
			i18n.Printf("    Test returned invalid bandwidth: %d\n", bandwidth)
			continue
		}

		i18n.Printf("    Test succeeded: %s/s\n", formatBytes(bandwidth))
		bandwidths = append(bandwidths, bandwidth)
		sum += bandwidth
	}

	if len(bandwidths) == 0 {
		i18n.Printf("  Warning: All tests failed, using default 300 MB/s\n")
		i18n.Printf("Note: Use --io-limit to manually set bandwidth limit if needed\n")
		return 300 * 1024 * 1024, nil
	}

	averageBandwidth := sum / int64(len(bandwidths))

	i18n.Printf("  Tests: %d/%d successful\n", len(bandwidths), numTests)
	if len(bandwidths) > 1 {
		i18n.Printf("  Results: %s/s (average of %d tests)\n", formatBytes(averageBandwidth), len(bandwidths))
	} else {
		i18n.Printf("  Result: %s/s\n", formatBytes(averageBandwidth))
	}

	return averageBandwidth, nil
}

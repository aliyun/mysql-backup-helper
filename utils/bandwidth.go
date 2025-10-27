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

	for _, line := range lines {
		if strings.Contains(line, "copied,") && strings.Contains(line, "MB/s") {
			parts := strings.Fields(line)
			mbIndex := -1
			for i, part := range parts {
				if part == "MB/s" {
					mbIndex = i
					break
				}
			}
			if mbIndex > 0 {
				mbStr := parts[mbIndex-1]
				if mb, err := strconv.ParseFloat(mbStr, 64); err == nil && mb > 0 {
					bandwidth := int64(mb * 1024 * 1024)
					return bandwidth, nil
				}
			}
		}
	}

	for _, line := range lines {
		if strings.Contains(line, "bytes/sec)") {
			startIdx := strings.Index(line, "(")
			endIdx := strings.Index(line, "bytes/sec)")
			if startIdx > 0 && endIdx > startIdx {
				bytesStr := line[startIdx+1 : endIdx]
				bytesStr = strings.ReplaceAll(bytesStr, ",", "")
				bytesStr = strings.TrimSpace(bytesStr)
				if bytes, err := strconv.ParseInt(bytesStr, 10, 64); err == nil && bytes > 0 {
					return bytes, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("failed to parse dd output")
}

func runDDTest() (int64, error) {
	ddCmd := exec.Command("dd", "if=/dev/zero", "of=/tmp/backup-helper-iobench.tmp", "bs=1M", "count=10", "oflag=direct", "conv=fdatasync", "2>&1")
	output, err := ddCmd.CombinedOutput()

	if err != nil {
		ddCmd = exec.Command("dd", "if=/dev/zero", "of=/tmp/backup-helper-iobench.tmp", "bs=1M", "count=10", "2>&1")
		output, err = ddCmd.CombinedOutput()
	}

	os.Remove("/tmp/backup-helper-iobench.tmp")

	if err != nil {
		return 0, err
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
		}

		bandwidth, err := runDDTest()
		if err != nil {
			continue
		}

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

package utils

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ParseRateLimit parses a rate limit string with units (e.g., "100MB/s", "1GB/s", "500KB/s")
// Returns bytes per second
// Supported units: B/s, KB/s, MB/s, GB/s, TB/s (case insensitive)
func ParseRateLimit(rateStr string) (int64, error) {
	if rateStr == "" || rateStr == "0" {
		return 0, nil
	}

	rateStr = strings.TrimSpace(rateStr)
	rateStr = strings.ToUpper(rateStr)

	// Remove /s suffix if present
	rateStr = strings.TrimSuffix(rateStr, "/S")
	rateStr = strings.TrimSpace(rateStr)

	// Find where the number ends
	var numEnd int
	for i, r := range rateStr {
		if !unicode.IsDigit(r) && r != '.' {
			numEnd = i
			break
		}
		numEnd = i + 1
	}

	if numEnd == 0 {
		return 0, fmt.Errorf("invalid rate limit format: %s", rateStr)
	}

	// Parse number
	numStr := rateStr[:numEnd]
	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number in rate limit: %s", numStr)
	}

	if value <= 0 {
		return 0, nil
	}

	// Parse unit
	unitStr := strings.TrimSpace(rateStr[numEnd:])
	if unitStr == "" {
		// Default to bytes if no unit specified
		return int64(value), nil
	}

	// Map units to multipliers (base 1024)
	var multiplier float64
	switch unitStr {
	case "B", "BYTE", "BYTES":
		multiplier = 1
	case "KB", "K":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unsupported unit: %s (supported: B, KB, MB, GB, TB)", unitStr)
	}

	return int64(value * multiplier), nil
}

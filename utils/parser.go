package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseBandwidth parses bandwidth string with unit support
// Examples: "100MB/s", "1.5GB/s", "500KB/s", "1000000000" (bytes)
func ParseBandwidth(input string) (int64, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0, fmt.Errorf("empty bandwidth string")
	}

	// Remove /s suffix if present
	input = strings.TrimSuffix(strings.ToUpper(input), "/S")
	input = strings.TrimSpace(input)

	// Try to find unit suffix
	units := map[string]int64{
		"B":   1,
		"KB":  1024,
		"MB":  1024 * 1024,
		"GB":  1024 * 1024 * 1024,
		"TB":  1024 * 1024 * 1024 * 1024,
		"KIB": 1024,
		"MIB": 1024 * 1024,
		"GIB": 1024 * 1024 * 1024,
		"TIB": 1024 * 1024 * 1024 * 1024,
	}

	// Try to match unit
	for unit, multiplier := range units {
		if strings.HasSuffix(input, unit) {
			valueStr := strings.TrimSuffix(input, unit)
			value, err := strconv.ParseFloat(valueStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid number in bandwidth string '%s': %v", input, err)
			}
			return int64(value * float64(multiplier)), nil
		}
	}

	// No unit found, try parsing as pure number (bytes)
	value, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid bandwidth format '%s': expected format like '100MB/s' or bytes number", input)
	}
	return value, nil
}

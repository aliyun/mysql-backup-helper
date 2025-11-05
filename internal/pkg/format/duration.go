package format

import (
	"fmt"
	"time"
)

// Duration formats time.Duration to human-readable format
// Examples: "2h30m15s", "45m30s", "30s"
func Duration(d time.Duration) string {
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

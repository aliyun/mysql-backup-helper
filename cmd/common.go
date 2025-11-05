package cmd

import (
	"backup-helper/internal/pkg/format"
	"backup-helper/pkg/version"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gioco-play/easy-i18n/i18n"
)

// outputHeader displays the tool header
func outputHeader() {
	bar := strings.Repeat("#", 80)
	title := "MySQL Backup Helper"
	subtitle := "Powered by Alibaba Cloud Inc"
	ver := "v" + version.Get()
	timeStr := time.Now().Format("2006-01-02 15:04:05")

	i18n.Printf("%s\n", bar)
	// center display
	pad := (80 - len(title)) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Printf("%s%s\n", strings.Repeat(" ", pad), title)
	pad2 := (80 - len(subtitle)) / 2
	if pad2 < 0 {
		pad2 = 0
	}
	fmt.Printf("%s%s\n", strings.Repeat(" ", pad2), subtitle)
	fmt.Printf("%sVersion: %s    Time: %s\n", strings.Repeat(" ", 10), ver, timeStr)
	i18n.Printf("%s\n", bar)
}

// outputHeaderToStderr displays the tool header to stderr
func outputHeaderToStderr() {
	bar := strings.Repeat("#", 80)
	title := "MySQL Backup Helper"
	subtitle := "Powered by Alibaba Cloud Inc"
	ver := "v" + version.Get()
	timeStr := time.Now().Format("2006-01-02 15:04:05")

	fmt.Fprintf(os.Stderr, "%s\n", bar)
	pad := (80 - len(title)) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(os.Stderr, "%s%s\n", strings.Repeat(" ", pad), title)
	pad2 := (80 - len(subtitle)) / 2
	if pad2 < 0 {
		pad2 = 0
	}
	fmt.Fprintf(os.Stderr, "%s%s\n", strings.Repeat(" ", pad2), subtitle)
	fmt.Fprintf(os.Stderr, "%sVersion: %s    Time: %s\n", strings.Repeat(" ", 10), ver, timeStr)
	i18n.Fprintf(os.Stderr, "%s\n", bar)
}

// formatBytes formats bytes to human-readable format (delegate to format package)
func formatBytes(bytes int64) string {
	return format.Bytes(bytes)
}

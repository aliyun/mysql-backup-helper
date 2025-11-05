package cmd

import (
	"backup-helper/internal/config"
	"backup-helper/internal/pkg/format"
	"backup-helper/pkg/version"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gioco-play/easy-i18n/i18n"
	"github.com/spf13/cobra"
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

// parseTraffic parses traffic limit from flag or config
func parseTraffic(flagValue string, configValue int64) (int64, error) {
	if flagValue != "" {
		parsedLimit, err := format.ParseRateLimit(flagValue)
		if err != nil {
			return 0, fmt.Errorf("error parsing --traffic '%s': %v", flagValue, err)
		}
		return parsedLimit, nil
	}
	if configValue > 0 {
		return configValue, nil
	}
	return 0, nil
}

// applyTraffic updates config traffic based on traffic limit
func applyTraffic(cfg *config.Config, traffic int64) {
	if traffic == -1 {
		cfg.Traffic = 0 // 0 means unlimited
	} else if traffic > 0 {
		cfg.Traffic = traffic
	}
}

// parseAuthSettings parses authentication settings from flags or config
func parseAuthSettings(cmd *cobra.Command, enableFlagName string, enableFlag bool, keyFlag string, cfg *config.Config) (bool, string) {
	enableAuth := enableFlag
	if !cmd.Flags().Changed(enableFlagName) {
		enableAuth = cfg.EnableAuth
	}

	authKey := keyFlag
	if authKey == "" {
		authKey = cfg.AuthKey
	}

	return enableAuth, authKey
}

// determineMode determines operation mode based on flags
func determineMode(toOSS bool, toStream int) (mode string, streamPort int) {
	if toStream >= 0 {
		mode = "stream"
		streamPort = toStream
	} else if toOSS {
		mode = "oss"
		streamPort = 0
	} else {
		// Default to OSS if no destination specified
		mode = "oss"
		streamPort = 0
	}
	return mode, streamPort
}

// printTraffic prints traffic limit information
func printTraffic(traffic int64) {
	if traffic == 0 {
		i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
	} else {
		i18n.Printf("[backup-helper] Traffic limit set to: %s/s\n", format.Bytes(traffic))
	}
}

// logInfo prints info messages (suppressed in quiet mode)
func logInfo(format string, args ...interface{}) {
	if !IsQuiet() {
		i18n.Printf(format, args...)
	}
}

// logVerbose prints verbose messages (only in verbose mode)
func logVerbose(format string, args ...interface{}) {
	if IsVerbose() {
		i18n.Printf(format, args...)
	}
}

// logError prints error messages (always shown)
func logError(format string, args ...interface{}) {
	i18n.Printf(format, args...)
}

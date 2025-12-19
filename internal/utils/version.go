package utils

import (
	"os"
	"strings"
)

// variables injected at compile time
var (
	BuildVersion = "unknown"
	BuildTime    = "unknown"
	GitCommit    = "unknown"
)

// AppVersion struct stores application version information
type AppVersion struct {
	Version   string
	BuildTime string
	GitCommit string
}

// GetVersion gets version number, prefers compile-time injected version
func GetVersion() string {
	if BuildVersion != "unknown" {
		return BuildVersion
	}

	// fallback to reading from VERSION file
	data, err := os.ReadFile("VERSION")
	if err != nil {
		return "0.0.0"
	}
	return strings.TrimSpace(string(data))
}

// GetVersionInfo gets complete version information
func GetVersionInfo() AppVersion {
	return AppVersion{
		Version:   GetVersion(),
		BuildTime: BuildTime,
		GitCommit: GitCommit,
	}
}

// PrintVersion prints version information
func PrintVersion() {
	version := GetVersion()
	// Remove 'v' prefix if present to avoid double 'v' (e.g., vv1.0.0-beta)
	if strings.HasPrefix(version, "v") {
		version = version[1:]
	}
	println("MySQL Backup Helper v" + version)
}

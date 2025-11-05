package version

import (
	"os"
	"strings"
)

// Variables injected at compile time
var (
	BuildVersion = "unknown"
	BuildTime    = "unknown"
	GitCommit    = "unknown"
)

// Info struct stores application version information
type Info struct {
	Version   string
	BuildTime string
	GitCommit string
}

// Get gets version number, prefers compile-time injected version
func Get() string {
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

// GetInfo gets complete version information
func GetInfo() Info {
	return Info{
		Version:   Get(),
		BuildTime: BuildTime,
		GitCommit: GitCommit,
	}
}

// Print prints version information to stdout
func Print() {
	println("MySQL Backup Helper v" + Get())
}

package utils

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gioco-play/easy-i18n/i18n"
)

// Contains checks if a string contains a substring (case-insensitive)
func Contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// IsDirEmpty checks if a directory is empty
func IsDirEmpty(dir string) (bool, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil // Directory doesn't exist, consider it empty
		}
		return false, err
	}

	if !info.IsDir() {
		return false, fmt.Errorf("%s is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	return len(entries) == 0, nil
}

// PromptOverwrite asks user if they want to overwrite existing files in targetDir
// If autoYes is true, automatically returns true and shows a warning
func PromptOverwrite(targetDir string, autoYes bool) bool {
	i18n.Printf("Warning: Target directory '%s' already exists and is not empty.\n", targetDir)
	i18n.Printf("Extracting to this directory may overwrite existing files.\n")

	if autoYes {
		i18n.Printf("Auto-confirming overwrite (--yes/-y flag is set)...\n")
		return true
	}

	i18n.Printf("Do you want to continue? (y/n): ")

	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "y" || input == "yes"
}

// PromptAIDiagnosis asks user if they want to use AI diagnosis
// If autoYes is true, automatically returns true and shows a warning
func PromptAIDiagnosis(autoYes bool) bool {
	if autoYes {
		i18n.Printf("Auto-confirming AI diagnosis (--yes/-y flag is set)...\n")
		return true
	}

	var input string
	i18n.Printf("Would you like to use AI diagnosis? (y/n): ")
	fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// ClearDirectory removes all files and subdirectories in the given directory
func ClearDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		} else {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}

	return nil
}

// IsFlagPassed checks if command line parameter is set
func IsFlagPassed(name string) bool {
	found := false
	flag.CommandLine.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// FormatBytes formats bytes to human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// OutputHeader outputs the application header
func OutputHeader() {
	bar := strings.Repeat("#", 80)
	title := "MySQL Backup Helper"
	subtitle := "Powered by Alibaba Cloud Inc"
	versionStr := GetVersion()
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
	fmt.Printf("%sVersion: %s    Time: %s\n", strings.Repeat(" ", 10), versionStr, timeStr)
	i18n.Printf("%s\n", bar)
}

// OutputHeaderToStderr outputs the application header to stderr
func OutputHeaderToStderr() {
	bar := strings.Repeat("#", 80)
	title := "MySQL Backup Helper"
	subtitle := "Powered by Alibaba Cloud Inc"
	versionStr := GetVersion()
	timeStr := time.Now().Format("2006-01-02 15:04:05")

	fmt.Fprintf(os.Stderr, "%s\n", bar)
	// center display
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
	fmt.Fprintf(os.Stderr, "%sVersion: %s    Time: %s\n", strings.Repeat(" ", 10), versionStr, timeStr)
	i18n.Fprintf(os.Stderr, "%s\n", bar)
}

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ResolveXtrabackupPath resolves the paths to xtrabackup and xbstream binaries.
// Priority: cfg.XtrabackupPath (from flag or config) > XTRABACKUP_PATH env var > PATH lookup
// If a directory path is provided, xbstream will be searched in the same directory.
// Returns: (xtrabackupPath, xbstreamPath, error)
func ResolveXtrabackupPath(cfg *Config) (string, string, error) {
	var basePath string

	// Priority 1: Command-line flag or config file
	if cfg.XtrabackupPath != "" {
		basePath = cfg.XtrabackupPath
	} else {
		// Priority 2: Environment variable
		if envPath := os.Getenv("XTRABACKUP_PATH"); envPath != "" {
			basePath = envPath
		} else {
			// Priority 3: PATH lookup
			path, err := exec.LookPath("xtrabackup")
			if err != nil {
				return "", "", fmt.Errorf("xtrabackup not found in PATH. Please install Percona XtraBackup or specify path using --xtrabackup-path flag or XTRABACKUP_PATH environment variable")
			}
			basePath = path
		}
	}

	// Determine if basePath is a directory or file
	info, err := os.Stat(basePath)
	if err != nil {
		return "", "", fmt.Errorf("xtrabackup path not found: %s: %v", basePath, err)
	}

	var xtrabackupPath, xbstreamPath string
	var xbstreamDir string

	if info.IsDir() {
		// Directory: append /xtrabackup and /xbstream
		xtrabackupPath = filepath.Join(basePath, "xtrabackup")
		xbstreamPath = filepath.Join(basePath, "xbstream")
		xbstreamDir = basePath
	} else {
		// File: use as-is for xtrabackup, find xbstream in same directory
		xtrabackupPath = basePath
		xbstreamDir = filepath.Dir(basePath)
		xbstreamPath = filepath.Join(xbstreamDir, "xbstream")
	}

	// Validate xtrabackup path exists and is executable
	if _, err := os.Stat(xtrabackupPath); err != nil {
		return "", "", fmt.Errorf("xtrabackup binary not found at %s: %v", xtrabackupPath, err)
	}

	// Test if xtrabackup is executable
	cmd := exec.Command(xtrabackupPath, "--version")
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("xtrabackup at %s is not executable: %v", xtrabackupPath, err)
	}

	// Validate xbstream path exists and is executable
	if _, err := os.Stat(xbstreamPath); err != nil {
		return "", "", fmt.Errorf("xbstream binary not found at %s (expected in same directory as xtrabackup): %v", xbstreamPath, err)
	}

	// Test if xbstream is executable
	cmd = exec.Command(xbstreamPath, "--version")
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("xbstream at %s is not executable: %v", xbstreamPath, err)
	}

	return xtrabackupPath, xbstreamPath, nil
}

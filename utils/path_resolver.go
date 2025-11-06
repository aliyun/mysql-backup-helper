package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// checkBinaryExecutable checks if a binary file exists and is executable
// It uses multiple fallback methods for compatibility:
// 1. Check file exists and has execute permission
// 2. Try to run with --version (for tools that support it)
// 3. Try to run with -h or --help (for tools that support it)
// 4. Try to run without arguments (will show usage, exit code 1 is OK)
func checkBinaryExecutable(binaryPath string, binaryName string) error {
	// Check if file exists
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("%s binary not found at %s: %v", binaryName, binaryPath, err)
	}

	// Check if it's a regular file (not a directory)
	if info.IsDir() {
		return fmt.Errorf("%s path is a directory, not a file: %s", binaryName, binaryPath)
	}

	// Check execute permission
	mode := info.Mode()
	if mode.Perm()&0111 == 0 {
		return fmt.Errorf("%s at %s does not have execute permission", binaryName, binaryPath)
	}

	// Try multiple methods to verify it's actually executable
	// Method 1: Try --version (most common, but not all tools support it)
	cmd := exec.Command(binaryPath, "--version")
	if err := cmd.Run(); err == nil {
		return nil // Success
	}

	// Method 2: Try -h (help, very common)
	cmd = exec.Command(binaryPath, "-h")
	if err := cmd.Run(); err == nil {
		return nil // Success
	}

	// Method 3: Try --help
	cmd = exec.Command(binaryPath, "--help")
	if err := cmd.Run(); err == nil {
		return nil // Success
	}

	// Method 4: Try running without arguments
	// Most tools will show usage and exit with code 1, which is acceptable
	// We consider it executable if the process can start (even if it exits with error)
	cmd = exec.Command(binaryPath)
	err = cmd.Run()
	if err != nil {
		// Check if it's an exit error (exit code != 0)
		// This is OK - it means the binary ran but exited with an error
		// (e.g., missing arguments, which is expected)
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 or 2 usually means "usage" or "missing arguments"
			// This is acceptable - it proves the binary is executable
			if exitErr.ExitCode() == 1 || exitErr.ExitCode() == 2 {
				return nil // Success - binary is executable
			}
		}
		// Other errors (like "permission denied", "no such file") are real problems
		return fmt.Errorf("%s at %s appears to exist but cannot be executed: %v", binaryName, binaryPath, err)
	}

	// If we get here, the binary ran successfully (unlikely without args, but possible)
	return nil
}

// ResolveXtrabackupPath resolves the paths to xtrabackup and xbstream binaries.
// Priority: cfg.XtrabackupPath (from flag or config) > XTRABACKUP_PATH env var > PATH lookup
// If a directory path is provided, xbstream will be searched in the same directory.
// requireXbstream: if false, skip xbstream validation (e.g., for prepare mode)
// Returns: (xtrabackupPath, xbstreamPath, error)
func ResolveXtrabackupPath(cfg *Config, requireXbstream bool) (string, string, error) {
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
	if err := checkBinaryExecutable(xtrabackupPath, "xtrabackup"); err != nil {
		return "", "", err
	}

	// Validate xbstream only if required
	if requireXbstream {
		if err := checkBinaryExecutable(xbstreamPath, "xbstream"); err != nil {
			return "", "", fmt.Errorf("xbstream binary not found at %s (expected in same directory as xtrabackup): %v", xbstreamPath, err)
		}
	}

	return xtrabackupPath, xbstreamPath, nil
}

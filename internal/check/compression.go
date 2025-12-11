package check

import (
	"backup-helper/internal/config"
	"backup-helper/internal/utils"
	"fmt"
	"os/exec"

	"github.com/gioco-play/easy-i18n/i18n"
)

// checkToolExecutable checks if a tool is executable using multiple fallback methods
// This provides better compatibility with older versions that may not support --version
func checkToolExecutable(toolName string, toolPath string) error {
	// Try multiple methods to verify it's executable
	// Method 1: Try --version
	cmd := exec.Command(toolPath, "--version")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Method 2: Try -h
	cmd = exec.Command(toolPath, "-h")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Method 3: Try --help
	cmd = exec.Command(toolPath, "--help")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Method 4: Try without arguments (will show usage)
	cmd = exec.Command(toolPath)
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 or 2 usually means "usage" or "missing arguments"
			// This is acceptable - it proves the tool is executable
			if exitErr.ExitCode() == 1 || exitErr.ExitCode() == 2 {
				return nil // Success - tool is executable
			}
		}
		return fmt.Errorf("%s command found but not executable: %v", toolName, err)
	}

	return nil
}

// CheckCompressionDependencies checks if required tools are available for the specified compression type
// For backup mode: zstd needs zstd tool, qp needs qpress tool (xtrabackup --compress uses qpress internally)
// For download mode: both zstd and qp need external tools for decompression
// isBackupMode: true for backup mode, false for download mode
// cfg: configuration for resolving xtrabackup/xbstream paths
// Returns error if tool is not found or not executable
func CheckCompressionDependencies(compressType string, isBackupMode bool, cfg *config.Config) error {
	switch compressType {
	case "zstd":
		zstdPath, err := exec.LookPath("zstd")
		if err != nil {
			return fmt.Errorf("%s", i18n.Sprintf("zstd command not found. Please install zstd: https://github.com/facebook/zstd"))
		}
		// Test if zstd is executable using multiple fallback methods
		if err := checkToolExecutable("zstd", zstdPath); err != nil {
			return fmt.Errorf("%s", i18n.Sprintf("zstd command found but not executable. Please check installation"))
		}
		return nil
	case "qp":
		// For qpress compression, we need qpress tool
		// Even though xtrabackup --compress uses qpress internally, we still need qpress tool for backup mode
		qpressPath, err := exec.LookPath("qpress")
		if err != nil {
			return fmt.Errorf("%s", i18n.Sprintf("qpress command not found. Please install qpress: https://github.com/mariadb-corporation/qpress"))
		}
		// Test if qpress is executable using multiple fallback methods
		if err := checkToolExecutable("qpress", qpressPath); err != nil {
			return fmt.Errorf("%s", i18n.Sprintf("qpress command found but not executable. Please check installation"))
		}
		if !isBackupMode {
			// For download mode, also check xbstream and xtrabackup
			_, _, err := utils.ResolveXtrabackupPath(cfg.XtrabackupPath, true)
			if err != nil {
				return err
			}
		}
		return nil
	case "":
		// No compression, no dependencies needed
		return nil
	default:
		return fmt.Errorf("unknown compression type: %s", compressType)
	}
}

// CheckExtractionDependencies checks if required tools are available for extraction
// This is used in download mode when --target-dir is specified
func CheckExtractionDependencies(compressType string, cfg *config.Config) error {
	switch compressType {
	case "zstd":
		// Need zstd for decompression and xbstream for extraction
		if err := CheckCompressionDependencies("zstd", false, cfg); err != nil {
			return err
		}
		_, _, err := utils.ResolveXtrabackupPath(cfg.XtrabackupPath, true)
		if err != nil {
			return err
		}
		return nil
	case "qp":
		// Need qpress, xbstream, and xtrabackup
		return CheckCompressionDependencies("qp", false, cfg)
	case "":
		// No compression, only need xbstream
		_, _, err := utils.ResolveXtrabackupPath(cfg.XtrabackupPath, true)
		if err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unknown compression type: %s", compressType)
	}
}

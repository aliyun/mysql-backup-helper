package utils

import (
	"fmt"
	"os/exec"

	"github.com/gioco-play/easy-i18n/i18n"
)

// CheckCompressionDependencies checks if required tools are available for the specified compression type
// For backup mode: zstd needs zstd tool, qp needs qpress tool (xtrabackup --compress uses qpress internally)
// For download mode: both zstd and qp need external tools for decompression
// isBackupMode: true for backup mode, false for download mode
// Returns error if tool is not found or not executable
func CheckCompressionDependencies(compressType string, isBackupMode bool) error {
	switch compressType {
	case "zstd":
		if _, err := exec.LookPath("zstd"); err != nil {
			return fmt.Errorf("%s", i18n.Sprintf("zstd command not found. Please install zstd: https://github.com/facebook/zstd"))
		}
		// Test if zstd is executable
		cmd := exec.Command("zstd", "--version")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s", i18n.Sprintf("zstd command found but not executable. Please check installation"))
		}
		return nil
	case "qp":
		// For qpress compression, we need qpress tool
		// Even though xtrabackup --compress uses qpress internally, we still need qpress tool for backup mode
		if _, err := exec.LookPath("qpress"); err != nil {
			return fmt.Errorf("%s", i18n.Sprintf("qpress command not found. Please install qpress: https://github.com/mariadb-corporation/qpress"))
		}
		// Test if qpress is executable
		cmd := exec.Command("qpress", "-h")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s", i18n.Sprintf("qpress command found but not executable. Please check installation"))
		}
		if !isBackupMode {
			// For download mode, also check xbstream and xtrabackup
			if _, err := exec.LookPath("xbstream"); err != nil {
				return fmt.Errorf("%s", i18n.Sprintf("xbstream command not found. Please install Percona XtraBackup"))
			}
			if _, err := exec.LookPath("xtrabackup"); err != nil {
				return fmt.Errorf("%s", i18n.Sprintf("xtrabackup command not found. Please install Percona XtraBackup"))
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
func CheckExtractionDependencies(compressType string) error {
	switch compressType {
	case "zstd":
		// Need zstd for decompression and xbstream for extraction
		if err := CheckCompressionDependencies("zstd", false); err != nil {
			return err
		}
		if _, err := exec.LookPath("xbstream"); err != nil {
			return fmt.Errorf("%s", i18n.Sprintf("xbstream command not found. Please install Percona XtraBackup"))
		}
		return nil
	case "qp":
		// Need qpress, xbstream, and xtrabackup
		return CheckCompressionDependencies("qp", false)
	case "":
		// No compression, only need xbstream
		if _, err := exec.LookPath("xbstream"); err != nil {
			return fmt.Errorf("%s", i18n.Sprintf("xbstream command not found. Please install Percona XtraBackup"))
		}
		return nil
	default:
		return fmt.Errorf("unknown compression type: %s", compressType)
	}
}

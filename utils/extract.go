package utils

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gioco-play/easy-i18n/i18n"
)

// ExtractXbstream extracts xbstream backup data to target directory
// It reads from reader and pipes to xbstream -x -C targetDir
func ExtractXbstream(reader io.Reader, targetDir string) error {
	// Check if xbstream is available
	if _, err := exec.LookPath("xbstream"); err != nil {
		return fmt.Errorf("%s", i18n.Sprintf("xbstream command not found. Please install Percona XtraBackup: https://www.percona.com/downloads/Percona-XtraBackup-LATEST/"))
	}

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %v", targetDir, err)
	}

	// Create xbstream command
	cmd := exec.Command("xbstream", "-x", "-C", targetDir)
	cmd.Stdin = reader
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run xbstream
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xbstream extraction failed: %v", err)
	}

	return nil
}

// ExtractXbstreamWithDecompress extracts xbstream backup data and decompresses if needed
// It handles both compressed and uncompressed backups
func ExtractXbstreamWithDecompress(reader io.Reader, targetDir string, isCompressed bool, compressType string) error {
	// Check if xbstream is available
	if _, err := exec.LookPath("xbstream"); err != nil {
		return fmt.Errorf("%s", i18n.Sprintf("xbstream command not found. Please install Percona XtraBackup: https://www.percona.com/downloads/Percona-XtraBackup-LATEST/"))
	}

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %v", targetDir, err)
	}

	if !isCompressed || compressType == "" {
		// No compression, directly extract
		return ExtractXbstream(reader, targetDir)
	}

	// Handle compression
	switch compressType {
	case "zstd":
		// Check if zstd is available
		if _, err := exec.LookPath("zstd"); err != nil {
			return fmt.Errorf("%s", i18n.Sprintf("zstd command not found. Please install zstd: https://github.com/facebook/zstd"))
		}
		// First decompress, then extract: reader -> zstd -d -> xbstream -x
		decompressCmd := exec.Command("zstd", "-d")
		decompressCmd.Stdin = reader
		decompressCmd.Stderr = os.Stderr

		// Pipe decompressed data to xbstream
		decompressedPipe, err := decompressCmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create decompress pipe: %v", err)
		}

		// Create xbstream command
		xbstreamCmd := exec.Command("xbstream", "-x", "-C", targetDir)
		xbstreamCmd.Stdin = decompressedPipe
		xbstreamCmd.Stdout = os.Stdout
		xbstreamCmd.Stderr = os.Stderr

		// Start decompress command
		if err := decompressCmd.Start(); err != nil {
			return fmt.Errorf("failed to start decompress command: %v", err)
		}

		// Start xbstream command
		if err := xbstreamCmd.Start(); err != nil {
			decompressCmd.Process.Kill()
			return fmt.Errorf("failed to start xbstream command: %v", err)
		}

		// Wait for both commands
		decompressErr := decompressCmd.Wait()
		xbstreamErr := xbstreamCmd.Wait()

		if decompressErr != nil {
			return fmt.Errorf("decompress failed: %v", decompressErr)
		}
		if xbstreamErr != nil {
			return fmt.Errorf("xbstream extraction failed: %v", xbstreamErr)
		}

		return nil
	case "qp", "qpress":
		// qpress compression is handled by xtrabackup --decompress after extraction
		// First extract to temp location or directly, then decompress
		if err := ExtractXbstream(reader, targetDir); err != nil {
			return err
		}

		// Check if xtrabackup is available for decompression
		if _, err := exec.LookPath("xtrabackup"); err != nil {
			return fmt.Errorf("%s", i18n.Sprintf("xtrabackup command not found. Please install Percona XtraBackup: https://www.percona.com/downloads/Percona-XtraBackup-LATEST/"))
		}

		// Run xtrabackup --decompress
		decompressCmd := exec.Command("xtrabackup", "--decompress", "--target-dir", targetDir)
		decompressCmd.Stdout = os.Stdout
		decompressCmd.Stderr = os.Stderr

		if err := decompressCmd.Run(); err != nil {
			return fmt.Errorf("xtrabackup decompress failed: %v", err)
		}

		return nil
	default:
		// Unknown compression type, try direct extraction
		i18n.Printf("[backup-helper] Warning: Unknown compression type '%s', attempting direct extraction\n", compressType)
		return ExtractXbstream(reader, targetDir)
	}
}

// GetExtractTargetDir validates and returns absolute path for extraction target directory
func GetExtractTargetDir(targetDir string) (string, error) {
	if targetDir == "" {
		return "", fmt.Errorf("target directory cannot be empty")
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %s: %v", targetDir, err)
	}

	return absPath, nil
}


package utils

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/gioco-play/easy-i18n/i18n"
)

// ExtractBackupStream handles decompression and extraction of backup stream
// compressType: "zstd", "qp", or "" (no compression)
// extractDir: directory to extract files, if empty, just save compressed/uncompressed file
// Returns error if extraction fails
func ExtractBackupStream(reader io.Reader, compressType string, extractDir string, outputPath string) error {
	if extractDir == "" {
		// No extraction requested, just save the stream
		if compressType == "zstd" {
			// For zstd, we need to decompress first
			return saveZstdDecompressed(reader, outputPath)
		}
		// For qpress or no compression, save as-is
		file, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		defer file.Close()
		_, err = io.Copy(file, reader)
		return err
	}

	// Extraction requested
	if compressType == "zstd" {
		return extractZstdStream(reader, extractDir)
	} else if compressType == "qp" {
		// qpress compression requires saving to file first, then using xtrabackup --decompress
		// This is because xbstream doesn't support --decompress in stream mode for MySQL 5.7
		return extractQpressStream(reader, extractDir, outputPath)
	} else {
		// No compression, just extract with xbstream
		return extractXbstream(reader, extractDir)
	}
}

// saveZstdDecompressed saves zstd-compressed stream after decompression
func saveZstdDecompressed(reader io.Reader, outputPath string) error {
	// Check zstd dependency
	if _, err := exec.LookPath("zstd"); err != nil {
		return fmt.Errorf("%s", i18n.Sprintf("zstd command not found. Please install zstd: https://github.com/facebook/zstd"))
	}

	zstdCmd := exec.Command("zstd", "-d", "-o", outputPath)
	zstdCmd.Stdin = reader
	zstdCmd.Stderr = os.Stderr
	zstdCmd.Stdout = os.Stderr

	return zstdCmd.Run()
}

// extractZstdStream decompresses zstd stream and extracts with xbstream
func extractZstdStream(reader io.Reader, extractDir string) error {
	// Check dependencies
	if _, err := exec.LookPath("zstd"); err != nil {
		return fmt.Errorf("%s", i18n.Sprintf("zstd command not found. Please install zstd: https://github.com/facebook/zstd"))
	}
	if _, err := exec.LookPath("xbstream"); err != nil {
		return fmt.Errorf("%s", i18n.Sprintf("xbstream command not found. Please install Percona XtraBackup"))
	}

	// Create extraction directory
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("failed to create extraction directory: %v", err)
	}

	// Pipe: reader -> zstd -d -> xbstream -x
	zstdCmd := exec.Command("zstd", "-d", "-")
	zstdCmd.Stdin = reader
	zstdCmd.Stderr = os.Stderr

	xbstreamCmd := exec.Command("xbstream", "-x", "-C", extractDir)
	xbstreamCmd.Stdin, _ = zstdCmd.StdoutPipe()
	xbstreamCmd.Stderr = os.Stderr
	xbstreamCmd.Stdout = os.Stderr

	if err := zstdCmd.Start(); err != nil {
		return fmt.Errorf("failed to start zstd decompression: %v", err)
	}

	if err := xbstreamCmd.Start(); err != nil {
		zstdCmd.Process.Kill()
		return fmt.Errorf("failed to start xbstream extraction: %v", err)
	}

	// Wait for both processes
	zstdErr := zstdCmd.Wait()
	xbstreamErr := xbstreamCmd.Wait()

	if zstdErr != nil {
		return fmt.Errorf("zstd decompression failed: %v", zstdErr)
	}
	if xbstreamErr != nil {
		return fmt.Errorf("xbstream extraction failed: %v", xbstreamErr)
	}

	return nil
}

// extractQpressStream handles qpress-compressed backup stream
// Note: xbstream doesn't support --decompress in stream mode for MySQL 5.7
// So we need to save to file first, then extract and decompress
func extractQpressStream(reader io.Reader, extractDir string, outputPath string) error {
	// Check dependencies
	if _, err := exec.LookPath("xbstream"); err != nil {
		return fmt.Errorf("%s", i18n.Sprintf("xbstream command not found. Please install Percona XtraBackup"))
	}
	if _, err := exec.LookPath("xtrabackup"); err != nil {
		return fmt.Errorf("%s", i18n.Sprintf("xtrabackup command not found. Please install Percona XtraBackup"))
	}

	// Step 1: Save compressed stream to file
	if outputPath == "" {
		outputPath = "backup_temp.xb"
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}

	_, err = io.Copy(file, reader)
	file.Close()
	if err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("failed to save compressed stream: %v", err)
	}

	// Step 2: Extract with xbstream
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("failed to create extraction directory: %v", err)
	}

	extractFile, err := os.Open(outputPath)
	if err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("failed to open compressed file: %v", err)
	}
	defer extractFile.Close()

	xbstreamCmd := exec.Command("xbstream", "-x", "-C", extractDir)
	xbstreamCmd.Stdin = extractFile
	xbstreamCmd.Stderr = os.Stderr
	xbstreamCmd.Stdout = os.Stderr

	if err := xbstreamCmd.Run(); err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("xbstream extraction failed: %v", err)
	}

	// Step 3: Decompress extracted files using xtrabackup --decompress
	xtrabackupCmd := exec.Command("xtrabackup", "--decompress", "--target-dir", extractDir)
	xtrabackupCmd.Stderr = os.Stderr
	xtrabackupCmd.Stdout = os.Stderr

	if err := xtrabackupCmd.Run(); err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("xtrabackup decompression failed: %v", err)
	}

	// Clean up temporary file
	os.Remove(outputPath)

	return nil
}

// extractXbstream extracts uncompressed xbstream backup
func extractXbstream(reader io.Reader, extractDir string) error {
	// Check dependency
	if _, err := exec.LookPath("xbstream"); err != nil {
		return fmt.Errorf("%s", i18n.Sprintf("xbstream command not found. Please install Percona XtraBackup"))
	}

	// Create extraction directory
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("failed to create extraction directory: %v", err)
	}

	// Extract with xbstream
	xbstreamCmd := exec.Command("xbstream", "-x", "-C", extractDir)
	xbstreamCmd.Stdin = reader
	xbstreamCmd.Stderr = os.Stderr
	xbstreamCmd.Stdout = os.Stderr

	return xbstreamCmd.Run()
}

// ExtractBackupStreamToStdout handles decompression only (for piping to xbstream)
// Returns reader that can be piped to xbstream
func ExtractBackupStreamToStdout(reader io.Reader, compressType string) (io.Reader, *exec.Cmd, error) {
	if compressType == "zstd" {
		// Check zstd dependency
		if _, err := exec.LookPath("zstd"); err != nil {
			return nil, nil, fmt.Errorf("%s", i18n.Sprintf("zstd command not found. Please install zstd: https://github.com/facebook/zstd"))
		}

		// Decompress with zstd
		zstdCmd := exec.Command("zstd", "-d", "-")
		zstdCmd.Stdin = reader
		zstdCmd.Stderr = os.Stderr

		stdout, err := zstdCmd.StdoutPipe()
		if err != nil {
			return nil, nil, err
		}

		if err := zstdCmd.Start(); err != nil {
			return nil, nil, fmt.Errorf("failed to start zstd decompression: %v", err)
		}

		return stdout, zstdCmd, nil
	}

	// No compression or qpress - return as-is
	// Note: qpress cannot be stream-decompressed, so user needs to save file first
	return reader, nil, nil
}


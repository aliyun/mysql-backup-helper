package utils

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"github.com/gioco-play/easy-i18n/i18n"
)

// ExtractBackupStream handles decompression and extraction of backup stream
// compressType: "zstd", "qp", or "" (no compression)
// targetDir: directory to extract files, if empty, just save compressed/uncompressed file
// parallel: number of parallel threads (default: 4)
// cfg: configuration for resolving xtrabackup/xbstream paths
// logCtx: log context for writing logs
// Returns error if extraction fails
func ExtractBackupStream(reader io.Reader, compressType string, targetDir string, outputPath string, parallel int, cfg *Config, logCtx *LogContext) error {
	if parallel == 0 {
		parallel = 4
	}

	if targetDir == "" {
		// No extraction requested, just save the stream
		if compressType == "zstd" {
			// For zstd, we need to decompress first
			return saveZstdDecompressed(reader, outputPath, parallel, logCtx)
		}
		// For qpress or no compression, save as-is
		if logCtx != nil {
			logCtx.WriteLog("EXTRACT", "Saving stream to %s", outputPath)
		}
		file, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		defer file.Close()
		_, err = io.Copy(file, reader)
		if err != nil {
			if logCtx != nil {
				logCtx.WriteLog("EXTRACT", "Failed to save stream: %v", err)
				// Check if it's a connection error
				errStr := err.Error()
				if strings.Contains(strings.ToLower(errStr), "eof") || strings.Contains(strings.ToLower(errStr), "broken pipe") || strings.Contains(strings.ToLower(errStr), "connection") {
					logCtx.WriteLog("TCP", "Connection interrupted while saving stream: %v", err)
				}
			}
			return err
		}
		return nil
	}

	// Extraction requested
	if compressType == "zstd" {
		return extractZstdStream(reader, targetDir, parallel, cfg, logCtx)
	} else if compressType == "qp" {
		// qpress compression requires saving to file first, then using xtrabackup --decompress
		// This is because xbstream doesn't support --decompress in stream mode for MySQL 5.7
		return extractQpressStream(reader, targetDir, outputPath, parallel, cfg, logCtx)
	} else {
		// No compression, just extract with xbstream
		return extractXbstream(reader, targetDir, parallel, cfg, logCtx)
	}
}

// saveZstdDecompressed saves zstd-compressed stream after decompression
func saveZstdDecompressed(reader io.Reader, outputPath string, parallel int, logCtx *LogContext) error {
	// Check zstd dependency
	if _, err := exec.LookPath("zstd"); err != nil {
		return fmt.Errorf("%s", i18n.Sprintf("zstd command not found. Please install zstd: https://github.com/facebook/zstd"))
	}

	if parallel == 0 {
		parallel = 4
	}

	if logCtx != nil {
		logCtx.WriteLog("DECOMPRESS", "Decompressing zstd stream to %s", outputPath)
	}

	zstdCmd := exec.Command("zstd", "-d", fmt.Sprintf("-T%d", parallel), "-o", outputPath)
	zstdCmd.Stdin = reader
	if logCtx != nil {
		zstdCmd.Stderr = logCtx.GetFile()
		zstdCmd.Stdout = logCtx.GetFile()
	} else {
		zstdCmd.Stderr = os.Stderr
		zstdCmd.Stdout = os.Stderr
	}

	err := zstdCmd.Run()
	if err != nil && logCtx != nil {
		logCtx.WriteLog("DECOMPRESS", "zstd decompression failed: %v", err)
	} else if logCtx != nil {
		logCtx.WriteLog("DECOMPRESS", "zstd decompression completed successfully")
	}
	return err
}

// extractZstdStream decompresses zstd stream and extracts with xbstream
func extractZstdStream(reader io.Reader, targetDir string, parallel int, cfg *Config, logCtx *LogContext) error {
	// Check dependencies
	if _, err := exec.LookPath("zstd"); err != nil {
		return fmt.Errorf("%s", i18n.Sprintf("zstd command not found. Please install zstd: https://github.com/facebook/zstd"))
	}

	// Resolve xbstream path
	_, xbstreamPath, err := ResolveXtrabackupPath(cfg, true)
	if err != nil {
		return err
	}

	if parallel == 0 {
		parallel = 4
	}

	// Create extraction directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create extraction directory: %v", err)
	}

	if logCtx != nil {
		logCtx.WriteLog("DECOMPRESS", "Decompressing zstd stream")
		logCtx.WriteLog("XBSTREAM", "Extracting to directory: %s", targetDir)
	}

	// Pipe: reader -> zstd -d -T<parallel> -> xbstream -x --parallel=<parallel>
	zstdCmd := exec.Command("zstd", "-d", fmt.Sprintf("-T%d", parallel), "-")
	zstdCmd.Stdin = reader
	if logCtx != nil {
		zstdCmd.Stderr = logCtx.GetFile()
	} else {
		zstdCmd.Stderr = os.Stderr
	}

	xbstreamCmd := exec.Command(xbstreamPath, "-x", fmt.Sprintf("--parallel=%d", parallel), "-C", targetDir)
	xbstreamCmd.Stdin, _ = zstdCmd.StdoutPipe()
	if logCtx != nil {
		xbstreamCmd.Stderr = logCtx.GetFile()
		xbstreamCmd.Stdout = logCtx.GetFile()
	} else {
		xbstreamCmd.Stderr = os.Stderr
		xbstreamCmd.Stdout = os.Stderr
	}

	if err := zstdCmd.Start(); err != nil {
		if logCtx != nil {
			logCtx.WriteLog("DECOMPRESS", "Failed to start zstd: %v", err)
		}
		return fmt.Errorf("failed to start zstd decompression: %v", err)
	}

	if err := xbstreamCmd.Start(); err != nil {
		zstdCmd.Process.Kill()
		if logCtx != nil {
			logCtx.WriteLog("XBSTREAM", "Failed to start xbstream: %v", err)
		}
		return fmt.Errorf("failed to start xbstream extraction: %v", err)
	}

	// Wait for both processes
	zstdErr := zstdCmd.Wait()
	xbstreamErr := xbstreamCmd.Wait()

	// Check if zstd failed due to connection error
	if zstdErr != nil {
		if logCtx != nil {
			logCtx.WriteLog("DECOMPRESS", "zstd decompression failed: %v", zstdErr)
		}
		// Check if it's a connection error (broken pipe or EOF unexpectedly)
		errStr := zstdErr.Error()
		if strings.Contains(strings.ToLower(errStr), "broken pipe") || strings.Contains(strings.ToLower(errStr), "eof") || strings.Contains(strings.ToLower(errStr), "connection") {
			errMsg := fmt.Sprintf("zstd decompression interrupted: connection closed unexpectedly: %v", zstdErr)
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Connection interrupted during decompression: %s", errMsg)
			}
			return fmt.Errorf("%s", errMsg)
		}
		// Check for signal-based termination (Unix-like systems)
		if runtime.GOOS != "windows" {
			if exitError, ok := zstdErr.(*exec.ExitError); ok {
				if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
					if status.Signaled() && (status.Signal() == syscall.SIGPIPE || status.Signal() == syscall.SIGTERM) {
						errMsg := fmt.Sprintf("zstd decompression interrupted: connection closed unexpectedly (signal: %s)", status.Signal())
						if logCtx != nil {
							logCtx.WriteLog("TCP", "Connection interrupted during decompression: %s", errMsg)
						}
						return fmt.Errorf("%s", errMsg)
					}
				}
			}
		}
		return fmt.Errorf("zstd decompression failed: %v", zstdErr)
	}

	// Check if xbstream failed due to connection error
	if xbstreamErr != nil {
		if logCtx != nil {
			logCtx.WriteLog("XBSTREAM", "xbstream extraction failed: %v", xbstreamErr)
		}
		// Check if it's a connection error (broken pipe or EOF unexpectedly)
		errStr := xbstreamErr.Error()
		if strings.Contains(strings.ToLower(errStr), "broken pipe") || strings.Contains(strings.ToLower(errStr), "eof") || strings.Contains(strings.ToLower(errStr), "connection") {
			errMsg := fmt.Sprintf("xbstream extraction interrupted: connection closed unexpectedly: %v", xbstreamErr)
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Connection interrupted during extraction: %s", errMsg)
			}
			return fmt.Errorf("%s", errMsg)
		}
		// Check for signal-based termination (Unix-like systems)
		if runtime.GOOS != "windows" {
			if exitError, ok := xbstreamErr.(*exec.ExitError); ok {
				if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
					if status.Signaled() && (status.Signal() == syscall.SIGPIPE || status.Signal() == syscall.SIGTERM) {
						errMsg := fmt.Sprintf("xbstream extraction interrupted: connection closed unexpectedly (signal: %s)", status.Signal())
						if logCtx != nil {
							logCtx.WriteLog("TCP", "Connection interrupted during extraction: %s", errMsg)
						}
						return fmt.Errorf("%s", errMsg)
					}
				}
			}
		}
		return fmt.Errorf("xbstream extraction failed: %v", xbstreamErr)
	}

	if logCtx != nil {
		logCtx.WriteLog("DECOMPRESS", "zstd decompression completed successfully")
		logCtx.WriteLog("XBSTREAM", "xbstream extraction completed successfully")
	}
	return nil
}

// extractQpressStream handles qpress-compressed backup stream
// Note: xbstream doesn't support --decompress in stream mode for MySQL 5.7
// So we need to save to file first, then extract and decompress
func extractQpressStream(reader io.Reader, targetDir string, outputPath string, parallel int, cfg *Config, logCtx *LogContext) error {
	// Resolve xtrabackup and xbstream paths
	xtrabackupPath, xbstreamPath, err := ResolveXtrabackupPath(cfg, true)
	if err != nil {
		return err
	}

	if parallel == 0 {
		parallel = 4
	}

	if logCtx != nil {
		logCtx.WriteLog("EXTRACT", "Extracting qpress-compressed backup")
		logCtx.WriteLog("EXTRACT", "Target directory: %s", targetDir)
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
		if logCtx != nil {
			logCtx.WriteLog("EXTRACT", "Failed to save compressed stream: %v", err)
			// Check if it's a connection error
			errStr := err.Error()
			if strings.Contains(strings.ToLower(errStr), "eof") || strings.Contains(strings.ToLower(errStr), "broken pipe") || strings.Contains(strings.ToLower(errStr), "connection") {
				logCtx.WriteLog("TCP", "Connection interrupted while saving compressed stream: %v", err)
			}
		}
		return fmt.Errorf("failed to save compressed stream: %v", err)
	}

	if logCtx != nil {
		logCtx.WriteLog("EXTRACT", "Saved compressed stream to temporary file: %s", outputPath)
	}

	// Step 2: Extract with xbstream
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("failed to create extraction directory: %v", err)
	}

	extractFile, err := os.Open(outputPath)
	if err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("failed to open compressed file: %v", err)
	}
	defer extractFile.Close()

	if logCtx != nil {
		logCtx.WriteLog("XBSTREAM", "Extracting with xbstream")
	}

	xbstreamCmd := exec.Command(xbstreamPath, "-x", fmt.Sprintf("--parallel=%d", parallel), "-C", targetDir)
	xbstreamCmd.Stdin = extractFile
	if logCtx != nil {
		xbstreamCmd.Stderr = logCtx.GetFile()
		xbstreamCmd.Stdout = logCtx.GetFile()
	} else {
		xbstreamCmd.Stderr = os.Stderr
		xbstreamCmd.Stdout = os.Stderr
	}

	if err := xbstreamCmd.Run(); err != nil {
		os.Remove(outputPath)
		if logCtx != nil {
			logCtx.WriteLog("XBSTREAM", "xbstream extraction failed: %v", err)
		}
		return fmt.Errorf("xbstream extraction failed: %v", err)
	}

	if logCtx != nil {
		logCtx.WriteLog("XBSTREAM", "xbstream extraction completed successfully")
		logCtx.WriteLog("DECOMPRESS", "Decompressing with xtrabackup --decompress")
	}

	// Step 3: Decompress extracted files using xtrabackup --decompress
	xtrabackupCmd := exec.Command(xtrabackupPath, "--decompress", fmt.Sprintf("--parallel=%d", parallel), "--target-dir", targetDir)
	if logCtx != nil {
		xtrabackupCmd.Stderr = logCtx.GetFile()
		xtrabackupCmd.Stdout = logCtx.GetFile()
	} else {
		xtrabackupCmd.Stderr = os.Stderr
		xtrabackupCmd.Stdout = os.Stderr
	}

	if err := xtrabackupCmd.Run(); err != nil {
		os.Remove(outputPath)
		if logCtx != nil {
			logCtx.WriteLog("DECOMPRESS", "xtrabackup decompression failed: %v", err)
		}
		return fmt.Errorf("xtrabackup decompression failed: %v", err)
	}

	// Clean up temporary file
	os.Remove(outputPath)

	if logCtx != nil {
		logCtx.WriteLog("DECOMPRESS", "xtrabackup decompression completed successfully")
		logCtx.WriteLog("EXTRACT", "Extraction completed successfully")
	}
	return nil
}

// extractXbstream extracts uncompressed xbstream backup
func extractXbstream(reader io.Reader, targetDir string, parallel int, cfg *Config, logCtx *LogContext) error {
	// Resolve xbstream path
	_, xbstreamPath, err := ResolveXtrabackupPath(cfg, true)
	if err != nil {
		return err
	}

	if parallel == 0 {
		parallel = 4
	}

	// Create extraction directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create extraction directory: %v", err)
	}

	if logCtx != nil {
		logCtx.WriteLog("XBSTREAM", "Extracting xbstream backup to directory: %s", targetDir)
	}

	// Extract with xbstream
	xbstreamCmd := exec.Command(xbstreamPath, "-x", fmt.Sprintf("--parallel=%d", parallel), "-C", targetDir)
	xbstreamCmd.Stdin = reader
	if logCtx != nil {
		xbstreamCmd.Stderr = logCtx.GetFile()
		xbstreamCmd.Stdout = logCtx.GetFile()
	} else {
		xbstreamCmd.Stderr = os.Stderr
		xbstreamCmd.Stdout = os.Stderr
	}

	err = xbstreamCmd.Run()
	if err != nil {
		if logCtx != nil {
			logCtx.WriteLog("XBSTREAM", "xbstream extraction failed: %v", err)
		}
		// Check if it's a connection error (broken pipe or EOF unexpectedly)
		errStr := err.Error()
		if strings.Contains(strings.ToLower(errStr), "broken pipe") || strings.Contains(strings.ToLower(errStr), "eof") || strings.Contains(strings.ToLower(errStr), "connection") {
			errMsg := fmt.Sprintf("xbstream extraction interrupted: connection closed unexpectedly: %v", err)
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Connection interrupted during extraction: %s", errMsg)
			}
			return fmt.Errorf("%s", errMsg)
		}
		// Check for signal-based termination (Unix-like systems)
		if runtime.GOOS != "windows" {
			if exitError, ok := err.(*exec.ExitError); ok {
				if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
					if status.Signaled() && (status.Signal() == syscall.SIGPIPE || status.Signal() == syscall.SIGTERM) {
						errMsg := fmt.Sprintf("xbstream extraction interrupted: connection closed unexpectedly (signal: %s)", status.Signal())
						if logCtx != nil {
							logCtx.WriteLog("TCP", "Connection interrupted during extraction: %s", errMsg)
						}
						return fmt.Errorf("%s", errMsg)
					}
				}
			}
		}
		return fmt.Errorf("xbstream extraction failed: %v", err)
	}
	if logCtx != nil {
		logCtx.WriteLog("XBSTREAM", "xbstream extraction completed successfully")
	}
	return nil
}

// ExtractBackupStreamToStdout handles decompression only (for piping to xbstream)
// Returns reader that can be piped to xbstream
func ExtractBackupStreamToStdout(reader io.Reader, compressType string, parallel int, logCtx *LogContext) (io.Reader, *exec.Cmd, error) {
	if compressType == "zstd" {
		// Check zstd dependency
		if _, err := exec.LookPath("zstd"); err != nil {
			return nil, nil, fmt.Errorf("%s", i18n.Sprintf("zstd command not found. Please install zstd: https://github.com/facebook/zstd"))
		}

		if parallel == 0 {
			parallel = 4
		}

		if logCtx != nil {
			logCtx.WriteLog("DECOMPRESS", "Decompressing zstd stream to stdout")
		}

		// Decompress with zstd
		zstdCmd := exec.Command("zstd", "-d", fmt.Sprintf("-T%d", parallel), "-")
		zstdCmd.Stdin = reader
		if logCtx != nil {
			zstdCmd.Stderr = logCtx.GetFile()
		} else {
			zstdCmd.Stderr = os.Stderr
		}

		stdout, err := zstdCmd.StdoutPipe()
		if err != nil {
			return nil, nil, err
		}

		if err := zstdCmd.Start(); err != nil {
			if logCtx != nil {
				logCtx.WriteLog("DECOMPRESS", "Failed to start zstd: %v", err)
			}
			return nil, nil, fmt.Errorf("failed to start zstd decompression: %v", err)
		}

		return stdout, zstdCmd, nil
	}

	// No compression or qpress - return as-is
	// Note: qpress cannot be stream-decompressed, so user needs to save file first
	return reader, nil, nil
}

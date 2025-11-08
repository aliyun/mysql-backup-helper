package utils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogContext manages unified log file for all operations
type LogContext struct {
	logFile     *os.File
	logFileName string
	logDir      string
	completedOK bool // Flag to mark if operation completed successfully
}

// NewLogContext creates a new log context with backup-helper-{timestamp}.log
// If logFileName is provided, it will be used instead of auto-generated name
// logFileName can be:
//   - Empty string: auto-generate backup-helper-{timestamp}.log
//   - Relative path: will be joined with logDir
//   - Absolute path: will be used as-is (logDir will be ignored for this file)
//
// If logFileName is an absolute path and logDir is also specified, logDir will be ignored
// and a warning will be printed (if verbose logging is enabled)
func NewLogContext(logDir string, logFileName string) (*LogContext, error) {
	var finalLogFileName string
	originalLogDir := logDir // Store original for conflict detection

	if logFileName != "" {
		// Custom log file name provided
		if filepath.IsAbs(logFileName) {
			// Absolute path: use as-is, extract dir for cleanup
			finalLogFileName = logFileName
			// Check if logDir was also specified and differs from the logFileName's directory
			logFileNameDir := filepath.Dir(logFileName)
			if originalLogDir != "" && originalLogDir != logFileNameDir {
				// Conflict detected: logDir is specified but logFileName is absolute path
				// logDir will be ignored, use logFileName's directory instead
				fmt.Fprintf(os.Stderr, "[WARNING] logDir (%s) is specified but will be ignored because logFileName is an absolute path (%s). Using directory from logFileName: %s\n",
					originalLogDir, logFileName, logFileNameDir)
			}
			logDir = logFileNameDir
		} else {
			// Relative path: join with logDir
			if err := ensureLogsDir(logDir); err != nil {
				return nil, err
			}
			finalLogFileName = filepath.Join(logDir, logFileName)
		}
		// Ensure directory exists for custom file
		if err := ensureLogsDir(filepath.Dir(finalLogFileName)); err != nil {
			return nil, err
		}
	} else {
		// Auto-generate log file name
		if err := ensureLogsDir(logDir); err != nil {
			return nil, err
		}
		cleanOldLogs(logDir, 10)
		timestamp := time.Now().Format("20060102150405")
		finalLogFileName = filepath.Join(logDir, fmt.Sprintf("backup-helper-%s.log", timestamp))
	}

	logFile, err := os.Create(finalLogFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %v", err)
	}

	ctx := &LogContext{
		logFile:     logFile,
		logFileName: finalLogFileName,
		logDir:      logDir,
		completedOK: false, // Default to false, will be set to true on successful completion
	}

	// Write initial header
	timestampFormatted := time.Now().Format("2006-01-02 15:04:05")
	ctx.WriteLog("SYSTEM", "=== MySQL Backup Helper Log Started ===")
	ctx.WriteLog("SYSTEM", "Timestamp: %s", timestampFormatted)

	return ctx, nil
}

// WriteLog writes a log entry with [MODULE] prefix and timestamp
func (lc *LogContext) WriteLog(module string, format string, args ...interface{}) {
	if lc.logFile == nil {
		return
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logEntry := fmt.Sprintf("[%s] [%s] %s\n", timestamp, module, message)
	lc.logFile.WriteString(logEntry)
	lc.logFile.Sync()
}

// WriteCommandOutput writes command stderr/stdout to log
func (lc *LogContext) WriteCommandOutput(module string, data []byte) {
	if lc.logFile == nil || len(data) == 0 {
		return
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		logEntry := fmt.Sprintf("[%s] [%s] %s\n", timestamp, module, line)
		lc.logFile.WriteString(logEntry)
	}
	lc.logFile.Sync()
}

// GetFile returns the underlying file for direct writing (e.g., redirecting command output)
func (lc *LogContext) GetFile() *os.File {
	return lc.logFile
}

// GetFileName returns the log file path
func (lc *LogContext) GetFileName() string {
	return lc.logFileName
}

// MarkSuccess marks the operation as successfully completed
// This will cause "completed OK!" to be written to the log before "Log Ended"
func (lc *LogContext) MarkSuccess() {
	lc.completedOK = true
}

// Close closes the log file
// If MarkSuccess() was called, it will write "completed OK!" before "Log Ended"
func (lc *LogContext) Close() {
	if lc.logFile != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		if lc.completedOK {
			lc.logFile.WriteString(fmt.Sprintf("[%s] [SYSTEM] completed OK!\n", timestamp))
		}
		lc.logFile.WriteString(fmt.Sprintf("[%s] [SYSTEM] === MySQL Backup Helper Log Ended ===\n", timestamp))
		lc.logFile.Close()
		lc.logFile = nil
	}
}

// ExtractErrorSummary extracts error summary from log content based on module type
func ExtractErrorSummary(module string, logContent string) string {
	if logContent == "" {
		return ""
	}

	lines := strings.Split(logContent, "\n")
	errorLines := []string{}

	switch module {
	case "BACKUP":
		// Check if "completed OK!" exists
		if !strings.Contains(logContent, "completed OK!") {
			// Extract last 20 lines containing "error" or "failed"
			for i := len(lines) - 1; i >= 0 && len(errorLines) < 20; i-- {
				line := strings.ToLower(lines[i])
				if strings.Contains(line, "error") || strings.Contains(line, "failed") ||
					strings.Contains(line, "fatal") || strings.Contains(line, "critical") {
					errorLines = append([]string{lines[i]}, errorLines...)
				}
			}
			// If no error lines found, get last 20 lines
			if len(errorLines) == 0 {
				start := len(lines) - 20
				if start < 0 {
					start = 0
				}
				errorLines = lines[start:]
			}
		}

	case "PREPARE":
		// Extract last 20 lines containing "error" or "failed"
		for i := len(lines) - 1; i >= 0 && len(errorLines) < 20; i-- {
			line := strings.ToLower(lines[i])
			if strings.Contains(line, "error") || strings.Contains(line, "failed") ||
				strings.Contains(line, "fatal") || strings.Contains(line, "critical") {
				errorLines = append([]string{lines[i]}, errorLines...)
			}
		}
		// If no error lines found, get last 20 lines
		if len(errorLines) == 0 {
			start := len(lines) - 20
			if start < 0 {
				start = 0
			}
			errorLines = lines[start:]
		}

	case "TCP", "OSS":
		// Extract connection/network errors
		for i := len(lines) - 1; i >= 0 && len(errorLines) < 20; i-- {
			line := strings.ToLower(lines[i])
			if strings.Contains(line, "error") || strings.Contains(line, "failed") ||
				strings.Contains(line, "timeout") || strings.Contains(line, "connection") ||
				strings.Contains(line, "refused") {
				errorLines = append([]string{lines[i]}, errorLines...)
			}
		}
		if len(errorLines) == 0 {
			start := len(lines) - 20
			if start < 0 {
				start = 0
			}
			errorLines = lines[start:]
		}

	case "DECOMPRESS", "EXTRACT", "XBSTREAM":
		// Extract command output errors
		for i := len(lines) - 1; i >= 0 && len(errorLines) < 20; i-- {
			line := strings.ToLower(lines[i])
			if strings.Contains(line, "error") || strings.Contains(line, "failed") ||
				strings.Contains(line, "fatal") || strings.Contains(line, "cannot") ||
				strings.Contains(line, "unable") {
				errorLines = append([]string{lines[i]}, errorLines...)
			}
		}
		if len(errorLines) == 0 {
			start := len(lines) - 20
			if start < 0 {
				start = 0
			}
			errorLines = lines[start:]
		}

	default:
		// Default: get last 20 lines
		start := len(lines) - 20
		if start < 0 {
			start = 0
		}
		errorLines = lines[start:]
	}

	if len(errorLines) == 0 {
		return ""
	}

	return strings.Join(errorLines, "\n")
}

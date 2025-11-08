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
}

// NewLogContext creates a new log context with backup-helper-{timestamp}.log
// If logFileName is provided, it will be used instead of auto-generated name
// logFileName can be:
//   - Empty string: auto-generate backup-helper-{timestamp}.log
//   - Relative path: will be joined with logDir
//   - Absolute path: will be used as-is (logDir will be ignored for this file)
func NewLogContext(logDir string, logFileName string) (*LogContext, error) {
	var finalLogFileName string

	if logFileName != "" {
		// Custom log file name provided
		if filepath.IsAbs(logFileName) {
			// Absolute path: use as-is, extract dir for cleanup
			finalLogFileName = logFileName
			logDir = filepath.Dir(logFileName)
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

// Close closes the log file
func (lc *LogContext) Close() {
	if lc.logFile != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
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

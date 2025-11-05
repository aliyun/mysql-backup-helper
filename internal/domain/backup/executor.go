package backup

import (
	"backup-helper/internal/config"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gioco-play/easy-i18n/i18n"
)

// Executor handles xtrabackup execution
type Executor struct {
	cfg *config.Config
}

// NewExecutor creates a new backup executor
func NewExecutor(cfg *config.Config) *Executor {
	return &Executor{cfg: cfg}
}

// Execute runs xtrabackup and returns a reader for the backup stream
func (e *Executor) Execute() (io.Reader, *exec.Cmd, string, error) {
	if err := ensureLogsDir(e.cfg.LogDir); err != nil {
		return nil, nil, "", err
	}
	cleanOldLogs(e.cfg.LogDir, 10)

	args := []string{
		"--backup",
		fmt.Sprintf("--host=%s", e.cfg.MysqlHost),
		fmt.Sprintf("--port=%d", e.cfg.MysqlPort),
		fmt.Sprintf("--user=%s", e.cfg.MysqlUser),
		fmt.Sprintf("--password=%s", e.cfg.MysqlPassword),
		"--stream=xbstream",
		"--backup-lock-timeout=120",
		"--backup-lock-retry-count=0",
		"--close-files=0",
		"--ftwrl-wait-timeout=60",
		"--ftwrl-wait-threshold=60",
		"--ftwrl-wait-query-type=ALL",
		"--kill-long-queries-timeout=0",
		"--kill-long-query-type=SELECT",
		"--lock-ddl=0",
	}

	var cmd *exec.Cmd
	if e.cfg.CompressType == "zstd" {
		// Check zstd dependency
		if _, err := exec.LookPath("zstd"); err != nil {
			return nil, nil, "", fmt.Errorf("%s", i18n.Sprintf("zstd command not found. Please install zstd: https://github.com/facebook/zstd"))
		}
		// Print equivalent shell command
		cmdStr := "xtrabackup " + strings.Join(args, " ") + " | zstd -q -"
		i18n.Printf("Equivalent shell command: %s\n", cmdStr)
		// Use pipe method: xtrabackup ... | zstd
		xtrabackupCmd := exec.Command("xtrabackup", args...)
		zstdCmd := exec.Command("zstd", "-q", "-")

		logFileName := getLogFileName(e.cfg.LogDir)
		logFile, err := os.Create(logFileName)
		if err != nil {
			return nil, nil, "", err
		}
		xtrabackupCmd.Stderr = logFile
		zstdCmd.Stderr = logFile

		// Connect pipe
		pipe, err := xtrabackupCmd.StdoutPipe()
		if err != nil {
			logFile.Close()
			return nil, nil, "", err
		}
		zstdCmd.Stdin = pipe

		// Use zstd command as the main command
		cmd = zstdCmd
		cmd.ExtraFiles = append(cmd.ExtraFiles, xtrabackupCmd.ExtraFiles...)
		cmd.Env = append(cmd.Env, xtrabackupCmd.Env...)

		// Start xtrabackup
		if err := xtrabackupCmd.Start(); err != nil {
			logFile.Close()
			return nil, nil, "", err
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logFile.Close()
			return nil, nil, "", err
		}

		if err := cmd.Start(); err != nil {
			logFile.Close()
			return nil, nil, "", err
		}
		// Note: Caller needs to logFile.Close() after cmd.Wait()
		return stdout, cmd, logFileName, nil
	}

	// Non-zstd branch, always assign cmd
	if e.cfg.Compress {
		args = append(args, "--compress")
	}
	cmd = exec.Command("xtrabackup", args...)

	cmdStr := "xtrabackup " + strings.Join(args, " ")
	i18n.Printf("Equivalent shell command: %s\n", cmdStr)

	logFileName := getLogFileName(e.cfg.LogDir)
	logFile, err := os.Create(logFileName)
	if err != nil {
		return nil, nil, "", err
	}
	cmd.Stderr = logFile

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logFile.Close()
		return nil, nil, "", err
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, nil, "", err
	}
	return stdout, cmd, logFileName, nil
}

// CloseLogFile closes cmd's Stderr log file (if it's *os.File)
func CloseLogFile(cmd *exec.Cmd) {
	if cmd == nil || cmd.Stderr == nil {
		return
	}
	if f, ok := cmd.Stderr.(*os.File); ok {
		f.Close()
	}
}

// ensureLogsDir creates log directory if it doesn't exist
func ensureLogsDir(logDir string) error {
	// if log directory is relative path, make it relative to current working directory
	if !filepath.IsAbs(logDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %v", err)
		}
		logDir = filepath.Join(cwd, logDir)
	}

	// create log directory
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory %s: %v", logDir, err)
	}

	return nil
}

// getLogFileName generates a log file name with timestamp
func getLogFileName(logDir string) string {
	timestamp := time.Now().Format("20060102150405")
	return filepath.Join(logDir, fmt.Sprintf("xtrabackup-%s.log", timestamp))
}

// cleanOldLogs removes old log files, keeping only the most recent ones
func cleanOldLogs(logDir string, keep int) error {
	pattern := filepath.Join(logDir, "xtrabackup-*.log")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob log files: %v", err)
	}

	if len(files) <= keep {
		return nil
	}

	// sort by modification time, keep the latest files
	sort.Slice(files, func(i, j int) bool {
		infoI, _ := os.Stat(files[i])
		infoJ, _ := os.Stat(files[j])
		if infoI == nil || infoJ == nil {
			return false
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})

	// remove old files
	for _, f := range files[keep:] {
		if err := os.Remove(f); err != nil {
			// log removal failure but don't interrupt the process
			fmt.Printf("Warning: failed to remove old log file %s: %v\n", f, err)
		}
	}

	return nil
}

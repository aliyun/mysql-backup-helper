package utils

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/gioco-play/easy-i18n/i18n"
)

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

func getLogFileName(logDir string) string {
	timestamp := time.Now().Format("20060102150405")
	return filepath.Join(logDir, fmt.Sprintf("xtrabackup-%s.log", timestamp))
}

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
	for _, f := range files[:len(files)-keep] {
		if err := os.Remove(f); err != nil {
			// log removal failure but don't interrupt the process
			fmt.Printf("Warning: failed to remove old log file %s: %v\n", f, err)
		}
	}

	return nil
}

// RunXtraBackup calls xtrabackup, returns backup data io.Reader, cmd, log file name and error
// db is used to get MySQL config file path and must be a valid MySQL connection
func RunXtraBackup(cfg *Config, db *sql.DB) (io.Reader, *exec.Cmd, string, error) {
	if err := ensureLogsDir(cfg.LogDir); err != nil {
		return nil, nil, "", err
	}
	cleanOldLogs(cfg.LogDir, 10)

	// Check for MySQL config file first (must be first argument if present)
	var defaultsFile string
	if db != nil {
		defaultsFile = GetMySQLConfigFile(db)
	}

	args := []string{
		"--backup",
		fmt.Sprintf("--host=%s", cfg.MysqlHost),
		fmt.Sprintf("--port=%d", cfg.MysqlPort),
		fmt.Sprintf("--user=%s", cfg.MysqlUser),
		fmt.Sprintf("--password=%s", cfg.MysqlPassword),
		"--stream=xbstream",
		"--backup-lock-timeout=120",
		"--backup-lock-retry-count=0",
		"--close-files=1", // Enable close-files to handle large number of tables
		"--ftwrl-wait-timeout=60",
		"--ftwrl-wait-threshold=60",
		"--ftwrl-wait-query-type=ALL",
		"--kill-long-queries-timeout=0",
		"--kill-long-query-type=SELECT",
		"--lock-ddl=0",
	}

	// Prepend --defaults-file if config file is found (must be first argument)
	if defaultsFile != "" {
		args = append([]string{fmt.Sprintf("--defaults-file=%s", defaultsFile)}, args...)
	}

	// Add --parallel (default is 4)
	parallel := cfg.Parallel
	if parallel == 0 {
		parallel = 4
	}
	args = append(args, fmt.Sprintf("--parallel=%d", parallel))

	// Set ulimit for file descriptors (655360)
	// Set the limit for current process, child processes will inherit
	var rlimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err == nil {
		if rlimit.Cur < 655360 {
			rlimit.Cur = 655360
			if rlimit.Max < 655360 {
				rlimit.Max = 655360
			}
			// Try to set the limit (may fail if not enough privileges)
			syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlimit)
		}
	}

	var cmd *exec.Cmd
	if cfg.CompressType == "zstd" {
		// Check zstd dependency
		if _, err := exec.LookPath("zstd"); err != nil {
			return nil, nil, "", fmt.Errorf("%s", i18n.Sprintf("zstd command not found. Please install zstd: https://github.com/facebook/zstd"))
		}
		// Get parallel value for zstd compression
		parallel := cfg.Parallel
		if parallel == 0 {
			parallel = 4
		}
		// Print equivalent shell command
		cmdStr := fmt.Sprintf("xtrabackup %s | zstd -q -T%d -", strings.Join(args, " "), parallel)
		i18n.Printf("Equivalent shell command: %s\n", cmdStr)
		// Use pipe method: xtrabackup ... | zstd -T<parallel>
		xtrabackupCmd := exec.Command("xtrabackup", args...)
		zstdCmd := exec.Command("zstd", "-q", fmt.Sprintf("-T%d", parallel), "-")

		logFileName := getLogFileName(cfg.LogDir)
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
	// Use cfg.CompressType == "qp" to determine if we need --compress
	if cfg.CompressType == "qp" {
		args = append(args, "--compress")
		// Add --compress-threads for parallel compression
		args = append(args, fmt.Sprintf("--compress-threads=%d", parallel))
	}
	cmd = exec.Command("xtrabackup", args...)

	cmdStr := "xtrabackup " + strings.Join(args, " ")
	i18n.Printf("Equivalent shell command: %s\n", cmdStr)

	logFileName := getLogFileName(cfg.LogDir)
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

// CloseBackupLogFile closes cmd's Stderr log file (if it's *os.File)
func CloseBackupLogFile(cmd *exec.Cmd) {
	if cmd == nil || cmd.Stderr == nil {
		return
	}
	if f, ok := cmd.Stderr.(*os.File); ok {
		f.Close()
	}
}

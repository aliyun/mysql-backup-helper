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

func cleanOldLogs(logDir string, keep int) error {
	pattern := filepath.Join(logDir, "backup-helper-*.log")
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

// RunXtraBackup calls xtrabackup, returns backup data io.Reader, cmd and error
// db is used to get MySQL config file path and must be a valid MySQL connection
// logCtx is used to write logs for backup operations
func RunXtraBackup(cfg *Config, db *sql.DB, logCtx *LogContext) (io.Reader, *exec.Cmd, error) {
	if logCtx == nil {
		return nil, nil, fmt.Errorf("log context is required")
	}

	// Resolve xtrabackup and xbstream paths
	xtrabackupPath, _, err := ResolveXtrabackupPath(cfg, true)
	if err != nil {
		return nil, nil, err
	}

	// Check for MySQL config file first (must be first argument if present)
	// Only use defaults-file if explicitly specified by user (via --defaults-file or config)
	// We do NOT auto-detect to avoid using wrong config file (e.g., from another MySQL instance)
	var defaultsFile string
	if cfg.DefaultsFile != "" {
		// Use explicitly specified defaults-file
		defaultsFile = cfg.DefaultsFile
	}

	args := []string{
		"--backup",
		fmt.Sprintf("--host=%s", cfg.MysqlHost),
		fmt.Sprintf("--port=%d", cfg.MysqlPort),
		fmt.Sprintf("--user=%s", cfg.MysqlUser),
		fmt.Sprintf("--password=%s", cfg.MysqlPassword),
		"--stream=xbstream",
		"--slave-info", // Record master binary log position for replication setup
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
			return nil, nil, fmt.Errorf("%s", i18n.Sprintf("zstd command not found. Please install zstd: https://github.com/facebook/zstd"))
		}
		// Get parallel value for zstd compression
		parallel := cfg.Parallel
		if parallel == 0 {
			parallel = 4
		}
		// Print equivalent shell command
		cmdStr := fmt.Sprintf("%s %s | zstd -q -T%d -", xtrabackupPath, strings.Join(args, " "), parallel)
		i18n.Printf("Equivalent shell command: %s\n", cmdStr)
		logCtx.WriteLog("BACKUP", "Starting xtrabackup backup with zstd compression")
		logCtx.WriteLog("BACKUP", "Command: %s", cmdStr)
		// Use pipe method: xtrabackup ... | zstd -T<parallel>
		xtrabackupCmd := exec.Command(xtrabackupPath, args...)
		zstdCmd := exec.Command("zstd", "-q", fmt.Sprintf("-T%d", parallel), "-")

		xtrabackupCmd.Stderr = logCtx.GetFile()
		zstdCmd.Stderr = logCtx.GetFile()

		// Connect pipe
		pipe, err := xtrabackupCmd.StdoutPipe()
		if err != nil {
			logCtx.WriteLog("BACKUP", "Failed to create pipe: %v", err)
			return nil, nil, err
		}
		zstdCmd.Stdin = pipe

		// Use zstd command as the main command
		cmd = zstdCmd
		cmd.ExtraFiles = append(cmd.ExtraFiles, xtrabackupCmd.ExtraFiles...)
		cmd.Env = append(cmd.Env, xtrabackupCmd.Env...)

		// Start xtrabackup
		if err := xtrabackupCmd.Start(); err != nil {
			logCtx.WriteLog("BACKUP", "Failed to start xtrabackup: %v", err)
			return nil, nil, err
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logCtx.WriteLog("BACKUP", "Failed to create stdout pipe: %v", err)
			return nil, nil, err
		}

		if err := cmd.Start(); err != nil {
			logCtx.WriteLog("BACKUP", "Failed to start zstd: %v", err)
			return nil, nil, err
		}
		logCtx.WriteLog("BACKUP", "xtrabackup and zstd processes started successfully")
		return stdout, cmd, nil
	}

	// Non-zstd branch, always assign cmd
	// Use cfg.CompressType == "qp" to determine if we need --compress
	if cfg.CompressType == "qp" {
		args = append(args, "--compress")
		// Add --compress-threads for parallel compression
		args = append(args, fmt.Sprintf("--compress-threads=%d", parallel))
	}
	cmd = exec.Command(xtrabackupPath, args...)

	cmdStr := xtrabackupPath + " " + strings.Join(args, " ")
	i18n.Printf("Equivalent shell command: %s\n", cmdStr)
	logCtx.WriteLog("BACKUP", "Starting xtrabackup backup")
	if cfg.CompressType == "qp" {
		logCtx.WriteLog("BACKUP", "Using qpress compression")
	} else {
		logCtx.WriteLog("BACKUP", "No compression")
	}
	logCtx.WriteLog("BACKUP", "Command: %s", cmdStr)
	cmd.Stderr = logCtx.GetFile()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logCtx.WriteLog("BACKUP", "Failed to create stdout pipe: %v", err)
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		logCtx.WriteLog("BACKUP", "Failed to start xtrabackup: %v", err)
		return nil, nil, err
	}
	logCtx.WriteLog("BACKUP", "xtrabackup process started successfully")
	return stdout, cmd, nil
}

// RunXtrabackupPrepare executes xtrabackup --prepare on a backup directory
// targetDir: directory containing the backup to prepare
// cfg: configuration containing parallel and useMemory settings
// db: optional MySQL connection for getting defaults-file (can be nil for prepare)
// logCtx: log context for writing logs
func RunXtrabackupPrepare(cfg *Config, targetDir string, db *sql.DB, logCtx *LogContext) (*exec.Cmd, error) {
	if logCtx == nil {
		return nil, fmt.Errorf("log context is required")
	}

	// Resolve xtrabackup path
	xtrabackupPath, _, err := ResolveXtrabackupPath(cfg, false)
	if err != nil {
		return nil, err
	}

	// Check for MySQL config file first (must be first argument if present)
	// Only use defaults-file if explicitly specified by user (via --defaults-file or config)
	// We do NOT auto-detect to avoid using wrong config file (e.g., from another MySQL instance)
	var defaultsFile string
	if cfg.DefaultsFile != "" {
		// Use explicitly specified defaults-file
		defaultsFile = cfg.DefaultsFile
	}

	args := []string{
		"--prepare",
		fmt.Sprintf("--target-dir=%s", targetDir),
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

	// Add --use-memory (default is 1G)
	useMemory := cfg.UseMemory
	if useMemory == "" {
		useMemory = "1G"
	}
	args = append(args, fmt.Sprintf("--use-memory=%s", useMemory))

	// Set ulimit for file descriptors (655360)
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

	cmd := exec.Command(xtrabackupPath, args...)

	cmdStr := xtrabackupPath + " " + strings.Join(args, " ")
	i18n.Printf("Equivalent shell command: %s\n", cmdStr)
	logCtx.WriteLog("PREPARE", "Starting xtrabackup prepare")
	logCtx.WriteLog("PREPARE", "Target directory: %s", targetDir)
	logCtx.WriteLog("PREPARE", "Command: %s", cmdStr)
	cmd.Stderr = logCtx.GetFile()
	cmd.Stdout = logCtx.GetFile()

	if err := cmd.Start(); err != nil {
		logCtx.WriteLog("PREPARE", "Failed to start xtrabackup prepare: %v", err)
		return nil, err
	}
	logCtx.WriteLog("PREPARE", "xtrabackup prepare process started successfully")
	return cmd, nil
}

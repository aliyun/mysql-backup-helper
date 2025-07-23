package utils

import (
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

// Config 增加 Compress 字段
// type Config struct {
//   ...
//   Compress bool `json:"compress"`
// }

func ensureLogsDir() error {
	return os.MkdirAll("logs", 0755)
}

func getLogFileName() string {
	return filepath.Join("logs", fmt.Sprintf("xtrabackup-%s.log", time.Now().Format("20060102150405")))
}

func cleanOldLogs(keep int) error {
	files, err := filepath.Glob("logs/xtrabackup-*.log")
	if err != nil {
		return err
	}
	if len(files) <= keep {
		return nil
	}
	sort.Strings(files)
	for _, f := range files[:len(files)-keep] {
		os.Remove(f)
	}
	return nil
}

// RunXtraBackup 调用 xtrabackup，返回备份数据的 io.Reader、cmd、日志文件名和错误
func RunXtraBackup(cfg *Config) (io.Reader, *exec.Cmd, string, error) {
	if err := ensureLogsDir(); err != nil {
		return nil, nil, "", err
	}
	cleanOldLogs(10)

	args := []string{
		"--backup",
		fmt.Sprintf("--host=%s", cfg.MysqlHost),
		fmt.Sprintf("--port=%d", cfg.MysqlPort),
		fmt.Sprintf("--user=%s", cfg.MysqlUser),
		fmt.Sprintf("--password=%s", cfg.MysqlPassword),
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
	if cfg.CompressType == "zstd" {
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

		logFileName := getLogFileName()
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
	if cfg.Compress {
		args = append(args, "--compress")
	}
	cmd = exec.Command("xtrabackup", args...)

	cmdStr := "xtrabackup " + strings.Join(args, " ")
	i18n.Printf("Equivalent shell command: %s\n", cmdStr)

	logFileName := getLogFileName()
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

// CloseBackupLogFile 关闭cmd的Stderr日志文件（如果是*os.File）
func CloseBackupLogFile(cmd *exec.Cmd) {
	if cmd == nil || cmd.Stderr == nil {
		return
	}
	if f, ok := cmd.Stderr.(*os.File); ok {
		f.Close()
	}
}

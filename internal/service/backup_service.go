package service

import (
	"backup-helper/internal/config"
	"backup-helper/internal/domain/backup"
	"backup-helper/internal/domain/mysql"
	"backup-helper/internal/infrastructure/storage/oss"
	"backup-helper/internal/infrastructure/stream"
	"backup-helper/internal/pkg/format"
	"backup-helper/internal/pkg/ratelimit"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gioco-play/easy-i18n/i18n"
)

// BackupService handles the complete backup workflow
type BackupService struct {
	cfg *config.Config
}

// NewBackupService creates a new backup service
func NewBackupService(cfg *config.Config) *BackupService {
	return &BackupService{cfg: cfg}
}

// BackupOptions contains options for backup execution
type BackupOptions struct {
	Mode       string // "oss" or "stream"
	StreamPort int
	EnableAuth bool
	AuthKey    string
}

// Execute performs the complete backup workflow
func (s *BackupService) Execute(opts *BackupOptions) error {
	// 1. Validate MySQL connection and parameters
	conn, err := s.validateMySQLConnection()
	if err != nil {
		return err
	}
	defer conn.Close()

	// 2. Display IO limit information
	s.displayIOLimit()

	// 3. Check xtrabackup compatibility
	s.checkXtraBackupVersion()

	// 4. Execute xtrabackup and get reader
	reader, xtraCmd, logFileName, err := s.executeXtraBackup()
	if err != nil {
		return err
	}

	// 5. Calculate total size for progress tracking (auto-detect)
	totalSize := s.calculateTotalSize(conn)

	// 6. Determine object name for upload
	fullObjectName := s.determineObjectName(opts.Mode)

	// 7. Transfer backup data
	transferErr := s.transferBackup(opts, reader, xtraCmd, fullObjectName, totalSize)
	if transferErr != nil {
		return transferErr
	}

	// 8. Wait for xtrabackup to complete
	xtraCmd.Wait()
	backup.CloseLogFile(xtraCmd)

	// 9. Validate backup result
	return s.validateBackupResult(logFileName)
}

// validateMySQLConnection validates MySQL connection and parameters
func (s *BackupService) validateMySQLConnection() (*mysql.Connection, error) {
	conn, err := mysql.NewConnection(s.cfg.MysqlHost, s.cfg.MysqlPort, s.cfg.MysqlUser, s.cfg.MysqlPassword)
	if err != nil {
		return nil, fmt.Errorf("MySQL connection error: %v", err)
	}

	checker := mysql.NewChecker(conn)
	if err := checker.CheckAll(s.cfg); err != nil {
		conn.Close()
		return nil, fmt.Errorf("parameter check error: %v", err)
	}

	return conn, nil
}

// displayIOLimit displays IO rate limit information
func (s *BackupService) displayIOLimit() {
	if s.cfg.Traffic == 0 {
		i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
	} else {
		i18n.Printf("[backup-helper] IO rate limit set to: %s/s\n", format.Bytes(s.cfg.Traffic))
	}
}

// checkXtraBackupVersion checks xtrabackup compatibility
func (s *BackupService) checkXtraBackupVersion() {
	mysqlVer := mysql.Version{
		Major: s.cfg.MysqlVersion.Major,
		Minor: s.cfg.MysqlVersion.Minor,
		Micro: s.cfg.MysqlVersion.Micro,
	}
	mysql.CheckXtraBackupCompatibility(mysqlVer)
}

// executeXtraBackup executes xtrabackup and returns reader
func (s *BackupService) executeXtraBackup() (io.Reader, *exec.Cmd, string, error) {
	i18n.Printf("[backup-helper] Running xtrabackup...\n")
	executor := backup.NewExecutor(s.cfg)
	return executor.Execute()
}

// calculateTotalSize calculates total backup size by detecting datadir size
func (s *BackupService) calculateTotalSize(conn *mysql.Connection) int64 {
	datadir, err := conn.GetDatadir()
	if err != nil {
		i18n.Printf("Warning: Could not get datadir, progress tracking will be limited: %v\n", err)
		return 0
	}

	totalSize, err := backup.CalculateDatadirSize(datadir)
	if err != nil {
		i18n.Printf("Warning: Could not calculate backup size, progress tracking will be limited: %v\n", err)
		return 0
	}

	i18n.Printf("[backup-helper] Calculated datadir size: %s\n", format.Bytes(totalSize))
	return totalSize
}

// determineObjectName determines the full object name based on mode and compression
func (s *BackupService) determineObjectName(mode string) string {
	objectSuffix := ".xb"
	if mode == "stream" {
		s.cfg.Compress = false
		s.cfg.CompressType = ""
	} else if s.cfg.Compress {
		switch s.cfg.CompressType {
		case "zstd":
			objectSuffix = ".xb.zst"
		default:
			objectSuffix = "_qp.xb"
		}
	}
	timestamp := time.Now().Format("_20060102150405")
	return s.cfg.ObjectName + timestamp + objectSuffix
}

// transferBackup transfers backup data based on mode
func (s *BackupService) transferBackup(opts *BackupOptions, reader io.Reader, xtraCmd *exec.Cmd, fullObjectName string, totalSize int64) error {
	switch opts.Mode {
	case "oss":
		return s.transferToOSS(reader, xtraCmd, fullObjectName, totalSize)
	case "stream":
		return s.transferToStream(opts, reader, xtraCmd, totalSize)
	default:
		return fmt.Errorf("unknown mode: %s", opts.Mode)
	}
}

// transferToOSS uploads backup to OSS
func (s *BackupService) transferToOSS(reader io.Reader, xtraCmd *exec.Cmd, fullObjectName string, totalSize int64) error {
	i18n.Printf("[backup-helper] Uploading to OSS...\n")
	uploader := oss.NewUploader(s.cfg)
	err := uploader.Upload(nil, reader, fullObjectName, totalSize)
	if err != nil {
		i18n.Printf("OSS upload error: %v\n", err)
		if xtraCmd != nil && xtraCmd.Process != nil {
			xtraCmd.Process.Kill()
		}
		return err
	}
	return nil
}

// transferToStream streams backup via TCP
func (s *BackupService) transferToStream(opts *BackupOptions, reader io.Reader, xtraCmd *exec.Cmd, totalSize int64) error {
	sender := stream.NewSender(opts.StreamPort, opts.EnableAuth, opts.AuthKey, totalSize)
	tcpWriter, _, closer, _, _, err := sender.Start()
	if err != nil {
		return fmt.Errorf("stream server error: %v", err)
	}
	defer closer()

	// Apply rate limiting
	writer := io.Writer(tcpWriter)
	if s.cfg.Traffic > 0 {
		rateLimitedWriter := ratelimit.NewWriter(tcpWriter, s.cfg.Traffic)
		writer = rateLimitedWriter
	}

	_, err = io.Copy(writer, reader)
	if err != nil {
		i18n.Printf("TCP stream error: %v\n", err)
		if xtraCmd != nil && xtraCmd.Process != nil {
			xtraCmd.Process.Kill()
		}
		return err
	}
	return nil
}

// validateBackupResult validates backup completion
func (s *BackupService) validateBackupResult(logFileName string) error {
	logContent, err := os.ReadFile(logFileName)
	if err != nil {
		return fmt.Errorf("backup log read error")
	}

	if !strings.Contains(string(logContent), "completed OK!") {
		i18n.Printf("Backup failed (no 'completed OK!').\n")
		i18n.Printf("You can check the backup log file for details: %s\n", logFileName)
		i18n.Printf("\n💡 Tip: Use AI to diagnose the issue:\n")
		i18n.Printf("   mysql-backup-helper ai --log-file %s\n", logFileName)
		return fmt.Errorf("backup failed")
	}

	i18n.Printf("[backup-helper] Backup and upload completed!\n")
	return nil
}

package service

import (
	"backup-helper/internal/config"
	"backup-helper/internal/domain/backup"
	"backup-helper/internal/domain/mysql"
	"backup-helper/internal/infrastructure/ai"
	"backup-helper/internal/infrastructure/storage/oss"
	"backup-helper/internal/infrastructure/stream"
	"backup-helper/internal/pkg/format"
	"backup-helper/internal/pkg/ratelimit"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
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
	Mode            string // "oss" or "stream"
	StreamPort      int
	EstimatedSize   int64
	EnableHandshake bool
	StreamKey       string
	AIDiagnose      string // "on", "off", or "auto"
}

// Execute performs the complete backup workflow
func (s *BackupService) Execute(opts *BackupOptions) error {
	// 1. Connect to MySQL
	conn, err := mysql.NewConnection(s.cfg.MysqlHost, s.cfg.MysqlPort, s.cfg.MysqlUser, s.cfg.MysqlPassword)
	if err != nil {
		return fmt.Errorf("MySQL connection error: %v", err)
	}
	defer conn.Close()

	// 2. Check MySQL parameters
	checker := mysql.NewChecker(conn)
	if err := checker.CheckAll(s.cfg); err != nil {
		return fmt.Errorf("parameter check error: %v", err)
	}

	// 3. Display IO limit
	if s.cfg.Traffic == 0 {
		i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
	} else {
		i18n.Printf("[backup-helper] IO rate limit set to: %s/s\n", format.Bytes(s.cfg.Traffic))
	}

	// 4. Check xtrabackup version
	mysqlVer := mysql.Version{
		Major: s.cfg.MysqlVersion.Major,
		Minor: s.cfg.MysqlVersion.Minor,
		Micro: s.cfg.MysqlVersion.Micro,
	}
	mysql.CheckXtraBackupCompatibility(mysqlVer)

	i18n.Printf("[backup-helper] Running xtrabackup...\n")

	// 5. Execute xtrabackup
	executor := backup.NewExecutor(s.cfg)
	reader, xtraCmd, logFileName, err := executor.Execute()
	if err != nil {
		return fmt.Errorf("run xtrabackup error: %v", err)
	}

	// 6. Calculate total size for progress tracking
	var totalSize int64
	if opts.EstimatedSize > 0 {
		totalSize = opts.EstimatedSize
		i18n.Printf("[backup-helper] Using estimated size: %s\n", format.Bytes(totalSize))
	} else {
		datadir, err := conn.GetDatadir()
		if err != nil {
			i18n.Printf("Warning: Could not get datadir, progress tracking will be limited: %v\n", err)
		} else {
			totalSize, err = backup.CalculateDatadirSize(datadir)
			if err != nil {
				i18n.Printf("Warning: Could not calculate backup size, progress tracking will be limited: %v\n", err)
				totalSize = 0
			} else {
				i18n.Printf("[backup-helper] Calculated datadir size: %s\n", format.Bytes(totalSize))
			}
		}
	}

	// 7. Determine object name
	objectSuffix := ".xb"
	if opts.Mode == "stream" {
		s.cfg.Compress = false
		s.cfg.CompressType = ""
		objectSuffix = ".xb"
	} else if s.cfg.Compress {
		switch s.cfg.CompressType {
		case "zstd":
			objectSuffix = ".xb.zst"
		default:
			objectSuffix = "_qp.xb"
		}
	}
	timestamp := time.Now().Format("_20060102150405")
	fullObjectName := s.cfg.ObjectName + timestamp + objectSuffix

	// 8. Execute backup based on mode
	var transferErr error
	switch opts.Mode {
	case "oss":
		i18n.Printf("[backup-helper] Uploading to OSS...\n")
		uploader := oss.NewUploader(s.cfg)
		transferErr = uploader.Upload(nil, reader, fullObjectName, totalSize)
		if transferErr != nil {
			i18n.Printf("OSS upload error: %v\n", transferErr)
			xtraCmd.Process.Kill()
		}
	case "stream":
		sender := stream.NewSender(opts.StreamPort, opts.EnableHandshake, opts.StreamKey, totalSize)
		tcpWriter, _, closer, _, _, err := sender.Start()
		if err != nil {
			return fmt.Errorf("stream server error: %v", err)
		}
		defer closer()

		// Apply rate limiting for stream mode
		writer := tcpWriter
		if s.cfg.Traffic > 0 {
			rateLimitedWriter := ratelimit.NewWriter(tcpWriter, s.cfg.Traffic)
			writer = rateLimitedWriter
		}

		_, transferErr = io.Copy(writer, reader)
		if transferErr != nil {
			i18n.Printf("TCP stream error: %v\n", transferErr)
			xtraCmd.Process.Kill()
		}
	default:
		return fmt.Errorf("unknown mode: %s", opts.Mode)
	}

	if transferErr != nil {
		return transferErr
	}

	// 9. Wait for xtrabackup to complete
	xtraCmd.Wait()
	backup.CloseLogFile(xtraCmd)

	// 10. Check backup log
	logContent, err := os.ReadFile(logFileName)
	if err != nil {
		return fmt.Errorf("backup log read error")
	}

	if !strings.Contains(string(logContent), "completed OK!") {
		i18n.Printf("Backup failed (no 'completed OK!').\n")
		i18n.Printf("You can check the backup log file for details: %s\n", logFileName)

		// AI diagnosis
		if opts.AIDiagnose == "" {
			opts.AIDiagnose = "auto"
		}

		switch opts.AIDiagnose {
		case "on":
			if s.cfg.QwenAPIKey == "" {
				i18n.Printf("Qwen API Key is required for AI diagnosis. Please set it in config.\n")
				return fmt.Errorf("backup failed")
			}
			s.runAIDiagnosis(string(logContent))
		case "off":
			// do nothing
		default:
			var input string
			i18n.Printf("Would you like to use AI diagnosis? (y/n): ")
			fmt.Scanln(&input)
			if input == "y" || input == "Y" || input == "yes" || input == "Yes" {
				s.runAIDiagnosis(string(logContent))
			}
		}
		return fmt.Errorf("backup failed")
	}

	i18n.Printf("[backup-helper] Backup and upload completed!\n")
	return nil
}

// runAIDiagnosis runs AI diagnosis on backup log
func (s *BackupService) runAIDiagnosis(logContent string) {
	qwenClient := ai.NewQwenClient(s.cfg.QwenAPIKey)
	aiSuggestion, err := qwenClient.Diagnose(logContent)
	if err != nil {
		i18n.Printf("AI diagnosis failed: %v\n", err)
	} else {
		fmt.Print(color.YellowString(i18n.Sprintf("AI diagnosis suggestion:\n")))
		fmt.Println(color.YellowString(aiSuggestion))
	}
}

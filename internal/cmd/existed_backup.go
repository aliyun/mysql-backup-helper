package cmd

import (
	"backup-helper/internal/backup"
	"backup-helper/internal/config"
	"backup-helper/internal/log"
	"backup-helper/internal/mysql"
	"backup-helper/internal/rate"
	"backup-helper/internal/transfer"
	"backup-helper/internal/utils"
	"io"
	"os"
	"time"

	"github.com/gioco-play/easy-i18n/i18n"
)

// HandleExistedBackup handles uploading/streaming an existing backup file
func HandleExistedBackup(cfg *config.Config, effective *config.EffectiveValues, flags *config.Flags) error {
	// Create log context for existed backup
	logCtx, err := log.NewLogContext(cfg.LogDir, cfg.LogFileName)
	if err != nil {
		i18n.Printf("Failed to create log context: %v\n", err)
		os.Exit(1)
	}
	defer logCtx.Close()

	// upload existed backup file to OSS or stream via TCP
	logCtx.WriteLog("BACKUP", "Processing existing backup file")
	i18n.Printf("[backup-helper] Processing existing backup file...\n")

	// Validate backup file before processing
	var backupInfo *backup.BackupFileInfo
	var err2 error

	if effective.ExistedBackup == "-" {
		// Validate data from stdin
		backupInfo, err2 = backup.ValidateBackupFileFromStdin()
		if err2 != nil {
			i18n.Printf("Validation error: %v\n", err2)
			os.Exit(1)
		}
		backup.PrintBackupFileValidationFromStdin(backupInfo)
	} else {
		// Validate file
		backupInfo, err2 = backup.ValidateBackupFile(effective.ExistedBackup)
		if err2 != nil {
			i18n.Printf("Validation error: %v\n", err2)
			os.Exit(1)
		}
		backup.PrintBackupFileValidation(effective.ExistedBackup, backupInfo)
	}

	// Exit if backup file is invalid
	if !backupInfo.IsValid {
		i18n.Printf("[backup-helper] Cannot proceed with invalid backup file.\n")
		os.Exit(1)
	}

	// Display IO limit after validation
	if cfg.IOLimit == -1 {
		i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
	} else if cfg.IOLimit > 0 {
		i18n.Printf("[backup-helper] IO rate limit set to: %s/s\n", utils.FormatBytes(cfg.IOLimit))
	} else {
		i18n.Printf("[backup-helper] IO rate limit set to: %s/s (default)\n", utils.FormatBytes(cfg.GetRateLimit()))
	}

	// Get reader from existing backup file or stdin
	var reader io.Reader
	if effective.ExistedBackup == "-" {
		// Read from stdin (for cat command)
		reader = os.Stdin
		i18n.Printf("[backup-helper] Reading backup data from stdin...\n")
	} else {
		// Read from file
		file, err := os.Open(effective.ExistedBackup)
		if err != nil {
			i18n.Printf("Open backup file error: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		reader = file
		i18n.Printf("[backup-helper] Reading backup data from file: %s\n", effective.ExistedBackup)
	}

	// Determine object name suffix based on compression type
	ossObjectName := cfg.ObjectName
	objectSuffix := ".xb"
	effectiveCompressType := effective.CompressType
	cfg.CompressType = effectiveCompressType
	switch effectiveCompressType {
	case "zstd":
		objectSuffix = ".xb.zst"
	case "qp":
		objectSuffix = "_qp.xb"
	default:
		objectSuffix = ".xb"
	}
	timestamp := time.Now().Format("_20060102150405")
	fullObjectName := ossObjectName + timestamp + objectSuffix

	// Calculate total size for existing backup
	var totalSize int64
	if effective.EstimatedSize > 0 {
		totalSize = effective.EstimatedSize
		i18n.Printf("[backup-helper] Using estimated size: %s\n", utils.FormatBytes(totalSize))
	} else if effective.ExistedBackup != "-" {
		// Get file size for existing backup file
		totalSize, err = mysql.GetFileSize(effective.ExistedBackup)
		if err != nil {
			i18n.Printf("Warning: Could not get backup file size, progress tracking will be limited: %v\n", err)
			totalSize = 0
		} else {
			i18n.Printf("[backup-helper] Backup file size: %s\n", utils.FormatBytes(totalSize))
		}
	} else {
		// stdin - we can't get size
		i18n.Printf("[backup-helper] Uploading from stdin, size unknown\n")
	}

	switch flags.Mode {
	case "oss":
		i18n.Printf("[backup-helper] Uploading existing backup to OSS...\n")
		isCompressed := cfg.CompressType != ""
		err := transfer.UploadReaderToOSS(cfg, fullObjectName, reader, totalSize, isCompressed, logCtx)
		if err != nil {
			i18n.Printf("OSS upload error: %v\n", err)
			os.Exit(1)
		}
		i18n.Printf("[backup-helper] OSS upload completed!\n")
		logCtx.MarkSuccess()
	case "stream":
		// Parse stream-host from command line or config
		streamHost := effective.StreamHost
		if streamHost == "" && cfg.StreamHost != "" {
			streamHost = cfg.StreamHost
		}

		// Only use config value if command line didn't specify and config has non-zero value
		streamPort := effective.StreamPort
		if streamHost == "" {
			if streamPort == 0 && cfg.StreamPort > 0 {
				streamPort = cfg.StreamPort
			}
			// Show equivalent command (before starting server, so we show original port)
			equivalentSource := effective.ExistedBackup
			if effective.ExistedBackup == "-" {
				equivalentSource = "stdin"
			}
			if streamPort > 0 {
				i18n.Printf("[backup-helper] Starting TCP stream server on port %d...\n", streamPort)
				i18n.Printf("[backup-helper] Equivalent command: cat %s | nc -l4 %d\n",
					equivalentSource, streamPort)
			} else {
				i18n.Printf("[backup-helper] Starting TCP stream server (auto-find available port)...\n")
			}
		} else {
			// When using stream-host, port is required
			if streamPort == 0 {
				if cfg.StreamPort > 0 {
					streamPort = cfg.StreamPort
				} else {
					i18n.Printf("Error: --stream-port is required when using --stream-host\n")
					os.Exit(1)
				}
			}
		}

		// handshake priority: command line > config > default
		enableHandshake := effective.EnableHandshake
		streamKey := effective.StreamKey

		var writer io.WriteCloser
		var closer func()
		var err error

		if streamHost != "" {
			// Active connection: connect to remote server
			logCtx.WriteLog("TCP", "Active push mode: connecting to %s:%d", streamHost, streamPort)
			isCompressed := cfg.CompressType != ""
			writer, _, closer, _, err = transfer.StartStreamClient(streamHost, streamPort, enableHandshake, streamKey, totalSize, isCompressed, logCtx)
			if err != nil {
				i18n.Printf("Stream client error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Passive connection: listen locally and wait for connection
			tcpWriter, _, closerFunc, _, _, err := transfer.StartStreamSender(streamPort, enableHandshake, streamKey, totalSize, cfg.CompressType != "", cfg.Timeout, logCtx)
			if err != nil {
				i18n.Printf("Stream server error: %v\n", err)
				os.Exit(1)
			}
			writer = tcpWriter
			closer = closerFunc
		}
		defer closer()

		// Apply rate limiting for stream mode if configured
		var finalWriter io.WriteCloser = writer
		rateLimit := cfg.GetRateLimit()
		if rateLimit > 0 {
			rateLimitedWriter := rate.NewRateLimitedWriter(writer, rateLimit)
			finalWriter = rateLimitedWriter
		}

		// Stream the backup data
		i18n.Printf("[backup-helper] Streaming backup data...\n")

		_, err = io.Copy(finalWriter, reader)
		if err != nil {
			i18n.Printf("TCP stream error: %v\n", err)
			os.Exit(1)
		}

		i18n.Printf("[backup-helper] Stream completed!\n")
		logCtx.MarkSuccess()
	default:
		i18n.Printf("Unknown mode: %s\n", flags.Mode)
		os.Exit(1)
	}
	return nil
}

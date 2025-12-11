package cmd

import (
	"backup-helper/internal/ai"
	"backup-helper/internal/backup"
	"backup-helper/internal/check"
	"backup-helper/internal/config"
	"backup-helper/internal/log"
	"backup-helper/internal/mysql"
	"backup-helper/internal/rate"
	"backup-helper/internal/transfer"
	"backup-helper/internal/utils"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gioco-play/easy-i18n/i18n"
	"golang.org/x/term"
)

// HandleBackup handles the backup command
func HandleBackup(cfg *config.Config, effective *config.EffectiveValues, flags *config.Flags) error {
	// Pre-check for backup mode
	effectiveCompressType := effective.CompressType

	// MySQL param check (only needed for backup)
	password := effective.Password
	if password == "" {
		i18n.Printf("Please input mysql-server password: ")
		pwd, _ := term.ReadPassword(0)
		i18n.Printf("\n")
		password = string(pwd)
	}

	// Get MySQL connection for pre-check
	var db *sql.DB
	if effective.Host != "" && effective.User != "" && password != "" {
		db = mysql.GetConnection(effective.Host, effective.Port, effective.User, password)
		defer db.Close()
	}

	// Run pre-flight checks
	backupResults := check.CheckForBackupMode(cfg, effectiveCompressType, db, "", 0)
	hasCriticalError := false
	for _, result := range backupResults {
		if result.Status == "ERROR" {
			hasCriticalError = true
			i18n.Printf("[ERROR] %s: %s - %s\n", result.Item, result.Value, result.Message)
		}
	}
	if hasCriticalError {
		i18n.Printf("\n[ERROR] Pre-flight checks failed. Please fix the errors above before proceeding.\n")
		os.Exit(1)
	}

	i18n.Printf("connect to mysql-server host=%s port=%d user=%s\n", effective.Host, effective.Port, effective.User)
	utils.OutputHeader()

	// db may already be set from pre-check above
	if db == nil {
		db = mysql.GetConnection(effective.Host, effective.Port, effective.User, password)
		defer db.Close()
	}

	// Display IO limit after parameter check
	if cfg.IOLimit == -1 {
		i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
	} else if cfg.IOLimit > 0 {
		i18n.Printf("[backup-helper] IO rate limit set to: %s/s\n", utils.FormatBytes(cfg.IOLimit))
	} else {
		i18n.Printf("[backup-helper] IO rate limit set to: %s/s (default)\n", utils.FormatBytes(cfg.GetRateLimit()))
	}

	// Check compression dependencies early
	if effectiveCompressType != "" {
		if err := check.CheckCompressionDependencies(effectiveCompressType, true, cfg); err != nil {
			i18n.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Check xtrabackup version
	mysqlVer := cfg.MysqlVersion
	check.CheckXtraBackupVersion(mysqlVer, cfg)

	// Create log context
	logCtx, err := log.NewLogContext(cfg.LogDir, cfg.LogFileName)
	if err != nil {
		i18n.Printf("Failed to create log context: %v\n", err)
		os.Exit(1)
	}
	defer logCtx.Close()

	i18n.Printf("[backup-helper] Running xtrabackup...\n")
	cfg.MysqlHost = effective.Host
	cfg.MysqlPort = effective.Port
	cfg.MysqlUser = effective.User
	cfg.MysqlPassword = password
	logCtx.WriteLog("BACKUP", "Starting backup operation")
	logCtx.WriteLog("BACKUP", "MySQL host: %s, port: %d, user: %s", effective.Host, effective.Port, effective.User)

	// Determine objectName suffix and compression param
	ossObjectName := cfg.ObjectName
	objectSuffix := ".xb"
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

	reader, cmd, err := backup.RunXtraBackup(cfg, db, logCtx)
	if err != nil {
		logCtx.WriteLog("BACKUP", "Failed to start xtrabackup: %v", err)
		i18n.Printf("Run xtrabackup error: %v\n", err)
		os.Exit(1)
	}

	// Calculate total size for progress tracking
	var totalSize int64
	if effective.EstimatedSize > 0 {
		totalSize = effective.EstimatedSize
		i18n.Printf("[backup-helper] Using estimated size: %s\n", utils.FormatBytes(totalSize))
	} else {
		// Calculate datadir size
		datadir, err := mysql.GetDatadirFromMySQL(db)
		if err != nil {
			i18n.Printf("Warning: Could not get datadir, progress tracking will be limited: %v\n", err)
		} else {
			totalSize, err = mysql.CalculateBackupSize(datadir)
			if err != nil {
				i18n.Printf("Warning: Could not calculate backup size, progress tracking will be limited: %v\n", err)
				totalSize = 0
			} else {
				i18n.Printf("[backup-helper] Calculated datadir size: %s\n", utils.FormatBytes(totalSize))
			}
		}
	}

	switch flags.Mode {
	case "oss":
		err = handleOSSBackup(cfg, fullObjectName, reader, totalSize, logCtx, cmd)
	case "stream":
		err = handleStreamBackup(cfg, effective, flags, totalSize, reader, logCtx, cmd)
	default:
		i18n.Printf("Unknown mode: %s\n", flags.Mode)
		os.Exit(1)
	}

	if err != nil {
		return err
	}

	// Wait for backup to complete
	cmd.Wait()
	logCtx.WriteLog("BACKUP", "xtrabackup process completed")

	// Check backup log
	logContent, err := os.ReadFile(logCtx.GetFileName())
	if err != nil {
		logCtx.WriteLog("BACKUP", "Failed to read log file: %v", err)
		i18n.Printf("Backup log read error.\n")
		os.Exit(1)
	}

	if !strings.Contains(string(logContent), "completed OK!") {
		logCtx.WriteLog("BACKUP", "Backup failed: no 'completed OK!' found in log")
		errorSummary := log.ExtractErrorSummary("BACKUP", string(logContent))
		if errorSummary != "" {
			i18n.Printf("Backup failed. Error summary:\n%s\n", errorSummary)
		} else {
			i18n.Printf("Backup failed (no 'completed OK!').\n")
		}
		i18n.Printf("Log file: %s\n", logCtx.GetFileName())

		// Handle AI diagnosis
		if flags.AIDiagnoseFlag == "on" {
			if utils.PromptAIDiagnosis(flags.AutoYes) {
				if cfg.QwenAPIKey == "" {
					i18n.Printf("Qwen API Key is required for AI diagnosis. Please set it in config.\n")
					os.Exit(1)
				}
				aiSuggestion, err := ai.DiagnoseWithAliQwen(cfg, "BACKUP", string(logContent))
				if err != nil {
					i18n.Printf("AI diagnosis failed: %v\n", err)
				} else {
					fmt.Print(color.YellowString(i18n.Sprintf("AI diagnosis suggestion:\n")))
					fmt.Println(color.YellowString(aiSuggestion))
				}
			}
		}
		os.Exit(1)
	}

	fmt.Print("\n")
	logCtx.WriteLog("BACKUP", "Backup completed successfully")
	logCtx.MarkSuccess()
	i18n.Printf("[backup-helper] Backup and upload completed!\n")
	i18n.Printf("[backup-helper] Log file: %s\n", logCtx.GetFileName())
	return nil
}

func handleOSSBackup(cfg *config.Config, fullObjectName string, reader io.Reader, totalSize int64, logCtx *log.LogContext, cmd *exec.Cmd) error {
	i18n.Printf("[backup-helper] Uploading to OSS...\n")
	logCtx.WriteLog("OSS", "Starting OSS upload")
	isCompressed := cfg.CompressType != ""
	err := transfer.UploadReaderToOSS(cfg, fullObjectName, reader, totalSize, isCompressed, logCtx)
	if err != nil {
		logCtx.WriteLog("OSS", "OSS upload failed: %v", err)
		i18n.Printf("OSS upload error: %v\n", err)
		if cmd != nil {
			cmd.Process.Kill()
		}
		os.Exit(1)
	}
	logCtx.WriteLog("OSS", "OSS upload completed successfully")
	logCtx.MarkSuccess()
	return nil
}

func handleStreamBackup(cfg *config.Config, effective *config.EffectiveValues, flags *config.Flags, totalSize int64, reader io.Reader, logCtx *log.LogContext, cmd *exec.Cmd) error {
	streamHost := effective.StreamHost
	if streamHost == "" && cfg.StreamHost != "" {
		streamHost = cfg.StreamHost
	}

	remoteOutput := effective.RemoteOutput
	if remoteOutput == "" && cfg.RemoteOutput != "" {
		remoteOutput = cfg.RemoteOutput
	}

	// Validate SSH mode requirements
	if flags.UseSSH && streamHost == "" {
		i18n.Printf("Error: --ssh requires --stream-host\n")
		if cmd != nil {
			cmd.Process.Kill()
		}
		os.Exit(1)
	}

	// handshake priority: command line > config > default
	enableHandshake := effective.EnableHandshake
	streamKey := effective.StreamKey

	var writer io.WriteCloser
	var closer func()
	var err error

	streamPort := effective.StreamPort
	if streamHost != "" {
		if flags.UseSSH {
			// SSH mode: Start receiver on remote via SSH
			logCtx.WriteLog("SSH", "Starting remote receiver via SSH")
			logCtx.WriteLog("SSH", "Remote host: %s", streamHost)
			i18n.Printf("[backup-helper] Starting remote receiver via SSH on %s...\n", streamHost)

			sshPort := streamPort
			if streamPort == 0 && cfg.StreamPort > 0 {
				sshPort = cfg.StreamPort
			}

			remotePort, outputPath, _, sshCleanup, err := transfer.StartRemoteReceiverViaSSH(
				streamHost, sshPort, remoteOutput, totalSize, enableHandshake, streamKey)
			if err != nil {
				i18n.Printf("SSH receiver error: %v\n", err)
				if cmd != nil {
					cmd.Process.Kill()
				}
				os.Exit(1)
			}

			streamPort = remotePort
			if sshPort > 0 {
				i18n.Printf("[backup-helper] Remote receiver started on port %d via SSH\n", streamPort)
			} else {
				i18n.Printf("[backup-helper] Remote receiver started on auto-discovered port %d via SSH\n", streamPort)
			}

			if outputPath != "" {
				i18n.Printf("[backup-helper] Remote backup will be saved to: %s\n", outputPath)
			} else if remoteOutput != "" {
				i18n.Printf("[backup-helper] Remote backup will be saved to: %s\n", remoteOutput)
			} else {
				i18n.Printf("[backup-helper] Remote backup will be saved to: auto-generated path (backup_YYYYMMDDHHMMSS.xb)\n")
			}

			// Connect to remote receiver
			isCompressed := cfg.CompressType != ""
			writer, _, closer, _, err = transfer.StartStreamClient(
				streamHost, streamPort, enableHandshake, streamKey, totalSize, isCompressed, logCtx)
			if err != nil {
				sshCleanup()
				i18n.Printf("Stream client error: %v\n", err)
				if cmd != nil {
					cmd.Process.Kill()
				}
				os.Exit(1)
			}

			// Wrap closer to cleanup SSH process
			originalCloser := closer
			closer = func() {
				if originalCloser != nil {
					originalCloser()
				}
				sshCleanup()
			}
		} else {
			// Normal mode: Direct connection to specified port
			if streamPort == 0 {
				if cfg.StreamPort > 0 {
					streamPort = cfg.StreamPort
				} else {
					i18n.Printf("Error: --stream-port is required when using --stream-host\n")
					if cmd != nil {
						cmd.Process.Kill()
					}
					os.Exit(1)
				}
			}

			isCompressed := cfg.CompressType != ""
			writer, _, closer, _, err = transfer.StartStreamClient(
				streamHost, streamPort, enableHandshake, streamKey, totalSize, isCompressed, logCtx)
			if err != nil {
				i18n.Printf("Stream client error: %v\n", err)
				if cmd != nil {
					cmd.Process.Kill()
				}
				os.Exit(1)
			}
		}
	} else {
		// Passive connection: listen locally and wait for connection
		if streamPort == 0 && cfg.StreamPort > 0 {
			streamPort = cfg.StreamPort
		}

		tcpWriter, _, closerFunc, _, _, err := transfer.StartStreamSender(streamPort, enableHandshake, streamKey, totalSize, cfg.CompressType != "", cfg.Timeout, logCtx)
		if err != nil {
			i18n.Printf("Stream server error: %v\n", err)
			if cmd != nil {
				cmd.Process.Kill()
			}
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

	_, err = io.Copy(finalWriter, reader)
	if err != nil {
		i18n.Printf("TCP stream error: %v\n", err)
		if cmd != nil {
			cmd.Process.Kill()
		}
		os.Exit(1)
	}
	return nil
}

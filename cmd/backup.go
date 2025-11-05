package cmd

import (
	"backup-helper/internal/service"

	"github.com/gioco-play/easy-i18n/i18n"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	// MySQL connection flags
	backupHost     string
	backupPort     int
	backupUser     string
	backupPassword string

	// Destination flags
	backupToOSS    bool
	backupToStream int

	// Compression flags
	backupCompressType string

	// Performance flags
	backupEstimatedSize string
	backupIOLimit       string

	// Stream flags
	backupEnableHandshake bool
	backupStreamKey       string

	// Diagnostic flags
	backupAIDiagnose string
)

// backupCmd represents the backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Execute MySQL backup and transfer to destination",
	Long: `Connect to MySQL database, execute xtrabackup, and transfer backup to OSS or TCP stream.

Examples:
  # Backup to OSS
  mysql-backup-helper backup --host 127.0.0.1 --user root --to-oss

  # Backup and stream via TCP on port 9000
  mysql-backup-helper backup --host 127.0.0.1 --user root --to-stream 9000

  # Backup with compression and bandwidth limit
  mysql-backup-helper backup --host 127.0.0.1 --user root \
    --compress-type zstd --io-limit 50MB/s --to-oss`,
	RunE: runBackup,
}

func init() {
	rootCmd.AddCommand(backupCmd)

	// MySQL connection flags
	backupCmd.Flags().StringVar(&backupHost, "host", "", "MySQL server host")
	backupCmd.Flags().IntVar(&backupPort, "port", 0, "MySQL server port (default: 3306)")
	backupCmd.Flags().StringVar(&backupUser, "user", "", "MySQL username")
	backupCmd.Flags().StringVar(&backupPassword, "password", "", "MySQL password (prompt if empty)")

	// Destination flags
	backupCmd.Flags().BoolVar(&backupToOSS, "to-oss", false, "Upload to Alibaba Cloud OSS")
	backupCmd.Flags().IntVar(&backupToStream, "to-stream", -1, "Stream via TCP (0=auto-find port)")

	// Compression flags
	backupCmd.Flags().StringVar(&backupCompressType, "compress-type", "", "Compression: qp, zstd, or none")

	// Performance flags
	backupCmd.Flags().StringVar(&backupEstimatedSize, "estimated-size", "", "Estimated backup size (e.g., '10GB')")
	backupCmd.Flags().StringVar(&backupIOLimit, "io-limit", "", "IO bandwidth limit (e.g., '100MB/s', -1=unlimited)")

	// Stream flags
	backupCmd.Flags().BoolVar(&backupEnableHandshake, "enable-handshake", false, "Enable handshake authentication")
	backupCmd.Flags().StringVar(&backupStreamKey, "stream-key", "", "Handshake key for authentication")

	// Diagnostic flags
	backupCmd.Flags().StringVar(&backupAIDiagnose, "ai-diagnose", "", "AI diagnosis: on, off, or auto (default: auto)")
}

func runBackup(cmd *cobra.Command, args []string) error {
	cfg := GetConfig()

	// Merge command line flags with config
	if backupHost == "" {
		backupHost = cfg.MysqlHost
	}
	if backupPort == 0 {
		backupPort = cfg.MysqlPort
	}
	if backupUser == "" {
		backupUser = cfg.MysqlUser
	}
	if backupPassword == "" {
		backupPassword = cfg.MysqlPassword
	}
	if backupCompressType == "" && cfg.CompressType != "" {
		backupCompressType = cfg.CompressType
	}

	// Determine mode based on flags
	mode, streamPort := determineMode(backupToOSS, backupToStream)
	if mode == "stream" && streamPort == 0 && cfg.StreamPort > 0 {
		streamPort = cfg.StreamPort
	}

	// Parse estimated size using common function
	estimatedSize, err := parseEstimatedSize(backupEstimatedSize, cfg.EstimatedSize)
	if err != nil {
		return err
	}

	// Parse IO limit using common function
	ioLimit, err := parseIOLimit(backupIOLimit, cfg.IOLimit)
	if err != nil {
		return err
	}

	// Apply IO limit to config
	applyIOLimit(cfg, ioLimit)

	// Prompt for password if not provided
	if backupPassword == "" {
		i18n.Printf("Please input mysql-server password: ")
		pwd, _ := term.ReadPassword(0)
		i18n.Printf("\n")
		backupPassword = string(pwd)
	}

	i18n.Printf("connect to mysql-server host=%s port=%d user=%s\n", backupHost, backupPort, backupUser)
	outputHeader()

	// Update config with command line values
	cfg.MysqlHost = backupHost
	cfg.MysqlPort = backupPort
	cfg.MysqlUser = backupUser
	cfg.MysqlPassword = backupPassword

	// Set compression type
	if backupCompressType != "" {
		if backupCompressType == "none" {
			cfg.Compress = false
			cfg.CompressType = ""
		} else {
			cfg.Compress = true
			cfg.CompressType = backupCompressType
		}
	}

	// Parse handshake settings using common function
	enableHandshake, streamKey := parseHandshakeSettings(cmd, "enable-handshake", backupEnableHandshake, backupStreamKey, cfg)

	// Create backup service and execute
	backupService := service.NewBackupService(cfg)
	opts := &service.BackupOptions{
		Mode:            mode,
		StreamPort:      streamPort,
		EstimatedSize:   estimatedSize,
		EnableHandshake: enableHandshake,
		StreamKey:       streamKey,
		AIDiagnose:      backupAIDiagnose,
	}

	return backupService.Execute(opts)
}

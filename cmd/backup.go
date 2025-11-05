package cmd

import (
	"backup-helper/internal/service"

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
	backupMode       string
	backupStreamPort int

	// OSS flags (non-sensitive)
	backupEndpoint   string
	backupBucketName string
	backupObjectName string

	// Compression flags
	backupCompressType string

	// Performance flags
	backupTraffic string

	// Authentication flags
	backupEnableAuth bool
	backupAuthKey    string
)

// backupCmd represents the backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Execute MySQL backup and transfer to destination",
	Long: `Connect to MySQL database, execute xtrabackup, and transfer backup to OSS or TCP stream.

Examples:
  # Backup to OSS (OSS credentials from config file)
  mysql-backup-helper backup --config config.json --host 127.0.0.1 --user root --mode oss

  # Backup to specific OSS bucket (override config)
  mysql-backup-helper backup --config config.json --host 127.0.0.1 --user root \
    --mode oss --bucket-name my-backup-bucket --object-name backup/mysql

  # Backup and stream via TCP on port 9000
  mysql-backup-helper backup --host 127.0.0.1 --user root --mode stream --stream-port 9000

  # Backup with compression and bandwidth limit
  mysql-backup-helper backup --config config.json --host 127.0.0.1 --user root \
    --compress-type zstd --traffic 50MB/s --mode oss`,
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
	backupCmd.Flags().StringVar(&backupMode, "mode", "", "Backup mode: oss or stream (default: oss)")
	backupCmd.Flags().IntVar(&backupStreamPort, "stream-port", 0, "Stream port for TCP (0=auto-find port, only used when mode=stream)")

	// OSS flags (non-sensitive)
	backupCmd.Flags().StringVar(&backupEndpoint, "endpoint", "", "OSS endpoint URL")
	backupCmd.Flags().StringVar(&backupBucketName, "bucket-name", "", "OSS bucket name")
	backupCmd.Flags().StringVar(&backupObjectName, "object-name", "", "OSS object name prefix")

	// Compression flags
	backupCmd.Flags().StringVar(&backupCompressType, "compress-type", "", "Compression: qp, zstd, or none")

	// Performance flags
	backupCmd.Flags().StringVar(&backupTraffic, "traffic", "", "Traffic bandwidth limit (e.g., '100MB/s', -1=unlimited)")

	// Authentication flags
	backupCmd.Flags().BoolVar(&backupEnableAuth, "enable-auth", false, "Enable stream authentication")
	backupCmd.Flags().StringVar(&backupAuthKey, "auth-key", "", "Authentication key for stream transfer")
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

	// Determine mode from flag or default to oss
	mode := backupMode
	if mode == "" {
		mode = "oss"
	}

	// Get stream port for stream mode
	streamPort := backupStreamPort
	if mode == "stream" && streamPort == 0 && cfg.StreamPort > 0 {
		streamPort = cfg.StreamPort
	}

	// Parse traffic limit using common function
	traffic, err := parseTraffic(backupTraffic, cfg.Traffic)
	if err != nil {
		return err
	}

	// Apply traffic limit to config
	applyTraffic(cfg, traffic)

	// Prompt for password if not provided
	if backupPassword == "" {
		logError("Please input mysql-server password: ")
		pwd, _ := term.ReadPassword(0)
		logError("\n")
		backupPassword = string(pwd)
	}

	logVerbose("connect to mysql-server host=%s port=%d user=%s\n", backupHost, backupPort, backupUser)
	if !IsQuiet() {
		outputHeader()
	}

	// Update config with command line values
	cfg.MysqlHost = backupHost
	cfg.MysqlPort = backupPort
	cfg.MysqlUser = backupUser
	cfg.MysqlPassword = backupPassword

	// Update OSS config if provided (non-sensitive info only)
	if backupEndpoint != "" {
		cfg.Endpoint = backupEndpoint
	}
	if backupBucketName != "" {
		cfg.BucketName = backupBucketName
	}
	if backupObjectName != "" {
		cfg.ObjectName = backupObjectName
	}

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

	// Parse authentication settings using common function
	enableAuth, authKey := parseAuthSettings(cmd, "enable-auth", backupEnableAuth, backupAuthKey, cfg)

	// Create backup service and execute
	backupService := service.NewBackupService(cfg)
	opts := &service.BackupOptions{
		Mode:       mode,
		StreamPort: streamPort,
		EnableAuth: enableAuth,
		AuthKey:    authKey,
	}

	return backupService.Execute(opts)
}

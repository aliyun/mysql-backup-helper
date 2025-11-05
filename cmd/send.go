package cmd

import (
	"backup-helper/internal/service"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Source flags
	sendFile  string
	sendStdin bool

	// Destination flags
	sendMode       string
	sendStreamPort int

	// OSS flags (non-sensitive)
	sendEndpoint   string
	sendBucketName string
	sendObjectName string

	// Validation flags
	sendSkipValidation bool
	sendValidateOnly   bool

	// Performance flags
	sendTraffic string

	// Authentication flags
	sendEnableAuth bool
	sendAuthKey    string
)

// sendCmd represents the send command
var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send existing backup file to destination",
	Long: `Upload existing backup file to OSS or stream via TCP.

Examples:
  # Upload existing backup to OSS (OSS credentials from config file)
  mysql-backup-helper send --config config.json --file /path/to/backup.xb --mode oss

  # Upload to specific OSS bucket (override config)
  mysql-backup-helper send --config config.json --file /path/to/backup.xb \
    --mode oss --bucket-name my-backup-bucket --object-name backup/mysql

  # Stream backup file via TCP
  mysql-backup-helper send --file /path/to/backup.xb --mode stream --stream-port 9000

  # Send from stdin (pipe from another command)
  cat backup.xb | mysql-backup-helper send --config config.json --stdin --mode oss

  # Only validate backup file
  mysql-backup-helper send --file backup.xb --validate-only`,
	RunE: runSend,
}

func init() {
	rootCmd.AddCommand(sendCmd)

	// Source flags
	sendCmd.Flags().StringVar(&sendFile, "file", "", "Path to backup file (or '-' for stdin)")
	sendCmd.Flags().BoolVar(&sendStdin, "stdin", false, "Read backup from stdin")

	// Destination flags
	sendCmd.Flags().StringVar(&sendMode, "mode", "", "Transfer mode: oss or stream (default: oss)")
	sendCmd.Flags().IntVar(&sendStreamPort, "stream-port", 0, "Stream port for TCP (0=auto-find port, only used when mode=stream)")

	// OSS flags (non-sensitive)
	sendCmd.Flags().StringVar(&sendEndpoint, "endpoint", "", "OSS endpoint URL")
	sendCmd.Flags().StringVar(&sendBucketName, "bucket-name", "", "OSS bucket name")
	sendCmd.Flags().StringVar(&sendObjectName, "object-name", "", "OSS object name prefix")

	// Validation flags
	sendCmd.Flags().BoolVar(&sendSkipValidation, "skip-validation", false, "Skip backup file validation")
	sendCmd.Flags().BoolVar(&sendValidateOnly, "validate-only", false, "Only validate, don't transfer")

	// Performance flags
	sendCmd.Flags().StringVar(&sendTraffic, "traffic", "", "Traffic bandwidth limit (e.g., '100MB/s')")

	// Authentication flags
	sendCmd.Flags().BoolVar(&sendEnableAuth, "enable-auth", false, "Enable stream authentication")
	sendCmd.Flags().StringVar(&sendAuthKey, "auth-key", "", "Authentication key for stream transfer")
}

func runSend(cmd *cobra.Command, args []string) error {
	cfg := GetConfig()

	// Determine source
	existedBackup := sendFile
	if sendStdin || existedBackup == "-" {
		existedBackup = "-"
	}
	if existedBackup == "" && cfg.ExistedBackup != "" {
		existedBackup = cfg.ExistedBackup
	}
	if existedBackup == "" {
		return fmt.Errorf("source file required: use --file PATH or --stdin")
	}

	// Determine mode from flag or default to oss
	mode := sendMode
	if mode == "" {
		mode = "oss"
	}

	// Get stream port for stream mode
	streamPort := sendStreamPort
	if mode == "stream" && streamPort == 0 && cfg.StreamPort > 0 {
		streamPort = cfg.StreamPort
	}

	// Parse traffic limit using common function
	traffic, err := parseTraffic(sendTraffic, cfg.Traffic)
	if err != nil {
		return err
	}

	// Apply traffic limit to config
	applyTraffic(cfg, traffic)

	// Update OSS config if provided (non-sensitive info only)
	if sendEndpoint != "" {
		cfg.Endpoint = sendEndpoint
	}
	if sendBucketName != "" {
		cfg.BucketName = sendBucketName
	}
	if sendObjectName != "" {
		cfg.ObjectName = sendObjectName
	}

	// Parse authentication settings using common function
	enableAuth, authKey := parseAuthSettings(cmd, "enable-auth", sendEnableAuth, sendAuthKey, cfg)

	// Create transfer service and execute
	transferService := service.NewTransferService(cfg)
	opts := &service.SendOptions{
		SourceFile:     existedBackup,
		Mode:           mode,
		StreamPort:     streamPort,
		SkipValidation: sendSkipValidation,
		ValidateOnly:   sendValidateOnly,
		EnableAuth:     enableAuth,
		AuthKey:        authKey,
	}

	return transferService.Send(opts)
}

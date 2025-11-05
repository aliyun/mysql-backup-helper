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

	// Validation flags
	sendSkipValidation bool
	sendValidateOnly   bool

	// Performance flags
	sendIOLimit string

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
  # Upload existing backup to OSS
  mysql-backup-helper send --file /path/to/backup.xb --mode oss

  # Stream backup file via TCP
  mysql-backup-helper send --file /path/to/backup.xb --mode stream --stream-port 9000

  # Send from stdin (pipe from another command)
  cat backup.xb | mysql-backup-helper send --stdin --mode oss

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

	// Validation flags
	sendCmd.Flags().BoolVar(&sendSkipValidation, "skip-validation", false, "Skip backup file validation")
	sendCmd.Flags().BoolVar(&sendValidateOnly, "validate-only", false, "Only validate, don't transfer")

	// Performance flags
	sendCmd.Flags().StringVar(&sendIOLimit, "io-limit", "", "IO bandwidth limit (e.g., '100MB/s')")

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

	// Parse IO limit using common function
	ioLimit, err := parseIOLimit(sendIOLimit, cfg.IOLimit)
	if err != nil {
		return err
	}

	// Apply IO limit to config
	applyIOLimit(cfg, ioLimit)

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

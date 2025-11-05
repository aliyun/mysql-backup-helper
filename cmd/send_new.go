package cmd

import (
	"backup-helper/internal/pkg/format"
	"backup-helper/internal/service"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Source flags
	sendFile  string
	sendStdin bool

	// Destination flags
	sendToOSS    bool
	sendToStream int

	// Validation flags
	sendSkipValidation bool
	sendValidateOnly   bool

	// Performance flags
	sendEstimatedSize string
	sendIOLimit       string

	// Stream flags
	sendEnableHandshake bool
	sendStreamKey       string
)

// sendCmd represents the send command
var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send existing backup file to destination",
	Long: `Upload existing backup file to OSS or stream via TCP.

Examples:
  # Upload existing backup to OSS
  mysql-backup-helper send --file /path/to/backup.xb --to-oss

  # Stream backup file via TCP
  mysql-backup-helper send --file /path/to/backup.xb --to-stream 9000

  # Send from stdin (pipe from another command)
  cat backup.xb | mysql-backup-helper send --stdin --to-oss

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
	sendCmd.Flags().BoolVar(&sendToOSS, "to-oss", false, "Upload to Alibaba Cloud OSS")
	sendCmd.Flags().IntVar(&sendToStream, "to-stream", -1, "Stream via TCP (0=auto-find port)")

	// Validation flags
	sendCmd.Flags().BoolVar(&sendSkipValidation, "skip-validation", false, "Skip backup file validation")
	sendCmd.Flags().BoolVar(&sendValidateOnly, "validate-only", false, "Only validate, don't transfer")

	// Performance flags
	sendCmd.Flags().StringVar(&sendEstimatedSize, "estimated-size", "", "Estimated size for progress")
	sendCmd.Flags().StringVar(&sendIOLimit, "io-limit", "", "IO bandwidth limit (e.g., '100MB/s')")

	// Stream flags
	sendCmd.Flags().BoolVar(&sendEnableHandshake, "enable-handshake", false, "Enable handshake authentication")
	sendCmd.Flags().StringVar(&sendStreamKey, "stream-key", "", "Handshake key for authentication")
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

	// Determine mode
	mode := "oss"
	streamPort := 0
	if sendToStream >= 0 {
		mode = "stream"
		streamPort = sendToStream
		if streamPort == 0 && cfg.StreamPort > 0 {
			streamPort = cfg.StreamPort
		}
	} else if !sendToOSS {
		sendToOSS = true
	}

	// Parse estimated size
	var estimatedSize int64
	if sendEstimatedSize != "" {
		parsedSize, err := format.ParseSize(sendEstimatedSize)
		if err != nil {
			return fmt.Errorf("error parsing --estimated-size '%s': %v", sendEstimatedSize, err)
		}
		estimatedSize = parsedSize
	} else if cfg.EstimatedSize > 0 {
		estimatedSize = cfg.EstimatedSize
	}

	// Parse IO limit
	var ioLimit int64
	if sendIOLimit != "" {
		parsedLimit, err := format.ParseRateLimit(sendIOLimit)
		if err != nil {
			return fmt.Errorf("error parsing --io-limit '%s': %v", sendIOLimit, err)
		}
		ioLimit = parsedLimit
	} else if cfg.IOLimit > 0 {
		ioLimit = cfg.IOLimit
	}

	// Update traffic config
	if ioLimit == -1 {
		cfg.Traffic = 0
	} else if ioLimit > 0 {
		cfg.Traffic = ioLimit
	}

	// Parse handshake settings
	enableHandshake := sendEnableHandshake
	if !cmd.Flags().Changed("enable-handshake") {
		enableHandshake = cfg.EnableHandshake
	}
	streamKey := sendStreamKey
	if streamKey == "" {
		streamKey = cfg.StreamKey
	}

	// Create transfer service and execute
	transferService := service.NewTransferService(cfg)
	opts := &service.SendOptions{
		SourceFile:      existedBackup,
		Mode:            mode,
		StreamPort:      streamPort,
		EstimatedSize:   estimatedSize,
		SkipValidation:  sendSkipValidation,
		ValidateOnly:    sendValidateOnly,
		EnableHandshake: enableHandshake,
		StreamKey:       streamKey,
	}

	return transferService.Send(opts)
}

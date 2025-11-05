package cmd

import (
	"backup-helper/internal/service"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	// Source flags
	receiveFromStream int

	// Output flags
	receiveOutput string
	receiveStdout bool

	// Performance flags
	receiveEstimatedSize string
	receiveIOLimit       string

	// Stream flags
	receiveEnableHandshake bool
	receiveStreamKey       string
)

// receiveCmd represents the receive command
var receiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Receive backup from TCP stream",
	Long: `Listen on TCP port and receive backup data.

Examples:
  # Receive backup on port 9000 and save to file
  mysql-backup-helper receive --from-stream 9000 --output backup.xb

  # Receive on auto-selected port
  mysql-backup-helper receive --from-stream 0

  # Receive and pipe to another command
  mysql-backup-helper receive --from-stream 9000 --stdout | xbstream -x

  # Receive with handshake authentication
  mysql-backup-helper receive --from-stream 9000 \
    --enable-handshake --stream-key "secret-key"`,
	RunE: runReceive,
}

func init() {
	rootCmd.AddCommand(receiveCmd)

	// Source flags
	receiveCmd.Flags().IntVar(&receiveFromStream, "from-stream", 0, "Listen on TCP port (0=auto-find)")

	// Output flags
	receiveCmd.Flags().StringVar(&receiveOutput, "output", "", "Save to file (default: backup_YYYYMMDDHHMMSS.xb)")
	receiveCmd.Flags().BoolVar(&receiveStdout, "stdout", false, "Write to stdout")

	// Performance flags
	receiveCmd.Flags().StringVar(&receiveEstimatedSize, "estimated-size", "", "Estimated size for progress")
	receiveCmd.Flags().StringVar(&receiveIOLimit, "io-limit", "", "IO bandwidth limit")

	// Stream flags
	receiveCmd.Flags().BoolVar(&receiveEnableHandshake, "enable-handshake", false, "Enable handshake")
	receiveCmd.Flags().StringVar(&receiveStreamKey, "stream-key", "", "Handshake key")
}

func runReceive(cmd *cobra.Command, args []string) error {
	cfg := GetConfig()

	// Parse stream port
	streamPort := receiveFromStream
	if streamPort == 0 && !cmd.Flags().Changed("from-stream") && cfg.StreamPort > 0 {
		streamPort = cfg.StreamPort
	}

	// Parse handshake settings using common function
	enableHandshake, streamKey := parseHandshakeSettings(cmd, "enable-handshake", receiveEnableHandshake, receiveStreamKey, cfg)

	// Determine output path
	outputPath := receiveOutput
	if receiveStdout || outputPath == "-" {
		outputPath = "-"
	}
	if outputPath == "" && cfg.DownloadOutput != "" {
		outputPath = cfg.DownloadOutput
	}
	if outputPath == "" {
		timestamp := time.Now().Format("20060102150405")
		outputPath = fmt.Sprintf("backup_%s.xb", timestamp)
	}

	// Parse estimated size using common function
	estimatedSize, err := parseEstimatedSize(receiveEstimatedSize, cfg.EstimatedSize)
	if err != nil {
		return err
	}

	// Parse IO limit using common function
	ioLimit, err := parseIOLimit(receiveIOLimit, cfg.IOLimit)
	if err != nil {
		return err
	}

	// Apply IO limit to config
	applyIOLimit(cfg, ioLimit)

	// Display header
	if outputPath != "-" {
		outputHeader()
	} else {
		outputHeaderToStderr()
	}

	// Create transfer service and execute
	transferService := service.NewTransferService(cfg)
	opts := &service.ReceiveOptions{
		OutputPath:      outputPath,
		StreamPort:      streamPort,
		EstimatedSize:   estimatedSize,
		EnableHandshake: enableHandshake,
		StreamKey:       streamKey,
	}

	return transferService.Receive(opts)
}

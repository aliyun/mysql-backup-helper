package cmd

import (
	"backup-helper/internal/ai"
	"backup-helper/internal/check"
	"backup-helper/internal/config"
	"backup-helper/internal/extract"
	"backup-helper/internal/log"
	"backup-helper/internal/progress"
	"backup-helper/internal/rate"
	"backup-helper/internal/transfer"
	"backup-helper/internal/utils"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/gioco-play/easy-i18n/i18n"
)

// HandleDownload handles the download command
func HandleDownload(cfg *config.Config, effective *config.EffectiveValues, flags *config.Flags) error {
	// Pre-check for download mode
	downloadCompressType := effective.CompressType

	downloadResults := check.CheckForDownloadMode(cfg, downloadCompressType, flags.TargetDir, "", 0)
	hasCriticalError := false
	for _, result := range downloadResults {
		if result.Status == "ERROR" {
			hasCriticalError = true
			i18n.Printf("[ERROR] %s: %s - %s\n", result.Item, result.Value, result.Message)
		}
	}
	if hasCriticalError {
		i18n.Printf("\n[ERROR] Pre-flight checks failed. Please fix the errors above before proceeding.\n")
		os.Exit(1)
	}

	// Create log context
	logCtx, err := log.NewLogContext(cfg.LogDir, cfg.LogFileName)
	if err != nil {
		i18n.Printf("Failed to create log context: %v\n", err)
		os.Exit(1)
	}
	defer logCtx.Close()

	// Display header (only if not outputting to stdout)
	outputPath := flags.DownloadOutput
	if outputPath == "" && cfg.DownloadOutput != "" {
		outputPath = cfg.DownloadOutput
	}
	if outputPath == "" && flags.TargetDir == "" {
		// Default: backup_YYYYMMDDHHMMSS.xb (only if not extracting)
		timestamp := time.Now().Format("20060102150405")
		outputPath = fmt.Sprintf("backup_%s.xb", timestamp)
	}

	if outputPath != "-" {
		utils.OutputHeader()
	} else {
		// When outputting to stdout, output header to stderr
		utils.OutputHeaderToStderr()
	}
	logCtx.WriteLog("DOWNLOAD", "Starting download mode")

	// Parse stream-host from command line or config
	streamHost := effective.StreamHost
	if streamHost == "" && cfg.StreamHost != "" {
		streamHost = cfg.StreamHost
	}

	// Parse stream-port from command line or config
	streamPort := effective.StreamPort
	if streamPort == 0 && cfg.StreamPort > 0 {
		streamPort = cfg.StreamPort
	}

	// Parse handshake settings
	enableHandshake := effective.EnableHandshake
	streamKey := effective.StreamKey

	// Display IO limit
	if outputPath == "-" {
		// Output to stderr when streaming to stdout
		if cfg.IOLimit == -1 {
			i18n.Fprintf(os.Stderr, "[backup-helper] Rate limiting disabled (unlimited speed)\n")
		} else if cfg.IOLimit > 0 {
			i18n.Fprintf(os.Stderr, "[backup-helper] IO rate limit set to: %s/s\n", utils.FormatBytes(cfg.IOLimit))
		} else {
			i18n.Fprintf(os.Stderr, "[backup-helper] IO rate limit set to: %s/s (default)\n", utils.FormatBytes(cfg.GetRateLimit()))
		}
	} else {
		// Output to stdout when saving to file
		if cfg.IOLimit == -1 {
			i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
		} else if cfg.IOLimit > 0 {
			i18n.Printf("[backup-helper] IO rate limit set to: %s/s\n", utils.FormatBytes(cfg.IOLimit))
		} else {
			i18n.Printf("[backup-helper] IO rate limit set to: %s/s (default)\n", utils.FormatBytes(cfg.GetRateLimit()))
		}
	}

	// Start TCP receiver or client based on stream-host
	isCompressed := downloadCompressType != ""
	var receiver io.ReadCloser
	var tracker *progress.ProgressTracker
	var closer func()

	if streamHost != "" && streamPort > 0 {
		// Active mode: connect to remote server to pull data
		logCtx.WriteLog("DOWNLOAD", "Connecting to remote server %s:%d to pull data", streamHost, streamPort)
		if outputPath == "-" {
			i18n.Fprintf(os.Stderr, "[backup-helper] Connecting to %s:%d...\n", streamHost, streamPort)
		} else {
			i18n.Printf("[backup-helper] Connecting to %s:%d...\n", streamHost, streamPort)
		}
		receiver, tracker, closer, _, err = transfer.StartStreamClientReader(streamHost, streamPort, enableHandshake, streamKey, effective.EstimatedSize, isCompressed, logCtx)
		if err != nil {
			logCtx.WriteLog("DOWNLOAD", "Stream client error: %v", err)
			if outputPath == "-" {
				i18n.Fprintf(os.Stderr, "Stream client error: %v\n", err)
			} else {
				i18n.Printf("Stream client error: %v\n", err)
			}
			os.Exit(1)
		}
	} else {
		// Passive mode: listen locally and wait for connection
		logCtx.WriteLog("DOWNLOAD", "Starting TCP receiver on port %d", streamPort)
		var actualPort int
		var localIP string
		receiver, tracker, closer, actualPort, localIP, err = transfer.StartStreamReceiver(streamPort, enableHandshake, streamKey, effective.EstimatedSize, isCompressed, cfg.Timeout, logCtx)
		_ = actualPort // Port info already displayed in StartStreamReceiver
		_ = localIP    // IP info already displayed in StartStreamReceiver
		if err != nil {
			logCtx.WriteLog("DOWNLOAD", "Stream receiver error: %v", err)
			if outputPath == "-" {
				i18n.Fprintf(os.Stderr, "Stream receiver error: %v\n", err)
			} else {
				i18n.Printf("Stream receiver error: %v\n", err)
			}
			os.Exit(1)
		}
	}
	defer closer() // This will call tracker.Complete() internally

	// Apply rate limiting if configured
	var reader io.Reader = receiver
	rateLimit := cfg.GetRateLimit()
	if rateLimit > 0 {
		rateLimitedReader := rate.NewRateLimitedReader(receiver, rateLimit)
		reader = rateLimitedReader
	}

	// Determine output destination and handle extraction
	if flags.TargetDir != "" {
		// Extraction mode: decompress (if needed) and extract
		if outputPath == "-" {
			i18n.Printf("Error: --target-dir cannot be used with --output -\n")
			os.Exit(1)
		}

		// Check if target directory exists and is not empty
		if info, err := os.Stat(flags.TargetDir); err == nil {
			if info.IsDir() {
				empty, err := utils.IsDirEmpty(flags.TargetDir)
				if err != nil {
					logCtx.WriteLog("DOWNLOAD", "Failed to check target directory: %v", err)
					i18n.Printf("Error: Failed to check target directory: %v\n", err)
					os.Exit(1)
				}
				if !empty {
					// Directory exists and is not empty, ask user for confirmation
					if !utils.PromptOverwrite(flags.TargetDir, flags.AutoYes) {
						logCtx.WriteLog("DOWNLOAD", "User cancelled extraction to non-empty directory: %s", flags.TargetDir)
						i18n.Printf("Extraction cancelled.\n")
						os.Exit(0)
					}
					logCtx.WriteLog("DOWNLOAD", "User confirmed overwrite for directory: %s", flags.TargetDir)
					i18n.Printf("Clearing target directory...\n")
					logCtx.WriteLog("DOWNLOAD", "Clearing target directory: %s", flags.TargetDir)
					if err := utils.ClearDirectory(flags.TargetDir); err != nil {
						logCtx.WriteLog("DOWNLOAD", "Failed to clear target directory: %v", err)
						i18n.Printf("Error: Failed to clear target directory: %v\n", err)
						os.Exit(1)
					}
					logCtx.WriteLog("DOWNLOAD", "Target directory cleared successfully")
					i18n.Printf("Target directory cleared. Proceeding with extraction...\n")
				}
			} else {
				logCtx.WriteLog("DOWNLOAD", "Target path exists but is not a directory: %s", flags.TargetDir)
				i18n.Printf("Error: Target path '%s' exists but is not a directory\n", flags.TargetDir)
				os.Exit(1)
			}
		}

		// Set default output path if not specified (for qpress temp file)
		if outputPath == "" && downloadCompressType == "qp" {
			timestamp := time.Now().Format("20060102150405")
			outputPath = fmt.Sprintf("backup_%s.xb", timestamp)
		}

		if downloadCompressType != "" {
			i18n.Printf("[backup-helper] Receiving backup data (compression: %s)...\n", downloadCompressType)
			logCtx.WriteLog("DOWNLOAD", "Receiving compressed backup data (compression: %s)", downloadCompressType)
		} else {
			i18n.Printf("[backup-helper] Receiving backup data (no compression)...\n")
			logCtx.WriteLog("DOWNLOAD", "Receiving uncompressed backup data")
		}
		i18n.Printf("[backup-helper] Extracting to directory: %s\n", flags.TargetDir)
		logCtx.WriteLog("DOWNLOAD", "Extracting to directory: %s", flags.TargetDir)

		err := extract.ExtractBackupStream(reader, downloadCompressType, flags.TargetDir, outputPath, cfg.Parallel, cfg, logCtx)
		if err != nil {
			logCtx.WriteLog("EXTRACT", "Extraction error: %v", err)
			// Read log content for error extraction
			logContent, err2 := os.ReadFile(logCtx.GetFileName())
			if err2 == nil {
				errorSummary := log.ExtractErrorSummary("EXTRACT", string(logContent))
				if errorSummary != "" {
					i18n.Printf("Extraction error. Error summary:\n%s\n", errorSummary)
				} else {
					i18n.Printf("Extraction error: %v\n", err)
				}
			} else {
				i18n.Printf("Extraction error: %v\n", err)
			}
			i18n.Printf("Log file: %s\n", logCtx.GetFileName())

			// Prompt for AI diagnosis
			if flags.AIDiagnoseFlag == "on" {
				if utils.PromptAIDiagnosis(flags.AutoYes) {
					if cfg.QwenAPIKey == "" {
						i18n.Printf("Qwen API Key is required for AI diagnosis. Please set it in config.\n")
						os.Exit(1)
					}
					logContent, _ := os.ReadFile(logCtx.GetFileName())
					aiSuggestion, err := ai.DiagnoseWithAliQwen(cfg, "EXTRACT", string(logContent))
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
		i18n.Printf("[backup-helper] Extraction completed to: %s\n", flags.TargetDir)
		logCtx.WriteLog("DOWNLOAD", "Extraction completed successfully")
		logCtx.MarkSuccess()
		i18n.Printf("[backup-helper] Log file: %s\n", logCtx.GetFileName())
	} else if outputPath == "-" {
		// Stream to stdout - set tracker to output progress to stderr
		if tracker != nil {
			tracker.SetOutputToStderr(true)
		}
		i18n.Fprintf(os.Stderr, "[backup-helper] Receiving backup data and streaming to stdout...\n")

		// If compression type is specified and outputting to stdout, handle decompression for piping
		if downloadCompressType == "zstd" {
			// Decompress zstd stream for piping to xbstream
			decompressedReader, decompressCmd, err := extract.ExtractBackupStreamToStdout(reader, downloadCompressType, cfg.Parallel, logCtx)
			if err != nil {
				logCtx.WriteLog("DECOMPRESS", "Decompression error: %v", err)
				i18n.Fprintf(os.Stderr, "Decompression error: %v\n", err)
				os.Exit(1)
			}
			if decompressCmd != nil {
				defer decompressCmd.Wait()
			}
			reader = decompressedReader
		} else if downloadCompressType == "qp" {
			logCtx.WriteLog("DOWNLOAD", "Warning: qpress compression cannot be stream-decompressed")
			i18n.Fprintf(os.Stderr, "Warning: qpress compression cannot be stream-decompressed. Please save to file first.\n")
		}

		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			// Check if the error is related to connection interruption
			errStr := err.Error()
			if utils.Contains(errStr, "connection closed unexpectedly") || utils.Contains(errStr, "EOF") || utils.Contains(errStr, "broken pipe") {
				logCtx.WriteLog("TCP", "Connection interrupted during transfer: %v", err)
				i18n.Fprintf(os.Stderr, "Transfer interrupted: connection closed unexpectedly\n")
				i18n.Fprintf(os.Stderr, "Error details: %v\n", err)
			} else {
				logCtx.WriteLog("DOWNLOAD", "Download error: %v", err)
				i18n.Fprintf(os.Stderr, "Download error: %v\n", err)
			}
			i18n.Fprintf(os.Stderr, "Log file: %s\n", logCtx.GetFileName())
			os.Exit(1)
		}
		// Progress tracker will display completion message via closer()
	} else {
		// Write to file
		i18n.Printf("[backup-helper] Receiving backup data and saving to: %s\n", outputPath)
		logCtx.WriteLog("DOWNLOAD", "Saving backup data to: %s", outputPath)
		if downloadCompressType == "zstd" {
			// Save decompressed zstd stream
			err := extract.ExtractBackupStream(reader, downloadCompressType, "", outputPath, cfg.Parallel, cfg, logCtx)
			if err != nil {
				logCtx.WriteLog("EXTRACT", "Save error: %v", err)
				i18n.Printf("Save error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Save as-is
			file, err := os.Create(outputPath)
			if err != nil {
				logCtx.WriteLog("DOWNLOAD", "Failed to create output file: %v", err)
				i18n.Printf("Failed to create output file: %v\n", err)
				os.Exit(1)
			}
			defer file.Close()

			_, err = io.Copy(file, reader)
			if err != nil {
				logCtx.WriteLog("DOWNLOAD", "Failed to save backup data: %v", err)
				i18n.Printf("Download error: %v\n", err)
				os.Exit(1)
			}
		}
		// Progress tracker will display completion message via closer()
		i18n.Printf("[backup-helper] Download completed! Saved to: %s\n", outputPath)
		logCtx.WriteLog("DOWNLOAD", "Download completed successfully")
		logCtx.MarkSuccess()
		i18n.Printf("[backup-helper] Log file: %s\n", logCtx.GetFileName())
	}
	return nil
}

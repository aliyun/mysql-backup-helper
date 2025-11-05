package main

import (
	"backup-helper/utils"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gioco-play/easy-i18n/i18n"
	"golang.org/x/term"
	"golang.org/x/text/language"
)

func main() {
	utils.InitI18nAuto()

	var doBackup bool
	var doDownload bool
	var configPath string
	var host, user, password string
	var port int
	var streamPort int
	var streamHost string
	var mode string
	var compressType string
	var langFlag string
	var aiDiagnoseFlag string
	var enableHandshake bool
	var streamKey string
	var existedBackup string
	var downloadOutput string
	var showVersion bool
	var estimatedSizeStr string
	var estimatedSize int64
	var ioLimitStr string
	var ioLimit int64
	var useSSH bool
	var remoteOutput string
	var extractDir string

	flag.BoolVar(&doBackup, "backup", false, "Run xtrabackup and upload to OSS")
	flag.BoolVar(&doDownload, "download", false, "Download backup from TCP stream (listen on port)")
	flag.StringVar(&downloadOutput, "output", "", "Output file path for download mode (use '-' for stdout, default: backup_YYYYMMDDHHMMSS.xb)")
	flag.StringVar(&extractDir, "extract-dir", "", "Directory to extract backup files (only for xbstream extraction, requires --compress-type)")
	flag.StringVar(&estimatedSizeStr, "estimated-size", "", "Estimated backup size with unit (e.g., '100MB', '1GB', '500KB') or bytes (for progress tracking)")
	flag.StringVar(&ioLimitStr, "io-limit", "", "IO bandwidth limit with unit (e.g., '100MB/s', '1GB/s', '500KB/s') or bytes per second. Use -1 for unlimited speed")
	flag.StringVar(&existedBackup, "existed-backup", "", "Path to existing xtrabackup backup file to upload (use '-' for stdin)")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (shorthand)")
	flag.StringVar(&configPath, "config", "", "config file path (optional)")
	flag.StringVar(&host, "host", "", "Connect to host")
	flag.IntVar(&port, "port", 0, "Port number to use for connection")
	flag.StringVar(&user, "user", "", "User for login")
	flag.StringVar(&password, "password", "", "Password to use when connecting to server. If password is not given it's asked from the tty.")
	flag.IntVar(&streamPort, "stream-port", 0, "Local TCP port for streaming (0 = auto-find available port), or remote port when --stream-host is specified")
	flag.StringVar(&streamHost, "stream-host", "", "Remote host IP for pushing data (e.g., '192.168.1.100'). When specified, actively connects to remote instead of listening locally")
	flag.StringVar(&mode, "mode", "oss", "Backup mode: oss (upload to OSS) or stream (push to TCP port)")
	flag.StringVar(&compressType, "compress-type", "", "Compress type: qp(qpress)/zstd, priority is higher than config file")
	flag.StringVar(&langFlag, "lang", "", "Language: zh (Chinese) or en (English), auto-detect if unset")
	flag.StringVar(&aiDiagnoseFlag, "ai-diagnose", "", "AI diagnosis on backup failure: on/off. If not set, prompt interactively.")
	flag.BoolVar(&enableHandshake, "enable-handshake", false, "Enable handshake for TCP streaming (default: false, can be set in config)")
	flag.StringVar(&streamKey, "stream-key", "", "Handshake key for TCP streaming (default: empty, can be set in config)")
	flag.BoolVar(&useSSH, "ssh", false, "Use SSH to start receiver on remote host (requires --stream-host)")
	flag.StringVar(&remoteOutput, "remote-output", "", "Remote output path when using SSH mode (default: auto-generated)")

	flag.Parse()

	// check version parameter
	if showVersion {
		utils.PrintVersion()
		os.Exit(0)
	}

	// Set language if --lang is specified
	switch langFlag {
	case "cn", "zh":
		i18n.SetLang(language.SimplifiedChinese)
	case "en":
		i18n.SetLang(language.English)
	default:
		// use auto setting
	}

	// Load config
	var cfg *utils.Config
	if configPath != "" {
		var err error
		cfg, err = utils.LoadConfig(configPath)
		if err != nil {
			i18n.Printf("Load config error: %v\n", err)
			os.Exit(1)
		}
		cfg.SetDefaults()
	} else {
		cfg = &utils.Config{}
		cfg.SetDefaults()
	}

	// Fill parameters not specified by command line with config
	if host == "" {
		host = cfg.MysqlHost
	}
	if port == 0 {
		port = cfg.MysqlPort
	}
	if user == "" {
		user = cfg.MysqlUser
	}
	if password == "" {
		password = cfg.MysqlPassword
	}
	if compressType == "" && cfg.CompressType != "" {
		compressType = cfg.CompressType
	}
	if existedBackup == "" && cfg.ExistedBackup != "" {
		existedBackup = cfg.ExistedBackup
	}

	// Parse estimatedSize from command line or config
	if estimatedSizeStr != "" {
		parsedSize, err := utils.ParseSize(estimatedSizeStr)
		if err != nil {
			i18n.Printf("Error parsing --estimated-size '%s': %v\n", estimatedSizeStr, err)
			os.Exit(1)
		}
		estimatedSize = parsedSize
	} else if estimatedSize == 0 && cfg.EstimatedSize > 0 {
		estimatedSize = cfg.EstimatedSize
	}

	// Parse ioLimit from command line or config
	if ioLimitStr != "" {
		parsedLimit, err := utils.ParseRateLimit(ioLimitStr)
		if err != nil {
			i18n.Printf("Error parsing --io-limit '%s': %v\n", ioLimitStr, err)
			os.Exit(1)
		}
		ioLimit = parsedLimit
	} else if ioLimit == 0 && cfg.IOLimit > 0 {
		ioLimit = cfg.IOLimit
	}

	// Update traffic config based on ioLimit
	if ioLimit == -1 {
		cfg.Traffic = 0 // 0 means unlimited
	} else if ioLimit > 0 {
		cfg.Traffic = ioLimit
	}
	// If ioLimit is 0, cfg.Traffic will use default from SetDefaults()

	// 4. Handle --download mode
	if doDownload {
		// Display header (only if not outputting to stdout)
		if downloadOutput != "-" {
			outputHeader()
		} else {
			// When outputting to stdout, output header to stderr
			outputHeaderToStderr()
		}

		// Parse stream-port from command line or config
		if streamPort == 0 && !isFlagPassed("stream-port") && cfg.StreamPort > 0 {
			streamPort = cfg.StreamPort
		}

		// Parse handshake settings
		if !isFlagPassed("enable-handshake") {
			enableHandshake = cfg.EnableHandshake
		}
		if streamKey == "" {
			streamKey = cfg.StreamKey
		}

		// Parse compression type for download mode
		downloadCompressType := compressType
		if downloadCompressType == "" && cfg.CompressType != "" {
			downloadCompressType = cfg.CompressType
		}

		// Determine output file path
		outputPath := downloadOutput
		if outputPath == "" && cfg.DownloadOutput != "" {
			outputPath = cfg.DownloadOutput
		}
		if outputPath == "" && extractDir == "" {
			// Default: backup_YYYYMMDDHHMMSS.xb (only if not extracting)
			timestamp := time.Now().Format("20060102150405")
			outputPath = fmt.Sprintf("backup_%s.xb", timestamp)
		}

		// Display IO limit
		if outputPath == "-" {
			// Output to stderr when streaming to stdout
			if ioLimit == -1 {
				i18n.Fprintf(os.Stderr, "[backup-helper] Rate limiting disabled (unlimited speed)\n")
			} else if ioLimit > 0 {
				i18n.Fprintf(os.Stderr, "[backup-helper] IO rate limit set to: %s/s\n", formatBytes(ioLimit))
			} else if cfg.Traffic > 0 {
				i18n.Fprintf(os.Stderr, "[backup-helper] IO rate limit set to: %s/s (default)\n", formatBytes(cfg.Traffic))
			}
		} else {
			// Output to stdout when saving to file
			if ioLimit == -1 {
				i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
			} else if ioLimit > 0 {
				i18n.Printf("[backup-helper] IO rate limit set to: %s/s\n", formatBytes(ioLimit))
			} else if cfg.Traffic > 0 {
				i18n.Printf("[backup-helper] IO rate limit set to: %s/s (default)\n", formatBytes(cfg.Traffic))
			}
		}

		// Start TCP receiver
		receiver, tracker, closer, actualPort, localIP, err := utils.StartStreamReceiver(streamPort, enableHandshake, streamKey, estimatedSize)
		_ = actualPort // Port info already displayed in StartStreamReceiver
		_ = localIP    // IP info already displayed in StartStreamReceiver
		if err != nil {
			i18n.Fprintf(os.Stderr, "Stream receiver error: %v\n", err)
			os.Exit(1)
		}
		defer closer() // This will call tracker.Complete() internally

		// Apply rate limiting if configured
		var reader io.Reader = receiver
		if cfg.Traffic > 0 {
			rateLimitedReader := utils.NewRateLimitedReader(receiver, cfg.Traffic)
			reader = rateLimitedReader
		}

		// Determine output destination and handle extraction
		if extractDir != "" {
			// Extraction mode: decompress and extract
			if downloadCompressType == "" {
				i18n.Printf("Error: --extract-dir requires --compress-type to be specified\n")
				os.Exit(1)
			}
			if outputPath == "-" {
				i18n.Printf("Error: --extract-dir cannot be used with --output -\n")
				os.Exit(1)
			}

			// Set default output path if not specified (for qpress temp file)
			if outputPath == "" {
				timestamp := time.Now().Format("20060102150405")
				outputPath = fmt.Sprintf("backup_%s.xb", timestamp)
			}

			i18n.Printf("[backup-helper] Receiving backup data (compression: %s)...\n", downloadCompressType)
			i18n.Printf("[backup-helper] Extracting to directory: %s\n", extractDir)

			err := utils.ExtractBackupStream(reader, downloadCompressType, extractDir, outputPath)
			if err != nil {
				i18n.Printf("Extraction error: %v\n", err)
				os.Exit(1)
			}
			i18n.Printf("[backup-helper] Extraction completed to: %s\n", extractDir)
		} else if outputPath == "-" {
			// Stream to stdout - set tracker to output progress to stderr
			if tracker != nil {
				tracker.SetOutputToStderr(true)
			}
			i18n.Fprintf(os.Stderr, "[backup-helper] Receiving backup data and streaming to stdout...\n")
			
			// If compression type is specified and outputting to stdout, handle decompression for piping
			if downloadCompressType == "zstd" {
				// Decompress zstd stream for piping to xbstream
				decompressedReader, decompressCmd, err := utils.ExtractBackupStreamToStdout(reader, downloadCompressType)
				if err != nil {
					i18n.Fprintf(os.Stderr, "Decompression error: %v\n", err)
					os.Exit(1)
				}
				if decompressCmd != nil {
					defer decompressCmd.Wait()
				}
				reader = decompressedReader
			} else if downloadCompressType == "qp" {
				i18n.Fprintf(os.Stderr, "Warning: qpress compression cannot be stream-decompressed. Please save to file first.\n")
			}

			_, err = io.Copy(os.Stdout, reader)
			if err != nil {
				i18n.Fprintf(os.Stderr, "Download error: %v\n", err)
				os.Exit(1)
			}
			// Progress tracker will display completion message via closer()
		} else {
			// Write to file
			i18n.Printf("[backup-helper] Receiving backup data and saving to: %s\n", outputPath)
			if downloadCompressType == "zstd" {
				// Save decompressed zstd stream
				err := utils.ExtractBackupStream(reader, downloadCompressType, "", outputPath)
				if err != nil {
					i18n.Printf("Save error: %v\n", err)
					os.Exit(1)
				}
			} else {
				// Save as-is
				file, err := os.Create(outputPath)
				if err != nil {
					i18n.Printf("Failed to create output file: %v\n", err)
					os.Exit(1)
				}
				defer file.Close()

				_, err = io.Copy(file, reader)
				if err != nil {
					i18n.Printf("Download error: %v\n", err)
					os.Exit(1)
				}
			}
			// Progress tracker will display completion message via closer()
			i18n.Printf("[backup-helper] Saved to: %s\n", outputPath)
		}
		return
	}

	// 5. If --backup, run backup/upload
	if doBackup {
		// MySQL param check (only needed for backup)
		if password == "" {
			i18n.Printf("Please input mysql-server password: ")
			pwd, _ := term.ReadPassword(0)
			i18n.Printf("\n")
			password = string(pwd)
		}

		i18n.Printf("connect to mysql-server host=%s port=%d user=%s\n", host, port, user)
		outputHeader()
		db := utils.GetConnection(host, port, user, password)
		defer db.Close()
		options := utils.CollectVariableFromMySQLServer(db)
		utils.Check(options, cfg)

		// Display IO limit after parameter check
		if ioLimit == -1 {
			i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
		} else if ioLimit > 0 {
			i18n.Printf("[backup-helper] IO rate limit set to: %s/s\n", formatBytes(ioLimit))
		} else if cfg.Traffic > 0 {
			// Using default rate limit
			i18n.Printf("[backup-helper] IO rate limit set to: %s/s (default)\n", formatBytes(cfg.Traffic))
		}

		// Check xtrabackup version (run early)
		mysqlVer := cfg.MysqlVersion
		utils.CheckXtraBackupVersion(mysqlVer)

		i18n.Printf("[backup-helper] Running xtrabackup...\n")
		cfg.MysqlHost = host
		cfg.MysqlPort = port
		cfg.MysqlUser = user
		cfg.MysqlPassword = password

		// 1. Decide objectName suffix and compression param
		ossObjectName := cfg.ObjectName
		objectSuffix := ".xb"
		// compressType default is empty
		if mode == "stream" {
			cfg.Compress = false
			cfg.CompressType = ""
			objectSuffix = ".xb"
		} else if cfg.Compress {
			switch compressType {
			case "zstd":
				objectSuffix = ".xb.zst"
				cfg.CompressType = "zstd"
			default:
				objectSuffix = "_qp.xb"
				cfg.CompressType = ""
			}
		} else {
			objectSuffix = ".xb"
			cfg.CompressType = ""
		}
		timestamp := time.Now().Format("_20060102150405")
		fullObjectName := ossObjectName + timestamp + objectSuffix

		reader, cmd, logFileName, err := utils.RunXtraBackup(cfg)
		if err != nil {
			i18n.Printf("Run xtrabackup error: %v\n", err)
			os.Exit(1)
		}

		// Calculate total size for progress tracking
		var totalSize int64
		if estimatedSize > 0 {
			totalSize = estimatedSize
			i18n.Printf("[backup-helper] Using estimated size: %s\n", formatBytes(totalSize))
		} else {
			// Calculate datadir size
			datadir, err := utils.GetDatadirFromMySQL(db)
			if err != nil {
				i18n.Printf("Warning: Could not get datadir, progress tracking will be limited: %v\n", err)
			} else {
				totalSize, err = utils.CalculateBackupSize(datadir)
				if err != nil {
					i18n.Printf("Warning: Could not calculate backup size, progress tracking will be limited: %v\n", err)
					totalSize = 0
				} else {
					i18n.Printf("[backup-helper] Calculated datadir size: %s\n", formatBytes(totalSize))
				}
			}
		}

		switch mode {
		case "oss":
			i18n.Printf("[backup-helper] Uploading to OSS...\n")
			err = utils.UploadReaderToOSS(cfg, fullObjectName, reader, totalSize)
			if err != nil {
				i18n.Printf("OSS upload error: %v\n", err)
				cmd.Process.Kill()
				os.Exit(1)
			}
		case "stream":
			// Parse stream-host from command line or config
			if streamHost == "" && cfg.StreamHost != "" {
				streamHost = cfg.StreamHost
			}

			// Parse remote-output from command line or config (if exists)
			if remoteOutput == "" && cfg.RemoteOutput != "" {
				remoteOutput = cfg.RemoteOutput
			}

			// Validate SSH mode requirements
			if useSSH && streamHost == "" {
				i18n.Printf("Error: --ssh requires --stream-host\n")
				cmd.Process.Kill()
				os.Exit(1)
			}

			// handshake priority：command line > config > default
			if !isFlagPassed("enable-handshake") {
				enableHandshake = cfg.EnableHandshake
			}
			if streamKey == "" {
				streamKey = cfg.StreamKey
			}

			var writer io.WriteCloser
			var closer func()
			var err error

			if streamHost != "" {
				if useSSH {
					// SSH mode: Start receiver on remote via SSH
					i18n.Printf("[backup-helper] Starting remote receiver via SSH on %s...\n", streamHost)

					// Use stream-port if specified, otherwise auto-find (0)
					sshPort := streamPort
					if !isFlagPassed("stream-port") && cfg.StreamPort > 0 {
						sshPort = cfg.StreamPort
					}

					remotePort, outputPath, _, sshCleanup, err := utils.StartRemoteReceiverViaSSH(
						streamHost, sshPort, remoteOutput, totalSize, enableHandshake, streamKey)
					if err != nil {
						i18n.Printf("SSH receiver error: %v\n", err)
						cmd.Process.Kill()
						os.Exit(1)
					}

					streamPort = remotePort
					if sshPort > 0 {
						i18n.Printf("[backup-helper] Remote receiver started on port %d via SSH\n", streamPort)
					} else {
						i18n.Printf("[backup-helper] Remote receiver started on auto-discovered port %d via SSH\n", streamPort)
					}

					// Display remote output path (show what was specified, or indicate auto-generated)
					if outputPath != "" {
						i18n.Printf("[backup-helper] Remote backup will be saved to: %s\n", outputPath)
					} else if remoteOutput != "" {
						i18n.Printf("[backup-helper] Remote backup will be saved to: %s\n", remoteOutput)
					} else {
						i18n.Printf("[backup-helper] Remote backup will be saved to: auto-generated path (backup_YYYYMMDDHHMMSS.xb)\n")
					}

					// Connect to remote receiver
					writer, _, closer, _, err = utils.StartStreamClient(
						streamHost, streamPort, enableHandshake, streamKey, totalSize)
					if err != nil {
						sshCleanup()
						i18n.Printf("Stream client error: %v\n", err)
						cmd.Process.Kill()
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
					if streamPort == 0 && !isFlagPassed("stream-port") {
						if cfg.StreamPort > 0 {
							streamPort = cfg.StreamPort
						} else {
							i18n.Printf("Error: --stream-port is required when using --stream-host\n")
							cmd.Process.Kill()
							os.Exit(1)
						}
					}

					writer, _, closer, _, err = utils.StartStreamClient(
						streamHost, streamPort, enableHandshake, streamKey, totalSize)
					if err != nil {
						i18n.Printf("Stream client error: %v\n", err)
						cmd.Process.Kill()
				os.Exit(1)
			}
				}
			} else {
				// Passive connection: listen locally and wait for connection
				// streamPort can be 0 now (auto-find available port)
				if streamPort == 0 && !isFlagPassed("stream-port") && cfg.StreamPort > 0 {
					streamPort = cfg.StreamPort
				}

				tcpWriter, _, closerFunc, actualPort, localIP, err := utils.StartStreamSender(streamPort, enableHandshake, streamKey, totalSize)
				_ = actualPort // Port info already displayed in StartStreamSender
				_ = localIP    // IP info already displayed in StartStreamSender
			if err != nil {
				i18n.Printf("Stream server error: %v\n", err)
					cmd.Process.Kill()
				os.Exit(1)
				}
				writer = tcpWriter
				closer = closerFunc
			}
			defer closer()

			// Apply rate limiting for stream mode if configured
			var finalWriter io.WriteCloser = writer
			if cfg.Traffic > 0 {
				rateLimitedWriter := utils.NewRateLimitedWriter(writer, cfg.Traffic)
				finalWriter = rateLimitedWriter
			}

			_, err = io.Copy(finalWriter, reader)
			if err != nil {
				i18n.Printf("TCP stream error: %v\n", err)
				cmd.Process.Kill()
				os.Exit(1)
			}
		default:
			i18n.Printf("Unknown mode: %s\n", mode)
			os.Exit(1)
		}

		cmd.Wait()
		// backup log file needs to be closed
		utils.CloseBackupLogFile(cmd)
		// Check backup log
		logContent, err := os.ReadFile(logFileName)
		if err != nil {
			i18n.Printf("Backup log read error.\n")
			os.Exit(1)
		}
		if !strings.Contains(string(logContent), "completed OK!") {
			i18n.Printf("Backup failed (no 'completed OK!').\n")
			i18n.Printf("You can check the backup log file for details: %s\n", logFileName)

			switch aiDiagnoseFlag {
			case "on":
				if cfg.QwenAPIKey == "" {
					i18n.Printf("Qwen API Key is required for AI diagnosis. Please set it in config.\n")
					os.Exit(1)
				}
				aiSuggestion, err := utils.DiagnoseWithAliQwen(cfg, string(logContent))
				if err != nil {
					i18n.Printf("AI diagnosis failed: %v\n", err)
				} else {
					fmt.Print(color.YellowString(i18n.Sprintf("AI diagnosis suggestion:\n")))
					fmt.Println(color.YellowString(aiSuggestion))
				}
			case "off":
				// do nothing, skip ai diagnose
			default:
				var input string
				i18n.Printf("Would you like to use AI diagnosis? (y/n): ")
				fmt.Scanln(&input)
				if input == "y" || input == "Y" || input == "yes" || input == "Yes" {
					aiSuggestion, err := utils.DiagnoseWithAliQwen(cfg, string(logContent))
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
		// Ensure a newline before completion message (in case progress tracker didn't clear properly)
		fmt.Print("\n")
		i18n.Printf("[backup-helper] Backup and upload completed!\n")
		return
	} else if existedBackup != "" {
		// upload existed backup file to OSS or stream via TCP
		i18n.Printf("[backup-helper] Processing existing backup file...\n")

		// Validate backup file before processing
		var backupInfo *utils.BackupFileInfo
		var err error

		if existedBackup == "-" {
			// Validate data from stdin
			backupInfo, err = utils.ValidateBackupFileFromStdin()
			if err != nil {
				i18n.Printf("Validation error: %v\n", err)
				os.Exit(1)
			}
			utils.PrintBackupFileValidationFromStdin(backupInfo)
		} else {
			// Validate file
			backupInfo, err = utils.ValidateBackupFile(existedBackup)
			if err != nil {
				i18n.Printf("Validation error: %v\n", err)
				os.Exit(1)
			}
			utils.PrintBackupFileValidation(existedBackup, backupInfo)
		}

		// Exit if backup file is invalid
		if !backupInfo.IsValid {
			i18n.Printf("[backup-helper] Cannot proceed with invalid backup file.\n")
			os.Exit(1)
		}

		// Display IO limit after validation
		if ioLimit == -1 {
			i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
		} else if ioLimit > 0 {
			i18n.Printf("[backup-helper] IO rate limit set to: %s/s\n", formatBytes(ioLimit))
		} else if cfg.Traffic > 0 {
			// Using default rate limit
			i18n.Printf("[backup-helper] IO rate limit set to: %s/s (default)\n", formatBytes(cfg.Traffic))
		}

		// Get reader from existing backup file or stdin
		var reader io.Reader
		if existedBackup == "-" {
			// Read from stdin (for cat command)
			reader = os.Stdin
			i18n.Printf("[backup-helper] Reading backup data from stdin...\n")
		} else {
			// Read from file
			file, err := os.Open(existedBackup)
			if err != nil {
				i18n.Printf("Open backup file error: %v\n", err)
				os.Exit(1)
			}
			defer file.Close()
			reader = file
			i18n.Printf("[backup-helper] Reading backup data from file: %s\n", existedBackup)
		}

		// Determine object name suffix based on compression type
		ossObjectName := cfg.ObjectName
		objectSuffix := ".xb"
		if mode == "stream" {
			cfg.Compress = false
			cfg.CompressType = ""
			objectSuffix = ".xb"
		} else if cfg.Compress {
			switch compressType {
			case "zstd":
				objectSuffix = ".xb.zst"
				cfg.CompressType = "zstd"
			default:
				objectSuffix = "_qp.xb"
				cfg.CompressType = ""
			}
		} else {
			objectSuffix = ".xb"
			cfg.CompressType = ""
		}
		timestamp := time.Now().Format("_20060102150405")
		fullObjectName := ossObjectName + timestamp + objectSuffix

		// Calculate total size for existing backup
		var totalSize int64
		if estimatedSize > 0 {
			totalSize = estimatedSize
			i18n.Printf("[backup-helper] Using estimated size: %s\n", formatBytes(totalSize))
		} else if existedBackup != "-" {
			// Get file size for existing backup file
			totalSize, err = utils.GetFileSize(existedBackup)
			if err != nil {
				i18n.Printf("Warning: Could not get backup file size, progress tracking will be limited: %v\n", err)
				totalSize = 0
			} else {
				i18n.Printf("[backup-helper] Backup file size: %s\n", formatBytes(totalSize))
			}
		} else {
			// stdin - we can't get size
			i18n.Printf("[backup-helper] Uploading from stdin, size unknown\n")
		}

		switch mode {
		case "oss":
			i18n.Printf("[backup-helper] Uploading existing backup to OSS...\n")
			err := utils.UploadReaderToOSS(cfg, fullObjectName, reader, totalSize)
			if err != nil {
				i18n.Printf("OSS upload error: %v\n", err)
				os.Exit(1)
			}
			i18n.Printf("[backup-helper] OSS upload completed!\n")
		case "stream":
			// Parse stream-host from command line or config
			if streamHost == "" && cfg.StreamHost != "" {
				streamHost = cfg.StreamHost
			}

			// Only use config value if command line didn't specify and config has non-zero value
			// streamPort 0 means auto-find available port (only when not using stream-host)
			if streamHost == "" {
				if streamPort == 0 && !isFlagPassed("stream-port") && cfg.StreamPort > 0 {
					streamPort = cfg.StreamPort
				}
				// Show equivalent command (before starting server, so we show original port)
				equivalentSource := existedBackup
				if existedBackup == "-" {
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
				if streamPort == 0 && !isFlagPassed("stream-port") {
					if cfg.StreamPort > 0 {
				streamPort = cfg.StreamPort
					} else {
						i18n.Printf("Error: --stream-port is required when using --stream-host\n")
						os.Exit(1)
					}
				}
			}

			// handshake priority：command line > config > default
			if !isFlagPassed("enable-handshake") {
				enableHandshake = cfg.EnableHandshake
			}
			if streamKey == "" {
				streamKey = cfg.StreamKey
			}

			var writer io.WriteCloser
			var closer func()
			var err error

			if streamHost != "" {
				// Active connection: connect to remote server
				writer, _, closer, _, err = utils.StartStreamClient(streamHost, streamPort, enableHandshake, streamKey, totalSize)
				if err != nil {
					i18n.Printf("Stream client error: %v\n", err)
				os.Exit(1)
			}
			} else {
				// Passive connection: listen locally and wait for connection
				// streamPort can be 0 now (auto-find available port)
				tcpWriter, _, closerFunc, actualPort, localIP, err := utils.StartStreamSender(streamPort, enableHandshake, streamKey, totalSize)
				_ = actualPort // Port info already displayed in StartStreamSender
				_ = localIP    // IP info already displayed in StartStreamSender
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
			if cfg.Traffic > 0 {
				rateLimitedWriter := utils.NewRateLimitedWriter(writer, cfg.Traffic)
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
		default:
			i18n.Printf("Unknown mode: %s\n", mode)
			os.Exit(1)
		}
		return
	}
}

func outputHeader() {
	bar := strings.Repeat("#", 80)
	title := "MySQL Backup Helper"
	subtitle := "Powered by Alibaba Cloud Inc"
	version := "v1.0.0"
	timeStr := time.Now().Format("2006-01-02 15:04:05")

	i18n.Printf("%s\n", bar)
	// center display
	pad := (80 - len(title)) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Printf("%s%s\n", strings.Repeat(" ", pad), title)
	pad2 := (80 - len(subtitle)) / 2
	if pad2 < 0 {
		pad2 = 0
	}
	fmt.Printf("%s%s\n", strings.Repeat(" ", pad2), subtitle)
	fmt.Printf("%sVersion: %s    Time: %s\n", strings.Repeat(" ", 10), version, timeStr)
	i18n.Printf("%s\n", bar)
}

func outputHeaderToStderr() {
	bar := strings.Repeat("#", 80)
	title := "MySQL Backup Helper"
	subtitle := "Powered by Alibaba Cloud Inc"
	version := "v1.0.0"
	timeStr := time.Now().Format("2006-01-02 15:04:05")

	fmt.Fprintf(os.Stderr, "%s\n", bar)
	// center display
	pad := (80 - len(title)) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(os.Stderr, "%s%s\n", strings.Repeat(" ", pad), title)
	pad2 := (80 - len(subtitle)) / 2
	if pad2 < 0 {
		pad2 = 0
	}
	fmt.Fprintf(os.Stderr, "%s%s\n", strings.Repeat(" ", pad2), subtitle)
	fmt.Fprintf(os.Stderr, "%sVersion: %s    Time: %s\n", strings.Repeat(" ", 10), version, timeStr)
	i18n.Fprintf(os.Stderr, "%s\n", bar)
}

// check if command line parameter is set
func isFlagPassed(name string) bool {
	found := false
	flag.CommandLine.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// formatBytes formats bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

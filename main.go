package main

import (
	"backup-helper/utils"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gioco-play/easy-i18n/i18n"
	"golang.org/x/term"
	"golang.org/x/text/language"
)

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// isDirEmpty checks if a directory is empty
func isDirEmpty(dir string) (bool, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil // Directory doesn't exist, consider it empty
		}
		return false, err
	}

	if !info.IsDir() {
		return false, fmt.Errorf("%s is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	return len(entries) == 0, nil
}

// promptOverwrite asks user if they want to overwrite existing files in targetDir
// If autoYes is true, automatically returns true and shows a warning
func promptOverwrite(targetDir string, autoYes bool) bool {
	i18n.Printf("Warning: Target directory '%s' already exists and is not empty.\n", targetDir)
	i18n.Printf("Extracting to this directory may overwrite existing files.\n")

	if autoYes {
		i18n.Printf("Auto-confirming overwrite (--yes/-y flag is set)...\n")
		return true
	}

	i18n.Printf("Do you want to continue? (y/n): ")

	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "y" || input == "yes"
}

// promptAIDiagnosis asks user if they want to use AI diagnosis
// If autoYes is true, automatically returns true and shows a warning
func promptAIDiagnosis(autoYes bool) bool {
	if autoYes {
		i18n.Printf("Auto-confirming AI diagnosis (--yes/-y flag is set)...\n")
		return true
	}

	var input string
	i18n.Printf("Would you like to use AI diagnosis? (y/n): ")
	fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// clearDirectory removes all files and subdirectories in the given directory
func clearDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		} else {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}

	return nil
}

func main() {
	utils.InitI18nAuto()

	var doBackup bool
	var doDownload bool
	var doPrepare bool
	var doCheck bool
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
	var useSSH bool
	var remoteOutput string
	var targetDir string
	var parallel int
	var useMemory string
	var autoYes bool
	var xtrabackupPath string
	var defaultsFile string

	flag.BoolVar(&doBackup, "backup", false, "Run xtrabackup and upload to OSS")
	flag.BoolVar(&autoYes, "y", false, "Automatically answer 'yes' to all prompts (non-interactive mode)")
	flag.BoolVar(&autoYes, "yes", false, "Automatically answer 'yes' to all prompts (non-interactive mode)")
	flag.BoolVar(&doDownload, "download", false, "Download backup from TCP stream (listen on port)")
	flag.BoolVar(&doPrepare, "prepare", false, "Prepare backup for restore (xtrabackup --prepare)")
	flag.BoolVar(&doCheck, "check", false, "Perform pre-flight validation checks (dependencies, MySQL compatibility, system resources, parameter recommendations)")
	flag.StringVar(&downloadOutput, "output", "", "Output file path for download mode (use '-' for stdout, default: backup_YYYYMMDDHHMMSS.xb)")
	flag.StringVar(&targetDir, "target-dir", "", "Directory for extraction (download mode) or backup directory (prepare mode)")
	flag.StringVar(&estimatedSizeStr, "estimated-size", "", "Estimated backup size with unit (e.g., '100MB', '1GB', '500KB') or bytes (for progress tracking)")
	flag.StringVar(&ioLimitStr, "io-limit", "", "IO bandwidth limit with unit (e.g., '100MB/s', '1GB/s', '500KB/s') or bytes per second. Use -1 for unlimited speed")
	flag.StringVar(&useMemory, "use-memory", "", "Memory to use for prepare operation (e.g., '1G', '512M'). Default: 1G")
	flag.StringVar(&xtrabackupPath, "xtrabackup-path", "", "Path to xtrabackup binary or directory containing xtrabackup/xbstream (overrides config and environment variable)")
	flag.StringVar(&defaultsFile, "defaults-file", "", "Path to MySQL configuration file (my.cnf). If not specified, --defaults-file will not be passed to xtrabackup")
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
	flag.StringVar(&compressType, "compress", "__NOT_SET__", "Compression: qp(qpress)/zstd/no, or no value (default: qp). Priority is higher than config file")
	flag.StringVar(&langFlag, "lang", "", "Language: zh (Chinese) or en (English), auto-detect if unset")
	flag.StringVar(&aiDiagnoseFlag, "ai-diagnose", "", "AI diagnosis on backup failure: on/off. If not set, prompt interactively.")
	flag.BoolVar(&enableHandshake, "enable-handshake", false, "Enable handshake for TCP streaming (default: false, can be set in config)")
	flag.StringVar(&streamKey, "stream-key", "", "Handshake key for TCP streaming (default: empty, can be set in config)")
	flag.BoolVar(&useSSH, "ssh", false, "Use SSH to start receiver on remote host (requires --stream-host)")
	flag.StringVar(&remoteOutput, "remote-output", "", "Remote output path when using SSH mode (default: auto-generated)")
	flag.IntVar(&parallel, "parallel", 0, "Number of parallel threads for xtrabackup (default: 4)")

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
	// Handle --compress flag
	// If compressType is "__NOT_SET__", flag was not passed
	// If compressType is "" and flag was passed, user passed --compress without value (default to qp)
	// If compressType has a value, use it
	if compressType == "__NOT_SET__" {
		// Flag was not passed, use config or empty
		if cfg.CompressType != "" {
			compressType = cfg.CompressType
		} else {
			compressType = ""
		}
	} else {
		// Flag was passed
		if compressType == "" {
			// --compress was passed but empty value (--compress= or --compress ""), default to qp
			compressType = "qp"
		}
		// Otherwise use the provided value (zstd, qp, no, etc.)
	}
	// Normalize: "no" means no compression
	if compressType == "no" {
		compressType = ""
	}
	if existedBackup == "" && cfg.ExistedBackup != "" {
		existedBackup = cfg.ExistedBackup
	}

	// Handle --xtrabackup-path flag (command-line flag overrides config)
	if xtrabackupPath != "" {
		cfg.XtrabackupPath = xtrabackupPath
	} else if cfg.XtrabackupPath == "" {
		// If not set in flag or config, check environment variable
		// (ResolveXtrabackupPath will handle this, but we can also set it here for consistency)
		if envPath := os.Getenv("XTRABACKUP_PATH"); envPath != "" {
			cfg.XtrabackupPath = envPath
		}
	}

	// Handle --defaults-file flag (command-line flag overrides config)
	if defaultsFile != "" {
		cfg.DefaultsFile = defaultsFile
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
		cfg.IOLimit = parsedLimit
	}
	// cfg.IOLimit now contains: -1 (unlimited), 0 (use default), or >0 (specified value)

	// Parse parallel from command line or config
	if parallel > 0 {
		cfg.Parallel = parallel
	} else if parallel == 0 && cfg.Parallel == 0 {
		// Use default (4) if not specified in command line or config
		cfg.Parallel = 4
	}

	// Parse useMemory from command line or config
	if useMemory != "" {
		cfg.UseMemory = useMemory
	} else if cfg.UseMemory == "" {
		// Use default (1G) if not specified in command line or config
		cfg.UseMemory = "1G"
	}

	// 4. Handle --check mode
	if doCheck {
		outputHeader()

		// Determine effective compression type
		effectiveCompressType := compressType
		if effectiveCompressType == "__NOT_SET__" {
			effectiveCompressType = cfg.CompressType
		}
		if effectiveCompressType == "no" {
			effectiveCompressType = ""
		}

		// Get MySQL connection if available
		var db *sql.DB
		if host != "" && user != "" {
			if password == "" {
				i18n.Printf("Please input mysql-server password (optional, for MySQL compatibility checks): ")
				pwd, _ := term.ReadPassword(0)
				i18n.Printf("\n")
				password = string(pwd)
			}
			if password != "" {
				i18n.Printf("Connecting to MySQL server for compatibility checks...\n")
				db = utils.GetConnection(host, port, user, password)
				defer db.Close()
			}
		}

		// Check if --check is combined with other modes
		if doBackup {
			// --check --backup: only check backup mode
			i18n.Printf("[backup-helper] Running pre-flight checks for BACKUP mode...\n\n")
			results := utils.CheckForBackupMode(cfg, effectiveCompressType, db)
			utils.PrintCheckResults(i18n.Sprintf("Backup Mode Checks"), results)

			hasCriticalError := false
			for _, result := range results {
				if result.Status == "ERROR" {
					hasCriticalError = true
					break
				}
			}

			i18n.Printf("\n=== %s ===\n", i18n.Sprintf("Check Summary"))
			if hasCriticalError {
				i18n.Printf("[ERROR] Critical errors found. Please fix them before proceeding with backup.\n")
				os.Exit(1)
			} else {
				i18n.Printf("[OK] Pre-flight checks completed. Backup mode is ready.\n")
			}
			return
		} else if doDownload {
			// --check --download: only check download mode
			i18n.Printf("[backup-helper] Running pre-flight checks for DOWNLOAD mode...\n\n")
			results := utils.CheckForDownloadMode(cfg, effectiveCompressType, targetDir)
			utils.PrintCheckResults(i18n.Sprintf("Download Mode Checks"), results)

			hasCriticalError := false
			for _, result := range results {
				if result.Status == "ERROR" {
					hasCriticalError = true
					break
				}
			}

			i18n.Printf("\n=== %s ===\n", i18n.Sprintf("Check Summary"))
			if hasCriticalError {
				i18n.Printf("[ERROR] Critical errors found. Please fix them before proceeding with download.\n")
				os.Exit(1)
			} else {
				i18n.Printf("[OK] Pre-flight checks completed. Download mode is ready.\n")
			}
			return
		} else if doPrepare {
			// --check --prepare: only check prepare mode
			i18n.Printf("[backup-helper] Running pre-flight checks for PREPARE mode...\n\n")
			results := utils.CheckForPrepareMode(cfg, targetDir, db)
			utils.PrintCheckResults(i18n.Sprintf("Prepare Mode Checks"), results)

			hasCriticalError := false
			for _, result := range results {
				if result.Status == "ERROR" {
					hasCriticalError = true
					break
				}
			}

			i18n.Printf("\n=== %s ===\n", i18n.Sprintf("Check Summary"))
			if hasCriticalError {
				i18n.Printf("[ERROR] Critical errors found. Please fix them before proceeding with prepare.\n")
				os.Exit(1)
			} else {
				i18n.Printf("[OK] Pre-flight checks completed. Prepare mode is ready.\n")
			}
			return
		} else {
			// Only --check: check all modes and show what would happen
			i18n.Printf("[backup-helper] Running comprehensive pre-flight checks for all modes...\n\n")

			// Check system resources (common to all modes)
			resources := utils.CheckSystemResources()
			systemResults := []utils.CheckResult{
				{
					Status:  "INFO",
					Item:    "CPU cores",
					Value:   fmt.Sprintf("%d", resources.CPUCores),
					Message: "",
				},
			}
			if resources.TotalMemory > 0 {
				systemResults = append(systemResults, utils.CheckResult{
					Status:  "INFO",
					Item:    "Total memory",
					Value:   formatBytes(resources.TotalMemory),
					Message: "",
				})
			}
			if resources.AvailableMemory > 0 {
				systemResults = append(systemResults, utils.CheckResult{
					Status:  "INFO",
					Item:    "Available memory",
					Value:   formatBytes(resources.AvailableMemory),
					Message: "",
				})
			}
			if resources.NetworkInfo != "" {
				systemResults = append(systemResults, utils.CheckResult{
					Status:  "INFO",
					Item:    "Network interfaces",
					Value:   resources.NetworkInfo,
					Message: "",
				})
			}
			utils.PrintCheckResults(i18n.Sprintf("System Resources"), systemResults)

			// Check BACKUP mode
			i18n.Printf("\n--- Checking BACKUP mode ---\n")
			backupResults := utils.CheckForBackupMode(cfg, effectiveCompressType, db)
			utils.PrintCheckResults(i18n.Sprintf("Backup Mode"), backupResults)

			backupHasError := false
			for _, result := range backupResults {
				if result.Status == "ERROR" {
					backupHasError = true
					break
				}
			}
			if backupHasError {
				i18n.Printf("[WARNING] BACKUP mode has critical errors and cannot proceed.\n")
			} else {
				// Calculate MySQL data size for recommendations
				var mysqlSize int64
				if db != nil {
					datadir, err := utils.GetDatadirFromMySQL(db)
					if err == nil {
						mysqlSize, _ = utils.CalculateBackupSize(datadir)
					}
				}
				paramResults := utils.RecommendParameters(resources, mysqlSize, effectiveCompressType, cfg)
				utils.PrintCheckResults(i18n.Sprintf("Recommended Parameters for Backup"), paramResults)
				i18n.Printf("[OK] BACKUP mode is ready.\n")
			}

			// Check DOWNLOAD mode
			i18n.Printf("\n--- Checking DOWNLOAD mode ---\n")
			downloadResults := utils.CheckForDownloadMode(cfg, effectiveCompressType, targetDir)
			utils.PrintCheckResults(i18n.Sprintf("Download Mode"), downloadResults)

			downloadHasError := false
			for _, result := range downloadResults {
				if result.Status == "ERROR" {
					downloadHasError = true
					break
				}
			}
			if downloadHasError {
				i18n.Printf("[WARNING] DOWNLOAD mode has critical errors and cannot proceed.\n")
			} else {
				i18n.Printf("[OK] DOWNLOAD mode is ready.\n")
			}

			// Check PREPARE mode
			i18n.Printf("\n--- Checking PREPARE mode ---\n")
			prepareResults := utils.CheckForPrepareMode(cfg, targetDir, db)
			utils.PrintCheckResults(i18n.Sprintf("Prepare Mode"), prepareResults)

			prepareHasError := false
			for _, result := range prepareResults {
				if result.Status == "ERROR" {
					prepareHasError = true
					break
				}
			}
			if prepareHasError {
				i18n.Printf("[WARNING] PREPARE mode has critical errors and cannot proceed.\n")
			} else {
				i18n.Printf("[OK] PREPARE mode is ready.\n")
			}

			// Summary
			i18n.Printf("\n=== %s ===\n", i18n.Sprintf("Check Summary"))
			i18n.Printf("BACKUP mode:   %s\n", map[bool]string{true: "[ERROR] Cannot proceed", false: "[OK] Ready"}[backupHasError])
			i18n.Printf("DOWNLOAD mode: %s\n", map[bool]string{true: "[ERROR] Cannot proceed", false: "[OK] Ready"}[downloadHasError])
			i18n.Printf("PREPARE mode:  %s\n", map[bool]string{true: "[ERROR] Cannot proceed", false: "[OK] Ready"}[prepareHasError])
			i18n.Printf("\nTo run a specific mode, use: --backup, --download, or --prepare\n")
			i18n.Printf("To check a specific mode only, use: --check --backup, --check --download, or --check --prepare\n")
		}
		return
	}

	// 5. Handle --prepare mode
	if doPrepare {
		// Pre-check for prepare mode
		var db *sql.DB
		if host != "" && user != "" && password != "" {
			db = utils.GetConnection(host, port, user, password)
			defer db.Close()
		} else if host != "" && user != "" {
			// Password might be prompted later, but for now we can check without it
		}

		prepareResults := utils.CheckForPrepareMode(cfg, targetDir, db)
		hasCriticalError := false
		for _, result := range prepareResults {
			if result.Status == "ERROR" {
				hasCriticalError = true
				i18n.Printf("[ERROR] %s: %s - %s\n", result.Item, result.Value, result.Message)
			}
		}
		if hasCriticalError {
			i18n.Printf("\n[ERROR] Pre-flight checks failed. Please fix the errors above before proceeding.\n")
			os.Exit(1)
		}

		if targetDir == "" {
			i18n.Printf("Error: --target-dir is required for --prepare mode\n")
			os.Exit(1)
		}

		// Check if target directory exists
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			i18n.Printf("Error: Backup directory does not exist: %s\n", targetDir)
			os.Exit(1)
		}

		// Create log context
		logCtx, err := utils.NewLogContext(cfg.LogDir)
		if err != nil {
			i18n.Printf("Failed to create log context: %v\n", err)
			os.Exit(1)
		}
		defer logCtx.Close()

		outputHeader()
		i18n.Printf("[backup-helper] Preparing backup in directory: %s\n", targetDir)
		i18n.Printf("[backup-helper] Parallel threads: %d\n", cfg.Parallel)
		i18n.Printf("[backup-helper] Use memory: %s\n", cfg.UseMemory)
		logCtx.WriteLog("PREPARE", "Starting prepare operation")
		logCtx.WriteLog("PREPARE", "Target directory: %s", targetDir)

		// Try to get MySQL connection for defaults-file (optional, can be nil)
		// db may already be set from pre-check above
		if db == nil && host != "" && user != "" {
			if password == "" {
				i18n.Printf("Please input mysql-server password (optional, for defaults-file): ")
				pwd, _ := term.ReadPassword(0)
				i18n.Printf("\n")
				password = string(pwd)
			}
			if password != "" {
				db = utils.GetConnection(host, port, user, password)
				defer db.Close()
			}
		}

		cmd, err := utils.RunXtrabackupPrepare(cfg, targetDir, db, logCtx)
		if err != nil {
			logCtx.WriteLog("PREPARE", "Failed to start prepare: %v", err)
			i18n.Printf("Failed to start prepare: %v\n", err)
			os.Exit(1)
		}

		// Wait for prepare to complete
		err = cmd.Wait()
		if err != nil {
			logCtx.WriteLog("PREPARE", "Prepare failed: %v", err)
			// Read log content for error extraction
			logContent, err2 := os.ReadFile(logCtx.GetFileName())
			if err2 == nil {
				errorSummary := utils.ExtractErrorSummary("PREPARE", string(logContent))
				if errorSummary != "" {
					i18n.Printf("Prepare failed. Error summary:\n%s\n", errorSummary)
				} else {
					i18n.Printf("Prepare failed: %v\n", err)
				}
			} else {
				i18n.Printf("Prepare failed: %v\n", err)
			}
			i18n.Printf("Log file: %s\n", logCtx.GetFileName())

			// Prompt for AI diagnosis
			switch aiDiagnoseFlag {
			case "on":
				if cfg.QwenAPIKey == "" {
					i18n.Printf("Qwen API Key is required for AI diagnosis. Please set it in config.\n")
					os.Exit(1)
				}
				logContent, _ := os.ReadFile(logCtx.GetFileName())
				aiSuggestion, err := utils.DiagnoseWithAliQwen(cfg, "PREPARE", string(logContent))
				if err != nil {
					i18n.Printf("AI diagnosis failed: %v\n", err)
				} else {
					fmt.Print(color.YellowString(i18n.Sprintf("AI diagnosis suggestion:\n")))
					fmt.Println(color.YellowString(aiSuggestion))
				}
			case "off":
				// do nothing
			default:
				if promptAIDiagnosis(autoYes) {
					logContent, _ := os.ReadFile(logCtx.GetFileName())
					aiSuggestion, err := utils.DiagnoseWithAliQwen(cfg, "PREPARE", string(logContent))
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

		logCtx.WriteLog("PREPARE", "Prepare completed successfully")
		i18n.Printf("[backup-helper] Prepare completed successfully!\n")
		i18n.Printf("[backup-helper] Backup is ready for restore in: %s\n", targetDir)
		i18n.Printf("[backup-helper] Log file: %s\n", logCtx.GetFileName())
		return
	}

	// 5. Handle --download mode
	if doDownload {
		// Pre-check for download mode
		downloadCompressType := compressType
		if downloadCompressType == "__NOT_SET__" {
			downloadCompressType = cfg.CompressType
		}
		if downloadCompressType == "no" {
			downloadCompressType = ""
		}

		downloadResults := utils.CheckForDownloadMode(cfg, downloadCompressType, targetDir)
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
		logCtx, err := utils.NewLogContext(cfg.LogDir)
		if err != nil {
			i18n.Printf("Failed to create log context: %v\n", err)
			os.Exit(1)
		}
		defer logCtx.Close()

		// Display header (only if not outputting to stdout)
		if downloadOutput != "-" {
			outputHeader()
		} else {
			// When outputting to stdout, output header to stderr
			outputHeaderToStderr()
		}
		logCtx.WriteLog("DOWNLOAD", "Starting download mode")

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

		// downloadCompressType is already set in pre-check above

		// Determine output file path
		outputPath := downloadOutput
		if outputPath == "" && cfg.DownloadOutput != "" {
			outputPath = cfg.DownloadOutput
		}
		if outputPath == "" && targetDir == "" {
			// Default: backup_YYYYMMDDHHMMSS.xb (only if not extracting)
			timestamp := time.Now().Format("20060102150405")
			outputPath = fmt.Sprintf("backup_%s.xb", timestamp)
		}

		// Display IO limit
		if outputPath == "-" {
			// Output to stderr when streaming to stdout
			if cfg.IOLimit == -1 {
				i18n.Fprintf(os.Stderr, "[backup-helper] Rate limiting disabled (unlimited speed)\n")
			} else if cfg.IOLimit > 0 {
				i18n.Fprintf(os.Stderr, "[backup-helper] IO rate limit set to: %s/s\n", formatBytes(cfg.IOLimit))
			} else {
				i18n.Fprintf(os.Stderr, "[backup-helper] IO rate limit set to: %s/s (default)\n", formatBytes(cfg.GetRateLimit()))
			}
		} else {
			// Output to stdout when saving to file
			if cfg.IOLimit == -1 {
				i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
			} else if cfg.IOLimit > 0 {
				i18n.Printf("[backup-helper] IO rate limit set to: %s/s\n", formatBytes(cfg.IOLimit))
			} else {
				i18n.Printf("[backup-helper] IO rate limit set to: %s/s (default)\n", formatBytes(cfg.GetRateLimit()))
			}
		}

		// Start TCP receiver
		isCompressed := downloadCompressType != ""
		logCtx.WriteLog("DOWNLOAD", "Starting TCP receiver on port %d", streamPort)
		receiver, tracker, closer, actualPort, localIP, err := utils.StartStreamReceiver(streamPort, enableHandshake, streamKey, estimatedSize, isCompressed, logCtx)
		_ = actualPort // Port info already displayed in StartStreamReceiver
		_ = localIP    // IP info already displayed in StartStreamReceiver
		if err != nil {
			logCtx.WriteLog("DOWNLOAD", "Stream receiver error: %v", err)
			i18n.Fprintf(os.Stderr, "Stream receiver error: %v\n", err)
			os.Exit(1)
		}
		defer closer() // This will call tracker.Complete() internally

		// Apply rate limiting if configured
		var reader io.Reader = receiver
		rateLimit := cfg.GetRateLimit()
		if rateLimit > 0 {
			rateLimitedReader := utils.NewRateLimitedReader(receiver, rateLimit)
			reader = rateLimitedReader
		}

		// Determine output destination and handle extraction
		if targetDir != "" {
			// Extraction mode: decompress (if needed) and extract
			if outputPath == "-" {
				i18n.Printf("Error: --target-dir cannot be used with --output -\n")
				os.Exit(1)
			}

			// Check if target directory exists and is not empty
			if info, err := os.Stat(targetDir); err == nil {
				if info.IsDir() {
					empty, err := isDirEmpty(targetDir)
					if err != nil {
						logCtx.WriteLog("DOWNLOAD", "Failed to check target directory: %v", err)
						i18n.Printf("Error: Failed to check target directory: %v\n", err)
						os.Exit(1)
					}
					if !empty {
						// Directory exists and is not empty, ask user for confirmation
						if !promptOverwrite(targetDir, autoYes) {
							logCtx.WriteLog("DOWNLOAD", "User cancelled extraction to non-empty directory: %s", targetDir)
							i18n.Printf("Extraction cancelled.\n")
							os.Exit(0)
						}
						logCtx.WriteLog("DOWNLOAD", "User confirmed overwrite for directory: %s", targetDir)
						i18n.Printf("Clearing target directory...\n")
						logCtx.WriteLog("DOWNLOAD", "Clearing target directory: %s", targetDir)
						if err := clearDirectory(targetDir); err != nil {
							logCtx.WriteLog("DOWNLOAD", "Failed to clear target directory: %v", err)
							i18n.Printf("Error: Failed to clear target directory: %v\n", err)
							os.Exit(1)
						}
						logCtx.WriteLog("DOWNLOAD", "Target directory cleared successfully")
						i18n.Printf("Target directory cleared. Proceeding with extraction...\n")
					}
				} else {
					logCtx.WriteLog("DOWNLOAD", "Target path exists but is not a directory: %s", targetDir)
					i18n.Printf("Error: Target path '%s' exists but is not a directory\n", targetDir)
					os.Exit(1)
				}
			}
			// If directory doesn't exist, it will be created by MkdirAll in extract functions

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
			i18n.Printf("[backup-helper] Extracting to directory: %s\n", targetDir)
			logCtx.WriteLog("DOWNLOAD", "Extracting to directory: %s", targetDir)

			err := utils.ExtractBackupStream(reader, downloadCompressType, targetDir, outputPath, cfg.Parallel, cfg, logCtx)
			if err != nil {
				logCtx.WriteLog("EXTRACT", "Extraction error: %v", err)
				// Read log content for error extraction
				logContent, err2 := os.ReadFile(logCtx.GetFileName())
				if err2 == nil {
					errorSummary := utils.ExtractErrorSummary("EXTRACT", string(logContent))
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
				switch aiDiagnoseFlag {
				case "on":
					if cfg.QwenAPIKey == "" {
						i18n.Printf("Qwen API Key is required for AI diagnosis. Please set it in config.\n")
						os.Exit(1)
					}
					logContent, _ := os.ReadFile(logCtx.GetFileName())
					aiSuggestion, err := utils.DiagnoseWithAliQwen(cfg, "EXTRACT", string(logContent))
					if err != nil {
						i18n.Printf("AI diagnosis failed: %v\n", err)
					} else {
						fmt.Print(color.YellowString(i18n.Sprintf("AI diagnosis suggestion:\n")))
						fmt.Println(color.YellowString(aiSuggestion))
					}
				case "off":
					// do nothing
				default:
					if promptAIDiagnosis(autoYes) {
						logContent, _ := os.ReadFile(logCtx.GetFileName())
						aiSuggestion, err := utils.DiagnoseWithAliQwen(cfg, "EXTRACT", string(logContent))
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
			i18n.Printf("[backup-helper] Extraction completed to: %s\n", targetDir)
			logCtx.WriteLog("DOWNLOAD", "Extraction completed successfully")
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
				decompressedReader, decompressCmd, err := utils.ExtractBackupStreamToStdout(reader, downloadCompressType, cfg.Parallel, logCtx)
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
				if contains(errStr, "connection closed unexpectedly") || contains(errStr, "EOF") || contains(errStr, "broken pipe") {
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
				err := utils.ExtractBackupStream(reader, downloadCompressType, "", outputPath, cfg.Parallel, cfg, logCtx)
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
			i18n.Printf("[backup-helper] Log file: %s\n", logCtx.GetFileName())
		}
		return
	}

	// 6. If --backup, run backup/upload
	if doBackup {
		// Pre-check for backup mode
		effectiveCompressType := compressType
		if effectiveCompressType == "__NOT_SET__" {
			effectiveCompressType = cfg.CompressType
		}
		if effectiveCompressType == "no" {
			effectiveCompressType = ""
		}

		// MySQL param check (only needed for backup)
		if password == "" {
			i18n.Printf("Please input mysql-server password: ")
			pwd, _ := term.ReadPassword(0)
			i18n.Printf("\n")
			password = string(pwd)
		}

		// Get MySQL connection for pre-check
		var db *sql.DB
		if host != "" && user != "" && password != "" {
			db = utils.GetConnection(host, port, user, password)
			defer db.Close()
		}

		// Run pre-flight checks
		backupResults := utils.CheckForBackupMode(cfg, effectiveCompressType, db)
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

		i18n.Printf("connect to mysql-server host=%s port=%d user=%s\n", host, port, user)
		outputHeader()
		// db may already be set from pre-check above
		if db == nil {
			db = utils.GetConnection(host, port, user, password)
			defer db.Close()
		}
		options := utils.CollectVariableFromMySQLServer(db)
		utils.Check(options, cfg)

		// Display IO limit after parameter check
		if cfg.IOLimit == -1 {
			i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
		} else if cfg.IOLimit > 0 {
			i18n.Printf("[backup-helper] IO rate limit set to: %s/s\n", formatBytes(cfg.IOLimit))
		} else {
			// Using default rate limit
			i18n.Printf("[backup-helper] IO rate limit set to: %s/s (default)\n", formatBytes(cfg.GetRateLimit()))
		}

		// Check compression dependencies early (before starting backup)
		// effectiveCompressType is already set in pre-check above
		if effectiveCompressType == "" && cfg.CompressType != "" {
			effectiveCompressType = cfg.CompressType
		}
		// Normalize: "no" or empty string means no compression
		if effectiveCompressType == "no" {
			effectiveCompressType = ""
		}
		if effectiveCompressType != "" {
			if err := utils.CheckCompressionDependencies(effectiveCompressType, true, cfg); err != nil {
				i18n.Printf("Error: %v\n", err)
				os.Exit(1)
			}
		}

		// Check xtrabackup version (run early)
		mysqlVer := cfg.MysqlVersion
		utils.CheckXtraBackupVersion(mysqlVer, cfg)

		// Create log context
		logCtx, err := utils.NewLogContext(cfg.LogDir)
		if err != nil {
			i18n.Printf("Failed to create log context: %v\n", err)
			os.Exit(1)
		}
		defer logCtx.Close()

		i18n.Printf("[backup-helper] Running xtrabackup...\n")
		cfg.MysqlHost = host
		cfg.MysqlPort = port
		cfg.MysqlUser = user
		cfg.MysqlPassword = password
		logCtx.WriteLog("BACKUP", "Starting backup operation")
		logCtx.WriteLog("BACKUP", "MySQL host: %s, port: %d, user: %s", host, port, user)

		// 1. Decide objectName suffix and compression param
		ossObjectName := cfg.ObjectName
		objectSuffix := ".xb"
		// Determine effective compression type (reuse variable from above)
		if effectiveCompressType == "" && cfg.CompressType != "" {
			effectiveCompressType = cfg.CompressType
		}
		// Normalize: "no" or empty string means no compression
		if effectiveCompressType == "no" {
			effectiveCompressType = ""
		}

		// Set cfg.CompressType based on effectiveCompressType
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

		reader, cmd, err := utils.RunXtraBackup(cfg, db, logCtx)
		if err != nil {
			logCtx.WriteLog("BACKUP", "Failed to start xtrabackup: %v", err)
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
			logCtx.WriteLog("OSS", "Starting OSS upload")
			isCompressed := cfg.CompressType != ""
			err = utils.UploadReaderToOSS(cfg, fullObjectName, reader, totalSize, isCompressed, logCtx)
			if err != nil {
				logCtx.WriteLog("OSS", "OSS upload failed: %v", err)
				i18n.Printf("OSS upload error: %v\n", err)
				cmd.Process.Kill()
				os.Exit(1)
			}
			logCtx.WriteLog("OSS", "OSS upload completed successfully")
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
					logCtx.WriteLog("SSH", "Starting remote receiver via SSH")
					logCtx.WriteLog("SSH", "Remote host: %s", streamHost)
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
					isCompressed := cfg.CompressType != ""
					writer, _, closer, _, err = utils.StartStreamClient(
						streamHost, streamPort, enableHandshake, streamKey, totalSize, isCompressed, logCtx)
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

					isCompressed := cfg.CompressType != ""
					writer, _, closer, _, err = utils.StartStreamClient(
						streamHost, streamPort, enableHandshake, streamKey, totalSize, isCompressed, logCtx)
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

				tcpWriter, _, closerFunc, actualPort, localIP, err := utils.StartStreamSender(streamPort, enableHandshake, streamKey, totalSize, cfg.CompressType != "", logCtx)
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
			rateLimit := cfg.GetRateLimit()
			if rateLimit > 0 {
				rateLimitedWriter := utils.NewRateLimitedWriter(writer, rateLimit)
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
			errorSummary := utils.ExtractErrorSummary("BACKUP", string(logContent))
			if errorSummary != "" {
				i18n.Printf("Backup failed. Error summary:\n%s\n", errorSummary)
			} else {
				i18n.Printf("Backup failed (no 'completed OK!').\n")
			}
			i18n.Printf("Log file: %s\n", logCtx.GetFileName())

			switch aiDiagnoseFlag {
			case "on":
				if cfg.QwenAPIKey == "" {
					i18n.Printf("Qwen API Key is required for AI diagnosis. Please set it in config.\n")
					os.Exit(1)
				}
				aiSuggestion, err := utils.DiagnoseWithAliQwen(cfg, "BACKUP", string(logContent))
				if err != nil {
					i18n.Printf("AI diagnosis failed: %v\n", err)
				} else {
					fmt.Print(color.YellowString(i18n.Sprintf("AI diagnosis suggestion:\n")))
					fmt.Println(color.YellowString(aiSuggestion))
				}
			case "off":
				// do nothing, skip ai diagnose
			default:
				if promptAIDiagnosis(autoYes) {
					aiSuggestion, err := utils.DiagnoseWithAliQwen(cfg, "BACKUP", string(logContent))
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
		logCtx.WriteLog("BACKUP", "Backup completed successfully")
		i18n.Printf("[backup-helper] Backup and upload completed!\n")
		i18n.Printf("[backup-helper] Log file: %s\n", logCtx.GetFileName())
		return
	} else if existedBackup != "" {
		// Create log context for existed backup
		logCtx, err := utils.NewLogContext(cfg.LogDir)
		if err != nil {
			i18n.Printf("Failed to create log context: %v\n", err)
			os.Exit(1)
		}
		defer logCtx.Close()

		// upload existed backup file to OSS or stream via TCP
		logCtx.WriteLog("BACKUP", "Processing existing backup file")
		i18n.Printf("[backup-helper] Processing existing backup file...\n")

		// Validate backup file before processing
		var backupInfo *utils.BackupFileInfo
		var err2 error

		if existedBackup == "-" {
			// Validate data from stdin
			backupInfo, err2 = utils.ValidateBackupFileFromStdin()
			if err2 != nil {
				i18n.Printf("Validation error: %v\n", err2)
				os.Exit(1)
			}
			utils.PrintBackupFileValidationFromStdin(backupInfo)
		} else {
			// Validate file
			backupInfo, err2 = utils.ValidateBackupFile(existedBackup)
			if err2 != nil {
				i18n.Printf("Validation error: %v\n", err2)
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
		if cfg.IOLimit == -1 {
			i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
		} else if cfg.IOLimit > 0 {
			i18n.Printf("[backup-helper] IO rate limit set to: %s/s\n", formatBytes(cfg.IOLimit))
		} else {
			// Using default rate limit
			i18n.Printf("[backup-helper] IO rate limit set to: %s/s (default)\n", formatBytes(cfg.GetRateLimit()))
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
		// Determine effective compression type
		effectiveCompressType := compressType
		if effectiveCompressType == "" && cfg.CompressType != "" {
			effectiveCompressType = cfg.CompressType
		}
		// Normalize: "no" or empty string means no compression
		if effectiveCompressType == "no" {
			effectiveCompressType = ""
		}

		// Set cfg.CompressType based on effectiveCompressType
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
			isCompressed := cfg.CompressType != ""
			err := utils.UploadReaderToOSS(cfg, fullObjectName, reader, totalSize, isCompressed, logCtx)
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
				logCtx.WriteLog("TCP", "Active push mode: connecting to %s:%d", streamHost, streamPort)
				isCompressed := cfg.CompressType != ""
				writer, _, closer, _, err = utils.StartStreamClient(streamHost, streamPort, enableHandshake, streamKey, totalSize, isCompressed, logCtx)
				if err != nil {
					i18n.Printf("Stream client error: %v\n", err)
					os.Exit(1)
				}
			} else {
				// Passive connection: listen locally and wait for connection
				// streamPort can be 0 now (auto-find available port)
				tcpWriter, _, closerFunc, actualPort, localIP, err := utils.StartStreamSender(streamPort, enableHandshake, streamKey, totalSize, cfg.CompressType != "", logCtx)
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
			rateLimit := cfg.GetRateLimit()
			if rateLimit > 0 {
				rateLimitedWriter := utils.NewRateLimitedWriter(writer, rateLimit)
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

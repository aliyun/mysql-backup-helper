package cmd

import (
	"backup-helper/internal/check"
	"backup-helper/internal/config"
	"backup-helper/internal/mysql"
	"backup-helper/internal/utils"
	"database/sql"
	"fmt"
	"os"

	"github.com/gioco-play/easy-i18n/i18n"
	"golang.org/x/term"
)

// HandleCheck handles the check command
func HandleCheck(cfg *config.Config, effective *config.EffectiveValues, flags *config.Flags) error {
	utils.OutputHeader()

	// Determine effective compression type
	effectiveCompressType := effective.CompressType

	// Parse stream-host and stream-port for connectivity checks
	checkStreamHost := effective.StreamHost
	checkStreamPort := effective.StreamPort

	// Get MySQL connection if available
	var db *sql.DB
	if effective.Host != "" && effective.User != "" {
		password := effective.Password
		if password == "" {
			i18n.Printf("Please input mysql-server password (optional, for MySQL compatibility checks): ")
			pwd, _ := term.ReadPassword(0)
			i18n.Printf("\n")
			password = string(pwd)
		}
		if password != "" {
			i18n.Printf("Connecting to MySQL server for compatibility checks...\n")
			db = mysql.GetConnection(effective.Host, effective.Port, effective.User, password)
			defer db.Close()
		}
	}

	// Check if --check is combined with other modes
	if flags.DoBackup {
		// --check --backup: only check backup mode
		i18n.Printf("[backup-helper] Running pre-flight checks for BACKUP mode...\n\n")
		results := check.CheckForBackupMode(cfg, effectiveCompressType, db, checkStreamHost, checkStreamPort)
		check.PrintCheckResults(i18n.Sprintf("Backup Mode Checks"), results)

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
		return nil
	} else if flags.DoDownload {
		// --check --download: only check download mode
		i18n.Printf("[backup-helper] Running pre-flight checks for DOWNLOAD mode...\n\n")
		results := check.CheckForDownloadMode(cfg, effectiveCompressType, flags.TargetDir, checkStreamHost, checkStreamPort)
		check.PrintCheckResults(i18n.Sprintf("Download Mode Checks"), results)

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
		return nil
	} else if flags.DoPrepare {
		// --check --prepare: only check prepare mode
		i18n.Printf("[backup-helper] Running pre-flight checks for PREPARE mode...\n\n")
		results := check.CheckForPrepareMode(cfg, flags.TargetDir, db)
		check.PrintCheckResults(i18n.Sprintf("Prepare Mode Checks"), results)

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
		return nil
	} else {
		// Only --check: check all modes and show what would happen
		i18n.Printf("[backup-helper] Running comprehensive pre-flight checks for all modes...\n\n")

		// Check system resources (common to all modes)
		resources := check.CheckSystemResources()
		systemResults := []check.CheckResult{
			{
				Status:  "INFO",
				Item:    "CPU cores",
				Value:   fmt.Sprintf("%d", resources.CPUCores),
				Message: "",
			},
		}
		if resources.TotalMemory > 0 {
			systemResults = append(systemResults, check.CheckResult{
				Status:  "INFO",
				Item:    "Total memory",
				Value:   utils.FormatBytes(resources.TotalMemory),
				Message: "",
			})
		}
		if resources.AvailableMemory > 0 {
			systemResults = append(systemResults, check.CheckResult{
				Status:  "INFO",
				Item:    "Available memory",
				Value:   utils.FormatBytes(resources.AvailableMemory),
				Message: "",
			})
		}
		if resources.NetworkInfo != "" {
			systemResults = append(systemResults, check.CheckResult{
				Status:  "INFO",
				Item:    "Network interfaces",
				Value:   resources.NetworkInfo,
				Message: "",
			})
		}
		check.PrintCheckResults(i18n.Sprintf("System Resources"), systemResults)

		// Check BACKUP mode
		i18n.Printf("\n--- Checking BACKUP mode ---\n")
		backupResults := check.CheckForBackupMode(cfg, effectiveCompressType, db, "", 0)
		check.PrintCheckResults(i18n.Sprintf("Backup Mode"), backupResults)

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
				datadir, err := mysql.GetDatadirFromMySQL(db)
				if err == nil {
					mysqlSize, _ = mysql.CalculateBackupSize(datadir)
				}
			}
			paramResults := check.RecommendParameters(resources, mysqlSize, effectiveCompressType, cfg)
			check.PrintCheckResults(i18n.Sprintf("Recommended Parameters for Backup"), paramResults)
			i18n.Printf("[OK] BACKUP mode is ready.\n")
		}

		// Check DOWNLOAD mode
		i18n.Printf("\n--- Checking DOWNLOAD mode ---\n")
		downloadResults := check.CheckForDownloadMode(cfg, effectiveCompressType, flags.TargetDir, "", 0)
		check.PrintCheckResults(i18n.Sprintf("Download Mode"), downloadResults)

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
		prepareResults := check.CheckForPrepareMode(cfg, flags.TargetDir, db)
		check.PrintCheckResults(i18n.Sprintf("Prepare Mode"), prepareResults)

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
	return nil
}

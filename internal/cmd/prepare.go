package cmd

import (
	"backup-helper/internal/ai"
	"backup-helper/internal/backup"
	"backup-helper/internal/check"
	"backup-helper/internal/config"
	"backup-helper/internal/log"
	"backup-helper/internal/mysql"
	"backup-helper/internal/utils"
	"database/sql"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/gioco-play/easy-i18n/i18n"
	"golang.org/x/term"
)

// HandlePrepare handles the prepare command
func HandlePrepare(cfg *config.Config, effective *config.EffectiveValues, flags *config.Flags) error {
	// Pre-check for prepare mode
	var db *sql.DB
	if effective.Host != "" && effective.User != "" && effective.Password != "" {
		db = mysql.GetConnection(effective.Host, effective.Port, effective.User, effective.Password)
		defer db.Close()
	} else if effective.Host != "" && effective.User != "" {
		// Password might be prompted later, but for now we can check without it
	}

	prepareResults := check.CheckForPrepareMode(cfg, flags.TargetDir, db)
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

	if flags.TargetDir == "" {
		i18n.Printf("Error: --target-dir is required for --prepare mode\n")
		os.Exit(1)
	}

	// Check if target directory exists
	if _, err := os.Stat(flags.TargetDir); os.IsNotExist(err) {
		i18n.Printf("Error: Backup directory does not exist: %s\n", flags.TargetDir)
		os.Exit(1)
	}

	// Create log context
	logCtx, err := log.NewLogContext(cfg.LogDir, cfg.LogFileName)
	if err != nil {
		i18n.Printf("Failed to create log context: %v\n", err)
		os.Exit(1)
	}
	defer logCtx.Close()

	utils.OutputHeader()
	i18n.Printf("[backup-helper] Preparing backup in directory: %s\n", flags.TargetDir)
	i18n.Printf("[backup-helper] Parallel threads: %d\n", cfg.Parallel)
	i18n.Printf("[backup-helper] Use memory: %s\n", cfg.UseMemory)
	logCtx.WriteLog("PREPARE", "Starting prepare operation")
	logCtx.WriteLog("PREPARE", "Target directory: %s", flags.TargetDir)

	// Try to get MySQL connection for defaults-file (optional, can be nil)
	if db == nil && effective.Host != "" && effective.User != "" {
		password := effective.Password
		if password == "" {
			i18n.Printf("Please input mysql-server password (optional, for defaults-file): ")
			pwd, _ := term.ReadPassword(0)
			i18n.Printf("\n")
			password = string(pwd)
		}
		if password != "" {
			db = mysql.GetConnection(effective.Host, effective.Port, effective.User, password)
			defer db.Close()
		}
	}

	cmd, err := backup.RunXtrabackupPrepare(cfg, flags.TargetDir, db, logCtx)
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
			errorSummary := log.ExtractErrorSummary("PREPARE", string(logContent))
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
		switch flags.AIDiagnoseFlag {
		case "on":
			// When --ai-diagnose=on, ask user (unless -y is set)
			if utils.PromptAIDiagnosis(flags.AutoYes) {
				if cfg.QwenAPIKey == "" {
					i18n.Printf("Qwen API Key is required for AI diagnosis. Please set it in config.\n")
					os.Exit(1)
				}
				logContent, _ := os.ReadFile(logCtx.GetFileName())
				aiSuggestion, err := ai.DiagnoseWithAliQwen(cfg, "PREPARE", string(logContent))
				if err != nil {
					i18n.Printf("AI diagnosis failed: %v\n", err)
				} else {
					fmt.Print(color.YellowString(i18n.Sprintf("AI diagnosis suggestion:\n")))
					fmt.Println(color.YellowString(aiSuggestion))
				}
			}
		case "off":
			// do nothing, skip ai diagnose
		default:
			// Default: off (skip AI diagnosis to avoid interrupting user workflow)
			// do nothing
		}
		os.Exit(1)
	}

	logCtx.WriteLog("PREPARE", "Prepare completed successfully")
	logCtx.MarkSuccess()
	i18n.Printf("[backup-helper] Prepare completed successfully!\n")
	i18n.Printf("[backup-helper] Backup is ready for restore in: %s\n", flags.TargetDir)
	i18n.Printf("[backup-helper] Log file: %s\n", logCtx.GetFileName())
	return nil
}

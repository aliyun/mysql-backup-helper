package main

import (
	"backup-helper/internal/cmd"
	"backup-helper/internal/config"
	"backup-helper/internal/utils"
	"os"

	i18nlib "github.com/gioco-play/easy-i18n/i18n"
	"golang.org/x/text/language"
)

func main() {
	// Initialize i18n early
	utils.InitI18nAuto()

	// Parse command line flags
	flags := ParseFlags()

	// Check version parameter
	if flags.ShowVersion {
		utils.PrintVersion()
		os.Exit(0)
	}

	// Set language if --lang is specified
	switch flags.LangFlag {
	case "cn", "zh":
		i18nlib.SetLang(language.SimplifiedChinese)
	case "en":
		i18nlib.SetLang(language.English)
	default:
		// use auto setting
	}

	// Load and merge configuration
	cfg, effective, err := config.LoadAndMergeConfig(flags)
	if err != nil {
		i18nlib.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Route to appropriate command handler
	if flags.DoCheck {
		cmd.HandleCheck(cfg, effective, flags)
		return
	}

	if flags.DoPrepare {
		if err := cmd.HandlePrepare(cfg, effective, flags); err != nil {
			os.Exit(1)
		}
		return
	}

	if flags.DoDownload {
		if err := cmd.HandleDownload(cfg, effective, flags); err != nil {
			os.Exit(1)
		}
		return
	}

	if flags.DoBackup {
		if err := cmd.HandleBackup(cfg, effective, flags); err != nil {
			os.Exit(1)
		}
		return
	}

	// Handle existed-backup mode (upload/stream existing backup file)
	if effective.ExistedBackup != "" {
		if err := cmd.HandleExistedBackup(cfg, effective, flags); err != nil {
			os.Exit(1)
		}
		return
	}

	// If no command specified, just exit
	i18nlib.Printf("No command specified. Use --backup, --download, --prepare, or --check\n")
	os.Exit(0)
}

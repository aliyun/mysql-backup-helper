package cmd

import (
	"backup-helper/internal/config"
	"backup-helper/internal/pkg/i18n"
	"fmt"
	"os"

	i18nlib "github.com/gioco-play/easy-i18n/i18n"
	"github.com/spf13/cobra"
	"golang.org/x/text/language"
)

var (
	// Global flags
	cfgFile  string
	langFlag string
	verbose  bool
	quiet    bool

	// Config object shared across commands
	cfg *config.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mysql-backup-helper",
	Short: "MySQL Backup Helper - A tool for MySQL backup and transfer",
	Long: `MySQL Backup Helper is a comprehensive tool for MySQL database backup,
upload to OSS, and TCP stream transfer.

Examples:
  # Backup MySQL and upload to OSS
  mysql-backup-helper backup --host 127.0.0.1 --user root --to-oss

  # Send existing backup file via TCP stream
  mysql-backup-helper send --file backup.xb --to-stream 9000

  # Receive backup from TCP stream
  mysql-backup-helper receive --from-stream 9000 --output backup.xb

  # Show version
  mysql-backup-helper version`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize i18n
		i18n.InitAuto()

		// Set language if --lang is specified
		switch langFlag {
		case "cn", "zh":
			i18nlib.SetLang(language.SimplifiedChinese)
		case "en":
			i18nlib.SetLang(language.English)
		default:
			// use auto setting
		}

		// Load config
		if cfgFile != "" {
			var err error
			cfg, err = config.LoadConfig(cfgFile)
			if err != nil {
				return fmt.Errorf("load config error: %v", err)
			}
			cfg.SetDefaults()
		} else {
			cfg = &config.Config{}
			cfg.SetDefaults()
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path (optional)")
	rootCmd.PersistentFlags().StringVar(&langFlag, "lang", "", "language: zh (Chinese) or en (English), auto-detect if unset")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet mode (minimal output)")

}

// GetConfig returns the global config object
func GetConfig() *config.Config {
	return cfg
}

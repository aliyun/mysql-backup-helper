package cmd

import (
	"backup-helper/pkg/version"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the version and build information of mysql-backup-helper.`,
	Run: func(cmd *cobra.Command, args []string) {
		version.Print()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

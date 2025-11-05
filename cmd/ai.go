package cmd

import (
	"backup-helper/internal/infra/ai"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/gioco-play/easy-i18n/i18n"
	"github.com/spf13/cobra"
)

var (
	// AI command flags
	aiLogFile  string
	aiQuestion string
)

// aiCmd represents the ai command
var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "AI diagnosis and Q&A for MySQL backup issues",
	Long: `Use AI to diagnose backup log files or ask questions about MySQL backup issues.

Examples:
  # Diagnose a backup log file
  mysql-backup-helper ai --log-file /var/log/mysql-backup-helper/backup.log

  # Ask a question
  mysql-backup-helper ai --question "How to fix Access denied error?"`,
	RunE: runAI,
}

func init() {
	rootCmd.AddCommand(aiCmd)

	// AI command flags
	aiCmd.Flags().StringVarP(&aiLogFile, "log-file", "f", "", "Path to backup log file for diagnosis")
	aiCmd.Flags().StringVar(&aiQuestion, "question", "", "Ask a question about MySQL backup")
}

func runAI(cmd *cobra.Command, args []string) error {
	cfg := GetConfig()

	// Check API key
	if cfg.QwenAPIKey == "" {
		return fmt.Errorf("qwen api key required, please set 'qwenAPIKey' in config file")
	}

	// Check that exactly one option is provided
	if aiLogFile == "" && aiQuestion == "" {
		return fmt.Errorf("please specify either --log-file or --question")
	}
	if aiLogFile != "" && aiQuestion != "" {
		return fmt.Errorf("please specify only one of --log-file or --question")
	}

	client := ai.NewQwenClient(cfg.QwenAPIKey)

	// Diagnose log file or answer question
	if aiLogFile != "" {
		return diagnoseLogFile(client, aiLogFile)
	} else {
		return answerQuestion(client, aiQuestion)
	}
}

// diagnoseLogFile diagnoses a backup log file
func diagnoseLogFile(client *ai.QwenClient, logPath string) error {
	logVerbose("Reading log file: %s\n", logPath)

	content, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Errorf("failed to read log file: %v", err)
	}

	if len(content) == 0 {
		return fmt.Errorf("log file is empty")
	}

	logInfo("Analyzing log with AI...\n\n")

	suggestion, err := client.Diagnose(string(content))
	if err != nil {
		return fmt.Errorf("ai diagnosis failed: %v", err)
	}

	fmt.Print(color.YellowString(i18n.Sprintf("AI diagnosis suggestion:\n")))
	fmt.Println(color.YellowString(suggestion))

	return nil
}

// answerQuestion answers a question about MySQL backup
func answerQuestion(client *ai.QwenClient, question string) error {
	logVerbose("Question: %s\n\n", question)

	// Prepare context for MySQL backup questions
	contextualQuestion := fmt.Sprintf("As a MySQL backup expert using xtrabackup, please answer this question: %s", question)

	answer, err := client.Diagnose(contextualQuestion)
	if err != nil {
		return fmt.Errorf("ai query failed: %v", err)
	}

	fmt.Print(color.GreenString("AI Answer:\n"))
	fmt.Println(answer)

	return nil
}

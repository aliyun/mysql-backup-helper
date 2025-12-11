package ai

import (
	"backup-helper/internal/config"
	"context"
	"errors"

	"github.com/gioco-play/easy-i18n/i18n"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// DiagnoseWithAliQwen call qwen-max-latest model to diagnose the log content
// module: module type (BACKUP, PREPARE, TCP, OSS, DECOMPRESS, EXTRACT, XBSTREAM)
// logContent: log content to diagnose
func DiagnoseWithAliQwen(cfg *config.Config, module string, logContent string) (string, error) {
	if cfg.QwenAPIKey == "" {
		return "", errors.New("DashScope API Key is not set in config")
	}
	client := openai.NewClient(
		option.WithAPIKey(cfg.QwenAPIKey),
		option.WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1/"),
	)

	// Get module-specific prompt
	prompt := getDiagnosisPrompt(module)

	chatCompletion, err := client.Chat.Completions.New(
		context.TODO(), openai.ChatCompletionNewParams{
			Messages: openai.F(
				[]openai.ChatCompletionMessageParamUnion{
					openai.SystemMessage(prompt),
					openai.UserMessage(logContent),
				},
			),
			Model: openai.F("qwen-max-latest"),
		},
	)
	if err != nil {
		return "", err
	}
	return chatCompletion.Choices[0].Message.Content, nil
}

// getDiagnosisPrompt returns module-specific diagnosis prompt
func getDiagnosisPrompt(module string) string {
	switch module {
	case "BACKUP":
		return i18n.Sprintf("AI_DIAG_PROMPT_BACKUP")
	case "PREPARE":
		return i18n.Sprintf("AI_DIAG_PROMPT_PREPARE")
	case "TCP":
		return i18n.Sprintf("AI_DIAG_PROMPT_TCP")
	case "OSS":
		return i18n.Sprintf("AI_DIAG_PROMPT_OSS")
	case "DECOMPRESS", "EXTRACT", "XBSTREAM":
		return i18n.Sprintf("AI_DIAG_PROMPT_EXTRACT")
	default:
		return i18n.Sprintf("AI_DIAG_PROMPT")
	}
}

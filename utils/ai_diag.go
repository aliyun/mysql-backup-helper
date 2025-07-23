package utils

import (
	"context"
	"errors"

	"github.com/gioco-play/easy-i18n/i18n"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// DiagnoseWithAliQwen call qwen-max-latest model to diagnose the log content
func DiagnoseWithAliQwen(cfg *Config, logContent string) (string, error) {
	if cfg.QwenAPIKey == "" {
		return "", errors.New("DashScope API Key is not set in config")
	}
	client := openai.NewClient(
		option.WithAPIKey(cfg.QwenAPIKey),
		option.WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1/"),
	)
	chatCompletion, err := client.Chat.Completions.New(
		context.TODO(), openai.ChatCompletionNewParams{
			Messages: openai.F(
				[]openai.ChatCompletionMessageParamUnion{
					openai.SystemMessage(i18n.Sprintf("AI_DIAG_PROMPT")),
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

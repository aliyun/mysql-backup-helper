package ai

import (
	"context"
	"errors"

	"github.com/gioco-play/easy-i18n/i18n"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// QwenClient handles AI diagnosis using Alibaba Cloud Qwen
type QwenClient struct {
	apiKey string
}

// NewQwenClient creates a new Qwen AI client
func NewQwenClient(apiKey string) *QwenClient {
	return &QwenClient{apiKey: apiKey}
}

// Diagnose analyzes log content and provides diagnosis suggestions
func (c *QwenClient) Diagnose(logContent string) (string, error) {
	if c.apiKey == "" {
		return "", errors.New("DashScope API Key is not set")
	}

	client := openai.NewClient(
		option.WithAPIKey(c.apiKey),
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

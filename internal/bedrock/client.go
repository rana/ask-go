package bedrock

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/rana/ask/internal/config"
)

// SendToClaude sends content to Claude via AWS Bedrock Converse API
func SendToClaude(content string) (string, error) {
	// Load Ask configuration
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve model ID
	modelID, err := cfg.ResolveModel()
	if err != nil {
		return "", fmt.Errorf("failed to resolve model: %w", err)
	}

	// Parse timeout
	timeout, err := cfg.ParseTimeout()
	if err != nil {
		return "", fmt.Errorf("failed to parse timeout: %w", err)
	}

	// Load AWS configuration
	awsCfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", fmt.Errorf("AWS credentials not configured. Run: aws configure")
	}

	// Create Bedrock client
	client := bedrockruntime.NewFromConfig(awsCfg)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Build the message
	userMessage := types.Message{
		Role: types.ConversationRoleUser,
		Content: []types.ContentBlock{
			&types.ContentBlockMemberText{
				Value: content,
			},
		},
	}

	// Build inference configuration
	inferenceConfig := &types.InferenceConfiguration{
		Temperature: aws.Float32(float32(cfg.Temperature)),
		MaxTokens:   aws.Int32(int32(cfg.MaxTokens)),
	}

	// Build the request
	input := &bedrockruntime.ConverseInput{
		ModelId:         aws.String(modelID),
		Messages:        []types.Message{userMessage},
		InferenceConfig: inferenceConfig,
	}

	// Build additional model fields if needed
	additionalFields := make(map[string]interface{})

	// Add thinking configuration if enabled
	if cfg.Thinking.Enabled {
		thinkingTokens := cfg.GetThinkingTokens()
		additionalFields["max_thinking_tokens"] = thinkingTokens
	}

	// Add any additional Bedrock parameters from config
	for key, value := range cfg.Bedrock {
		additionalFields[key] = value
	}

	// Only set AdditionalModelRequestFields if we have fields to add
	if len(additionalFields) > 0 {
		docMarshaler := document.NewLazyDocument(additionalFields)
		input.AdditionalModelRequestFields = docMarshaler
	}

	// Send to Bedrock
	result, err := client.Converse(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to invoke Claude: %w", err)
	}

	// Extract response
	if result.Output == nil {
		return "", fmt.Errorf("empty response from Claude")
	}

	switch v := result.Output.(type) {
	case *types.ConverseOutputMemberMessage:
		if len(v.Value.Content) > 0 {
			if textBlock, ok := v.Value.Content[0].(*types.ContentBlockMemberText); ok {
				return textBlock.Value, nil
			}
		}
	}

	return "", fmt.Errorf("unexpected response format from Claude")
}

// CountTokens returns the token count from a Converse response
func CountTokens(result *bedrockruntime.ConverseOutput) (input int, output int) {
	if result.Usage != nil {
		if result.Usage.InputTokens != nil {
			input = int(*result.Usage.InputTokens)
		}
		if result.Usage.OutputTokens != nil {
			output = int(*result.Usage.OutputTokens)
		}
	}
	return input, output
}

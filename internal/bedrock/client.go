package bedrock

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/rana/ask/internal/config"
	"github.com/rana/ask/internal/session"
)

// SendToClaude sends content to Claude via AWS Bedrock Converse API
func SendToClaude(content string) (string, error) {
	// Build a single message for backwards compatibility
	messages := []session.Turn{
		{Number: 1, Role: "Human", Content: content},
	}
	return SendToClaudeWithHistory(messages)
}

// SendToClaudeWithHistory sends a full conversation history to Claude
func SendToClaudeWithHistory(turns []session.Turn) (string, error) {
	return sendToClaudeWithRetry(turns, false)
}

// sendToClaudeWithRetry handles the actual sending with retry logic for stale profiles
func sendToClaudeWithRetry(turns []session.Turn, isRetry bool) (string, error) {
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

	// If this is a retry, invalidate the cache first
	if isRetry {
		profileName := deriveProfileName(modelID)
		invalidateCachedProfile(profileName)
	}

	// Ensure profile exists and get capabilities
	profileArn, capabilities, err := ensureProfile(modelID)
	if err != nil {
		return "", fmt.Errorf("failed to setup model: %w", err)
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

	// Build message array from turns
	var messages []types.Message
	for _, turn := range turns {
		var role types.ConversationRole
		if turn.Role == "Human" {
			role = types.ConversationRoleUser
		} else {
			role = types.ConversationRoleAssistant
		}

		messages = append(messages, types.Message{
			Role: role,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{
					Value: turn.Content,
				},
			},
		})
	}

	// Build standard inference configuration
	inferenceConfig := &types.InferenceConfiguration{
		Temperature: aws.Float32(float32(cfg.Temperature)),
		MaxTokens:   aws.Int32(int32(cfg.MaxTokens)),
	}

	// Build the request
	input := &bedrockruntime.ConverseInput{
		ModelId:         aws.String(profileArn),
		Messages:        messages,
		InferenceConfig: inferenceConfig,
	}

	// Add additional fields for custom profiles
	if !capabilities.UseSystemProfile {
		additionalFields := make(map[string]interface{})

		// Add thinking configuration if enabled and supported
		if cfg.Thinking.Enabled && capabilities.SupportsThinking {
			additionalFields["thinking"] = map[string]interface{}{
				"type":          "enabled",
				"budget_tokens": cfg.GetThinkingTokens(),
			}
		}

		// Add 1M context beta header for Sonnet 4
		if cfg.Uses1MContext() && strings.Contains(strings.ToLower(modelID), "sonnet-4") {
			additionalFields["anthropic-beta"] = "context-1m-2025-08-07"
		}

		// Add any other Bedrock parameters from config
		for key, value := range cfg.Bedrock {
			if key != "thinking" && key != "enable_1m_context" {
				additionalFields[key] = value
			}
		}

		// Only set AdditionalModelRequestFields if we have fields to add
		if len(additionalFields) > 0 {
			docMarshaler := document.NewLazyDocument(additionalFields)
			input.AdditionalModelRequestFields = docMarshaler
		}
	} else if cfg.Thinking.Enabled {
		// Warn user that thinking won't work with system profiles
		fmt.Println("Note: Thinking mode is not available with system profiles")
	}

	// Send to Bedrock
	result, err := client.Converse(ctx, input)
	if err != nil {
		// Check for profile-related errors and retry once
		if !isRetry && (strings.Contains(err.Error(), "profile") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "does not exist")) {
			fmt.Println("Profile may be stale, refreshing...")
			return sendToClaudeWithRetry(turns, true)
		}

		// Provide helpful error messages
		errStr := err.Error()
		if strings.Contains(errStr, "Extra inputs") {
			return "", fmt.Errorf("this model doesn't support the configured features. Try disabling thinking: ask cfg thinking off")
		}
		if strings.Contains(errStr, "thinking") || strings.Contains(errStr, "budget_tokens") {
			return "", fmt.Errorf("thinking configuration error. Try disabling with: ask cfg thinking off\nError: %w", err)
		}
		if strings.Contains(errStr, "inference profile") {
			return "", fmt.Errorf("model requires additional setup. Try: ask cfg model opus")
		}
		if strings.Contains(errStr, "context-1m") {
			return "", fmt.Errorf("1M context window requires tier 4 access. Remove 'enable_1m_context' from config")
		}
		return "", fmt.Errorf("failed to invoke Claude: %w", err)
	}

	// Extract response
	if result.Output == nil {
		return "", fmt.Errorf("empty response from Claude")
	}

	switch v := result.Output.(type) {
	case *types.ConverseOutputMemberMessage:
		if len(v.Value.Content) > 0 {
			// Look for text content (main response)
			for _, content := range v.Value.Content {
				if textBlock, ok := content.(*types.ContentBlockMemberText); ok {
					return textBlock.Value, nil
				}
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

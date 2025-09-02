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

// StreamCallback is called for each chunk of streaming response
type StreamCallback func(chunk string, tokenCount int) error

// StreamToClaudeWithHistory sends conversation history and streams the response
func StreamToClaudeWithHistory(ctx context.Context, turns []session.Turn, callback StreamCallback) (int, error) {
	return streamToClaudeWithRetry(ctx, turns, callback, false)
}

func streamToClaudeWithRetry(ctx context.Context, turns []session.Turn, callback StreamCallback, isRetry bool) (int, error) {
	// Load Ask configuration
	cfg, err := config.Load()
	if err != nil {
		return 0, fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve model ID
	modelID, err := cfg.ResolveModel()
	if err != nil {
		return 0, fmt.Errorf("failed to resolve model: %w", err)
	}

	// If this is a retry, invalidate the cache first
	if isRetry {
		profileName := deriveProfileName(modelID)
		invalidateCachedProfile(profileName)
	}

	// Ensure profile exists and get capabilities
	profileArn, capabilities, err := ensureProfile(modelID)
	if err != nil {
		return 0, fmt.Errorf("failed to setup model: %w", err)
	}

	// Load AWS configuration
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return 0, fmt.Errorf("AWS credentials not configured. Run: aws configure")
	}

	// Create Bedrock client
	client := bedrockruntime.NewFromConfig(awsCfg)

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
	input := &bedrockruntime.ConverseStreamInput{
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

	// Start streaming
	output, err := client.ConverseStream(ctx, input)
	if err != nil {
		// Check for profile-related errors and retry once
		if !isRetry && (strings.Contains(err.Error(), "profile") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "does not exist")) {
			fmt.Println("Profile may be stale, refreshing...")
			return streamToClaudeWithRetry(ctx, turns, callback, true)
		}

		// Provide helpful error messages
		errStr := err.Error()
		if strings.Contains(errStr, "Extra inputs") {
			return 0, fmt.Errorf("this model doesn't support the configured features. Try disabling thinking: ask cfg thinking off")
		}
		if strings.Contains(errStr, "thinking") || strings.Contains(errStr, "budget_tokens") {
			return 0, fmt.Errorf("thinking configuration error. Try disabling with: ask cfg thinking off\nError: %w", err)
		}
		if strings.Contains(errStr, "inference profile") {
			return 0, fmt.Errorf("model requires additional setup. Try: ask cfg model opus")
		}
		if strings.Contains(errStr, "context-1m") {
			return 0, fmt.Errorf("1M context window requires tier 4 access. Remove 'enable_1m_context' from config")
		}
		return 0, fmt.Errorf("failed to invoke Claude: %w", err)
	}

	// Get the event stream
	eventStream := output.GetStream()
	defer eventStream.Close()

	// Process the stream
	totalTokens := 0
	for {
		select {
		case <-ctx.Done():
			// Context cancelled (e.g., Ctrl+C)
			return totalTokens, context.Canceled
		default:
			event, ok := <-eventStream.Events()
			if !ok {
				// Stream ended
				return totalTokens, nil
			}

			switch v := event.(type) {
			case *types.ConverseStreamOutputMemberContentBlockDelta:
				if v.Value.Delta != nil {
					switch delta := v.Value.Delta.(type) {
					case *types.ContentBlockDeltaMemberText:
						// Regular content chunk - not thinking
						chunk := delta.Value
						if chunk != "" {
							tokens := len(chunk) / 4 // Approximate
							totalTokens += tokens
							if err := callback(chunk, totalTokens); err != nil {
								return totalTokens, err
							}
						}
					}
				}

			case *types.ConverseStreamOutputMemberMessageStop:
				// End of message
				return totalTokens, nil

			case *types.ConverseStreamOutputMemberMetadata:
				// Metadata about usage - could extract token counts here
				if v.Value.Usage != nil {
					if v.Value.Usage.OutputTokens != nil {
						totalTokens = int(*v.Value.Usage.OutputTokens)
					}
				}
			}
		}
	}
}

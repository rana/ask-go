package bedrock

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// SendToClaude sends content to Claude via AWS Bedrock
func SendToClaude(content string) (string, error) {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", fmt.Errorf("AWS credentials not configured. Run: aws configure")
	}

	// Create Bedrock client
	client := bedrockruntime.NewFromConfig(cfg)

	// Prepare the request
	request := map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        32000,
		"temperature":       1.0,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": content,
			},
		},
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send to Bedrock
	input := &bedrockruntime.InvokeModelInput{
		ModelId:     stringPtr("us.anthropic.claude-opus-4-1-20250805-v1:0"),
		ContentType: stringPtr("application/json"),
		Body:        requestJSON,
	}

	result, err := client.InvokeModel(context.TODO(), input)
	if err != nil {
		return "", fmt.Errorf("failed to invoke Claude: %w", err)
	}

	// Parse response
	var response struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(result.Body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}

	return response.Content[0].Text, nil
}

func stringPtr(s string) *string {
	return &s
}

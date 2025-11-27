package bedrock

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	"github.com/rana/ask/internal/config"
)

// ModelCapabilities defines what features a model supports
type ModelCapabilities struct {
	SupportsThinking  bool
	Supports1MContext bool
}

// getModelCapabilities returns capabilities based on model ID patterns
func getModelCapabilities(modelID string) ModelCapabilities {
	lower := strings.ToLower(modelID)

	// All modern Claude models support thinking mode
	supportsThinking := strings.Contains(lower, "claude")

	// 1M context: Currently only Sonnet 4 (20241022)
	// Will naturally extend as AWS adds more models
	supports1M := strings.Contains(lower, "sonnet") &&
		strings.Contains(lower, "20241022")

	return ModelCapabilities{
		SupportsThinking:  supportsThinking,
		Supports1MContext: supports1M,
	}
}

// ensureProfile discovers the system-provided inference profile for a model
func ensureProfile(modelID string) (string, ModelCapabilities, error) {
	caps := getModelCapabilities(modelID)
	profileName := deriveProfileName(modelID)

	// Check cache first
	if cachedARN, found := getCachedProfile(profileName); found {
		return cachedARN, caps, nil
	}

	// Discover system profile
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", caps, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := bedrock.NewFromConfig(cfg)

	askConfig, _ := config.Load()
	prefer1M := askConfig != nil && askConfig.Uses1MContext()

	profileArn, err := discoverSystemProfile(context.Background(), client, modelID, prefer1M)
	if err != nil {
		return "", caps, fmt.Errorf(`no system inference profile found for this model

This model requires a system-provided cross-region inference profile.

Common reasons:
  • Cross-region inference not enabled on your AWS account
  • Model not yet available in your region
  • AWS tier insufficient for this model

Solutions:
  1. Check AWS Bedrock console for available models
  2. Try a different model: ask cfg model sonnet
  3. Contact AWS support to enable cross-region inference

Visit: https://docs.aws.amazon.com/bedrock/latest/userguide/cross-region-inference.html

Original error: %w`, err)
	}

	// Cache successful discovery
	setCachedProfile(profileName, profileArn, modelID)

	return profileArn, caps, nil
}

// deriveProfileName creates consistent cache key from model ID
func deriveProfileName(modelID string) string {
	lower := strings.ToLower(modelID)

	// Match on model family and date for specificity
	switch {
	case strings.Contains(lower, "opus") && strings.Contains(lower, "20251101"):
		return "opus-4.5"
	case strings.Contains(lower, "opus") && strings.Contains(lower, "20240229"):
		return "opus-3"
	case strings.Contains(lower, "opus"):
		return "opus"

	case strings.Contains(lower, "sonnet") && strings.Contains(lower, "20241022"):
		return "sonnet-4"
	case strings.Contains(lower, "sonnet"):
		return "sonnet-3.5"

	case strings.Contains(lower, "haiku") && strings.Contains(lower, "20241022"):
		return "haiku-3.5"
	case strings.Contains(lower, "haiku"):
		return "haiku"

	default:
		// Use model family from ID: anthropic.claude-{family}-...
		parts := strings.Split(modelID, ".")
		if len(parts) >= 3 {
			return parts[2]
		}
		return "claude"
	}
}

// discoverSystemProfile finds AWS-provided inference profile
func discoverSystemProfile(ctx context.Context, client *bedrock.Client, modelID string, prefer1M bool) (string, error) {
	input := &bedrock.ListInferenceProfilesInput{
		MaxResults: aws.Int32(100),
	}

	result, err := client.ListInferenceProfiles(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to list profiles: %w", err)
	}

	// Extract model type for matching
	modelLower := strings.ToLower(modelID)
	var modelType string
	switch {
	case strings.Contains(modelLower, "opus"):
		modelType = "opus"
	case strings.Contains(modelLower, "sonnet"):
		modelType = "sonnet"
	case strings.Contains(modelLower, "haiku"):
		modelType = "haiku"
	default:
		return "", fmt.Errorf("unsupported model type in: %s", modelID)
	}

	var standardProfile, extendedProfile string

	for _, profile := range result.InferenceProfileSummaries {
		if profile.InferenceProfileArn == nil {
			continue
		}

		profileArn := *profile.InferenceProfileArn
		profileName := ""
		if profile.InferenceProfileName != nil {
			profileName = strings.ToLower(*profile.InferenceProfileName)
		}

		// Check if profile supports our model type
		supportsModel := false

		if profile.Models != nil {
			for _, model := range profile.Models {
				if model.ModelArn != nil {
					modelArnStr := strings.ToLower(*model.ModelArn)
					if strings.Contains(modelArnStr, modelID) ||
						strings.Contains(modelArnStr, modelType) {
						supportsModel = true
						break
					}
				}
			}
		}

		// Also check profile name (cross-region profiles often named by type)
		if !supportsModel && strings.Contains(profileName, modelType) {
			supportsModel = true
		}

		if supportsModel {
			// Categorize by context window size
			if strings.Contains(profileName, "1m") ||
				strings.Contains(profileName, "million") ||
				strings.Contains(profileName, "extended") {
				extendedProfile = profileArn
			} else {
				standardProfile = profileArn
			}
		}
	}

	// Return preferred profile
	if prefer1M && extendedProfile != "" {
		return extendedProfile, nil
	}

	if standardProfile != "" {
		return standardProfile, nil
	}

	return "", fmt.Errorf("no %s inference profile found", modelType)
}

// invalidateCachedProfile removes profile from cache (used on errors)
func invalidateCachedProfile(profileName string) {
	cache, _ := loadProfileCache()
	delete(cache.Profiles, profileName)
	saveProfileCache(cache)
}

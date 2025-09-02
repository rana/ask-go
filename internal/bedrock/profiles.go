package bedrock

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	"github.com/aws/aws-sdk-go-v2/service/bedrock/types"
	"github.com/rana/ask/internal/config"
)

// ModelCapabilities describes what a model supports
type ModelCapabilities struct {
	SupportsThinking      bool
	RequiresSystemProfile bool
	ProfileStrategy       string // "create" or "discover"
}

// getModelCapabilities returns capabilities for a model
func getModelCapabilities(modelID string) ModelCapabilities {
	lower := strings.ToLower(modelID)

	switch {
	case strings.Contains(lower, "sonnet-4"):
		// Sonnet 4 supports thinking and requires system profiles
		return ModelCapabilities{
			SupportsThinking:      true,
			RequiresSystemProfile: true,
			ProfileStrategy:       "discover",
		}
	case strings.Contains(lower, "opus-4"):
		// Opus 4.1 supports thinking
		return ModelCapabilities{
			SupportsThinking:      true,
			RequiresSystemProfile: false,
			ProfileStrategy:       "create",
		}
	case strings.Contains(lower, "opus") && strings.Contains(lower, "20240229"):
		// Claude 3 Opus supports thinking
		return ModelCapabilities{
			SupportsThinking:      true,
			RequiresSystemProfile: false,
			ProfileStrategy:       "create",
		}
	case strings.Contains(lower, "sonnet") && strings.Contains(lower, "20241022"):
		// Claude 3.5 Sonnet (October 2024) supports thinking
		return ModelCapabilities{
			SupportsThinking:      true,
			RequiresSystemProfile: false,
			ProfileStrategy:       "create",
		}
	default:
		// Older models don't support thinking
		return ModelCapabilities{
			SupportsThinking:      false,
			RequiresSystemProfile: false,
			ProfileStrategy:       "create",
		}
	}
}

// deriveProfileName generates a consistent profile name
func deriveProfileName(modelID string) string {
	lower := strings.ToLower(modelID)
	switch {
	case strings.Contains(lower, "opus"):
		return "ask-opus"
	case strings.Contains(lower, "sonnet"):
		return "ask-sonnet"
	case strings.Contains(lower, "haiku"):
		return "ask-haiku"
	default:
		parts := strings.Split(modelID, ".")
		if len(parts) >= 3 {
			return fmt.Sprintf("ask-%s", parts[2])
		}
		return "ask-default"
	}
}

// ensureProfile ensures we have a profile ARN for the model
func ensureProfile(modelID string) (string, ModelCapabilities, error) {
	// Load config to check context preference
	askConfig, _ := config.Load()
	uses1M := askConfig != nil && askConfig.Uses1MContext()

	caps := getModelCapabilities(modelID)

	// CRITICAL: Sonnet 4 CANNOT use custom profiles, even for 1M context
	if strings.Contains(strings.ToLower(modelID), "sonnet-4") {
		if uses1M {
			fmt.Println("Note: Sonnet 4 requires AWS-provided profiles for 1M context")
			fmt.Println("      Custom 1M profiles are not supported for this model")
		}
		caps.RequiresSystemProfile = true
		caps.ProfileStrategy = "discover"
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", caps, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := bedrock.NewFromConfig(cfg)
	ctx := context.Background()

	// For system profiles, always discover
	if caps.ProfileStrategy == "discover" {
		// Check cache for discovered system profiles
		profileName := "system-" + deriveProfileName(modelID)
		if cachedARN, found := getCachedProfile(profileName); found {
			// Quickly verify it still exists (system profiles should be stable)
			return cachedARN, caps, nil
		}

		// Find system profile
		profileArn, err := discoverSystemProfile(ctx, client, modelID, uses1M)
		if err != nil {
			return "", caps, fmt.Errorf("failed to find system profile: %w", err)
		}

		// Cache the discovered profile
		setCachedProfile(profileName, profileArn, modelID, false)
		return profileArn, caps, nil
	}

	// For custom profiles, check cache FIRST
	profileName := deriveProfileName(modelID)

	// Check cache before any creation attempt
	if cachedARN, found := getCachedProfile(profileName); found {
		// Quick verification that it still exists
		if profile, err := getInferenceProfile(ctx, client, profileName); err == nil && profile != nil {
			// Cache hit - profile exists
			return cachedARN, caps, nil
		}
		// Profile was deleted, continue to recreate
	}

	// Check if profile exists but isn't cached
	if profile, err := getInferenceProfile(ctx, client, profileName); err == nil && profile != nil {
		if profile.InferenceProfileArn != nil {
			arn := *profile.InferenceProfileArn
			// Add to cache
			setCachedProfile(profileName, arn, modelID, true)
			return arn, caps, nil
		}
	}

	// Only NOW do we create the profile
	fmt.Printf("Creating inference profile '%s'... ", profileName)
	arn, err := createInferenceProfile(ctx, client, profileName, modelID, cfg.Region)
	if err != nil {
		fmt.Println("failed")
		return "", caps, err
	}
	fmt.Println("done")

	// Cache the created profile
	setCachedProfile(profileName, arn, modelID, true)

	return arn, caps, nil
}

// discoverSystemProfile finds an existing system profile that supports the model
func discoverSystemProfile(ctx context.Context, client *bedrock.Client, modelID string, prefer1M bool) (string, error) {
	input := &bedrock.ListInferenceProfilesInput{
		MaxResults: aws.Int32(100),
	}

	result, err := client.ListInferenceProfiles(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to list profiles: %w", err)
	}

	var standardProfile string
	var extendedProfile string

	// Look for profiles that support this model
	for _, profile := range result.InferenceProfileSummaries {
		if profile.InferenceProfileArn == nil {
			continue
		}

		profileArn := *profile.InferenceProfileArn
		profileName := ""
		if profile.InferenceProfileName != nil {
			profileName = *profile.InferenceProfileName
		}

		// Check if this profile supports our model
		supportsModel := false
		if profile.Models != nil {
			for _, model := range profile.Models {
				if model.ModelArn != nil && strings.Contains(*model.ModelArn, "sonnet-4") {
					supportsModel = true
					break
				}
			}
		}

		// Also check by name patterns
		if !supportsModel {
			if strings.Contains(profileName, "sonnet") ||
				strings.Contains(profileName, "Sonnet") ||
				strings.Contains(profileName, "cross-region") {
				supportsModel = true
			}
		}

		if supportsModel {
			// Check if it's a 1M context profile
			if strings.Contains(profileName, "1m") ||
				strings.Contains(profileName, "1M") ||
				strings.Contains(profileName, "million") {
				extendedProfile = profileArn
			} else {
				standardProfile = profileArn
			}
		}
	}

	// Return based on preference
	if prefer1M && extendedProfile != "" {
		fmt.Printf("Using 1M context system profile\n")
		return extendedProfile, nil
	}

	if standardProfile != "" {
		if prefer1M {
			fmt.Printf("Note: 1M context profile not found, using standard profile\n")
		}
		return standardProfile, nil
	}

	return "", fmt.Errorf(`no suitable system profile found for Sonnet 4.

This model requires a system-provided inference profile.
Your AWS account may need:
1. Cross-region inference enabled
2. Access to Sonnet 4 profiles

Contact your AWS administrator or try:
  ask cfg model opus     # Claude Opus (supports custom profiles)
  ask cfg model sonnet   # Claude 3.5 Sonnet (older version)`)
}

// getInferenceProfile retrieves an inference profile by name
func getInferenceProfile(ctx context.Context, client *bedrock.Client, profileName string) (*types.InferenceProfileSummary, error) {
	input := &bedrock.ListInferenceProfilesInput{
		MaxResults: aws.Int32(100),
	}

	result, err := client.ListInferenceProfiles(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list profiles: %w", err)
	}

	for _, profile := range result.InferenceProfileSummaries {
		if profile.InferenceProfileName != nil && *profile.InferenceProfileName == profileName {
			return &profile, nil
		}
	}

	return nil, fmt.Errorf("profile not found")
}

// createInferenceProfile creates a new inference profile
func createInferenceProfile(ctx context.Context, client *bedrock.Client, profileName, modelID, region string) (string, error) {
	modelArn := fmt.Sprintf("arn:aws:bedrock:%s::foundation-model/%s", region, modelID)

	input := &bedrock.CreateInferenceProfileInput{
		InferenceProfileName: aws.String(profileName),
		Description:          aws.String("Created by Ask"),
		ModelSource: &types.InferenceProfileModelSourceMemberCopyFrom{
			Value: modelArn,
		},
	}

	result, err := client.CreateInferenceProfile(ctx, input)
	if err != nil {
		if strings.Contains(err.Error(), "AccessDeniedException") {
			return "", fmt.Errorf(`insufficient permissions. Add this policy to your IAM user:
{
    "Version": "2012-10-17",
    "Statement": [{
        "Effect": "Allow",
        "Action": [
            "bedrock:CreateInferenceProfile",
            "bedrock:GetInferenceProfile",
            "bedrock:ListInferenceProfiles"
        ],
        "Resource": "*"
    }]
}`)
		}

		return "", fmt.Errorf("failed to create profile: %w", err)
	}

	if result.InferenceProfileArn != nil {
		time.Sleep(2 * time.Second)
		return *result.InferenceProfileArn, nil
	}

	return profileName, nil
}

// ProfileNameFromModel returns the profile name for a model
func ProfileNameFromModel(modelID string) string {
	return deriveProfileName(modelID)
}

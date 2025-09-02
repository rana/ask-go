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
	SupportsThinking bool
	UseSystemProfile bool // True for models that require AWS-provided profiles
}

// getModelCapabilities returns capabilities for a model
func getModelCapabilities(modelID string) ModelCapabilities {
	lower := strings.ToLower(modelID)

	switch {
	case strings.Contains(lower, "sonnet-4"):
		// Sonnet 4 requires system profiles and supports thinking
		return ModelCapabilities{
			SupportsThinking: true,
			UseSystemProfile: true,
		}
	case strings.Contains(lower, "opus-4"):
		// Opus 4.1 supports thinking and custom profiles
		return ModelCapabilities{
			SupportsThinking: true,
			UseSystemProfile: false,
		}
	case strings.Contains(lower, "opus") && strings.Contains(lower, "20240229"):
		// Claude 3 Opus supports thinking
		return ModelCapabilities{
			SupportsThinking: true,
			UseSystemProfile: false,
		}
	case strings.Contains(lower, "sonnet") && strings.Contains(lower, "20241022"):
		// Claude 3.5 Sonnet (October 2024) supports thinking
		return ModelCapabilities{
			SupportsThinking: true,
			UseSystemProfile: false,
		}
	default:
		// Older models don't support thinking
		return ModelCapabilities{
			SupportsThinking: false,
			UseSystemProfile: false,
		}
	}
}

// ensureProfile ensures we have a profile ARN for the model
func ensureProfile(modelID string) (string, ModelCapabilities, error) {
	caps := getModelCapabilities(modelID)

	// For system profiles, use discovery
	if caps.UseSystemProfile {
		return ensureSystemProfile(modelID, caps)
	}

	// For custom profiles, use creation
	return ensureCustomProfile(modelID, caps)
}

// ensureSystemProfile finds an AWS-provided system profile
func ensureSystemProfile(modelID string, caps ModelCapabilities) (string, ModelCapabilities, error) {
	// Check cache first
	cacheKey := "system-" + deriveProfileName(modelID)
	if cachedARN, found := getCachedProfile(cacheKey); found {
		// Trust the cache - AWS system profiles are stable
		return cachedARN, caps, nil
	}

	// Load config to check context preference
	askConfig, _ := config.Load()
	prefer1M := askConfig != nil && askConfig.Uses1MContext()

	// Discover system profile
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", caps, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := bedrock.NewFromConfig(cfg)
	profileArn, err := discoverSystemProfile(context.Background(), client, modelID, prefer1M)
	if err != nil {
		return "", caps, err
	}

	// Cache the discovered profile
	setCachedProfile(cacheKey, profileArn, modelID, false)
	return profileArn, caps, nil
}

// ensureCustomProfile ensures a custom profile exists
func ensureCustomProfile(modelID string, caps ModelCapabilities) (string, ModelCapabilities, error) {
	profileName := deriveProfileName(modelID)

	// Check cache first - TRUST IT
	if cachedARN, found := getCachedProfile(profileName); found {
		// Return cached ARN immediately - no verification
		// If it fails during actual use, we'll handle it then
		return cachedARN, caps, nil
	}

	// No cache - create the profile
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", caps, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := bedrock.NewFromConfig(cfg)

	// Try to get existing profile by name (in case it exists but isn't cached)
	existingArn, err := findProfileByName(context.Background(), client, profileName)
	if err == nil && existingArn != "" {
		// Found existing profile - cache and return
		setCachedProfile(profileName, existingArn, modelID, true)
		return existingArn, caps, nil
	}

	// Create new profile
	fmt.Printf("Creating inference profile '%s'... ", profileName)
	arn, err := createInferenceProfile(context.Background(), client, profileName, modelID, cfg.Region)
	if err != nil {
		fmt.Println("failed")
		return "", caps, err
	}
	fmt.Println("done")

	// Cache the created profile
	setCachedProfile(profileName, arn, modelID, true)
	return arn, caps, nil
}

// findProfileByName searches for an existing profile by name
func findProfileByName(ctx context.Context, client *bedrock.Client, profileName string) (string, error) {
	input := &bedrock.ListInferenceProfilesInput{
		MaxResults: aws.Int32(100),
	}

	result, err := client.ListInferenceProfiles(ctx, input)
	if err != nil {
		return "", err
	}

	for _, profile := range result.InferenceProfileSummaries {
		if profile.InferenceProfileName != nil &&
			*profile.InferenceProfileName == profileName &&
			profile.InferenceProfileArn != nil {
			return *profile.InferenceProfileArn, nil
		}
	}

	return "", fmt.Errorf("profile not found")
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
		// Small delay to ensure profile is ready
		time.Sleep(2 * time.Second)
		return *result.InferenceProfileArn, nil
	}

	return profileName, nil
}

// invalidateCachedProfile removes a profile from cache
func invalidateCachedProfile(profileName string) {
	cache, _ := loadProfileCache()
	delete(cache.Profiles, profileName)
	saveProfileCache(cache)
}

package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
)

// ModelCache stores cached model information
type ModelCache struct {
	Models   []ModelInfo `toml:"models"`
	CachedAt time.Time   `toml:"cached_at"`
}

// ModelInfo represents a model's metadata
type ModelInfo struct {
	ID      string `toml:"id"`
	Name    string `toml:"name"`    // Human-readable name
	Type    string `toml:"type"`    // "opus", "sonnet", "haiku"
	Version string `toml:"version"` // "4.1", "3.5", etc.
	Date    string `toml:"date"`    // "20250805"
}

// GetModels returns available models, using cache if fresh
func GetModels() ([]ModelInfo, error) {
	cache, err := loadModelCache()
	if err == nil && time.Since(cache.CachedAt) < 24*time.Hour {
		return cache.Models, nil
	}

	// Query AWS Bedrock for models
	models, err := queryBedrockModels()
	if err != nil {
		// If query fails but we have cache, use it
		if cache != nil {
			return cache.Models, nil
		}
		return nil, err
	}

	// Save to cache
	cache = &ModelCache{
		Models:   models,
		CachedAt: time.Now(),
	}
	saveModelCache(cache)

	return models, nil
}

// SelectModel returns the full model ID for a given type or ID
func SelectModel(typeOrID string) (string, error) {
	// If it looks like a full model ID, use it directly
	if strings.Contains(typeOrID, ".") || strings.Contains(typeOrID, ":") {
		return typeOrID, nil
	}

	models, err := GetModels()
	if err != nil {
		return "", fmt.Errorf("failed to get models: %w", err)
	}

	// Normalize the input
	searchType := strings.ToLower(typeOrID)

	// Filter by type
	var matches []ModelInfo
	for _, m := range models {
		if m.Type == searchType {
			matches = append(matches, m)
		}
	}

	if len(matches) == 0 {
		// Try common mappings
		switch searchType {
		case "opus":
			return "anthropic.claude-opus-4-5-20251101-v1:0", nil
		case "sonnet":
			return "anthropic.claude-sonnet-4-5-20250929-v1:0", nil
		case "haiku":
			return "anthropic.claude-haiku-4-5-20251001-v1:0", nil
		default:
			return "", fmt.Errorf("no model found for type '%s'", typeOrID)
		}
	}

	// Sort by date desc, then version desc
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Date != matches[j].Date {
			return matches[i].Date > matches[j].Date
		}
		return matches[i].Version > matches[j].Version
	})

	selected := matches[0]
	return selected.ID, nil
}

// queryBedrockModels queries AWS Bedrock for available models
func queryBedrockModels() ([]ModelInfo, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := bedrock.NewFromConfig(cfg)

	input := &bedrock.ListFoundationModelsInput{}
	result, err := client.ListFoundationModels(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	var models []ModelInfo
	for _, model := range result.ModelSummaries {
		if model.ModelId == nil || !strings.Contains(*model.ModelId, "claude") {
			continue
		}

		info := parseModelID(*model.ModelId)
		if info != nil {
			models = append(models, *info)
		}
	}

	return models, nil
}

// parseModelID extracts model information from an ID
func parseModelID(id string) *ModelInfo {
	info := &ModelInfo{
		ID: id,
	}

	// Parse model type from ID
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "opus"):
		info.Type = "opus"
		info.Name = "Claude 3 Opus"
	case strings.Contains(lower, "sonnet"):
		info.Type = "sonnet"
		info.Name = "Claude 3.5 Sonnet"
	case strings.Contains(lower, "haiku"):
		info.Type = "haiku"
		info.Name = "Claude 3.5 Haiku"
	default:
		return nil
	}

	// Extract version if present (e.g., "4-1" -> "4.1")
	if parts := strings.Split(id, "-"); len(parts) > 3 {
		for i, part := range parts {
			if len(part) == 1 && i+1 < len(parts) && len(parts[i+1]) == 1 {
				// Found version like "4-1"
				info.Version = part + "." + parts[i+1]
				break
			}
		}
	}

	// Extract date if present (YYYYMMDD format)
	for _, part := range strings.Split(id, "-") {
		if len(part) == 8 {
			if _, err := time.Parse("20060102", part); err == nil {
				info.Date = part
				break
			}
		}
	}

	return info
}

// loadModelCache loads the model cache from disk
func loadModelCache() (*ModelCache, error) {
	cachePath := filepath.Join(CachePath(), "models.toml")

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil, err
	}

	var cache ModelCache
	_, err := toml.DecodeFile(cachePath, &cache)
	return &cache, err
}

// saveModelCache saves the model cache to disk
func saveModelCache(cache *ModelCache) error {
	cacheDir := CachePath()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	cachePath := filepath.Join(cacheDir, "models.toml")
	file, err := os.Create(cachePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return toml.NewEncoder(file).Encode(cache)
}

// ListModels returns a formatted list of available models
func ListModels() (string, error) {
	models, err := GetModels()
	if err != nil {
		return "", err
	}

	// Group by type
	byType := make(map[string][]ModelInfo)
	for _, m := range models {
		byType[m.Type] = append(byType[m.Type], m)
	}

	var output []string
	output = append(output, "Available models:\n")

	for _, modelType := range []string{"opus", "sonnet", "haiku"} {
		if typeModels, ok := byType[modelType]; ok && len(typeModels) > 0 {
			output = append(output, fmt.Sprintf("\n%s:", strings.Title(modelType)))
			for i, m := range typeModels {
				marker := ""
				if i == 0 {
					marker = " (latest)"
				}
				output = append(output, fmt.Sprintf("  - %s%s", m.ID, marker))
				if m.Version != "" {
					output = append(output, fmt.Sprintf("    Version: %s", m.Version))
				}
			}
		}
	}

	return strings.Join(output, "\n"), nil
}

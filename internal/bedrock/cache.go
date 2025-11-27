package bedrock

import (
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type ProfileCache struct {
	Profiles map[string]ProfileEntry `toml:"profiles"`
}

type ProfileEntry struct {
	ARN       string    `toml:"arn"`
	CreatedAt time.Time `toml:"created_at"`
	ModelID   string    `toml:"model_id"`
}

func loadProfileCache() (*ProfileCache, error) {
	cachePath := profileCachePath()

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return &ProfileCache{
			Profiles: make(map[string]ProfileEntry),
		}, nil
	}

	var cache ProfileCache
	_, err := toml.DecodeFile(cachePath, &cache)
	if err != nil {
		return &ProfileCache{
			Profiles: make(map[string]ProfileEntry),
		}, nil
	}

	if cache.Profiles == nil {
		cache.Profiles = make(map[string]ProfileEntry)
	}

	return &cache, nil
}

func saveProfileCache(cache *ProfileCache) error {
	cacheDir := filepath.Dir(profileCachePath())
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	file, err := os.Create(profileCachePath())
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	return encoder.Encode(cache)
}

func profileCachePath() string {
	return filepath.Join(os.Getenv("HOME"), ".ask", "cache", "profiles.toml")
}

func getCachedProfile(profileName string) (string, bool) {
	cache, err := loadProfileCache()
	if err != nil {
		return "", false
	}

	if entry, ok := cache.Profiles[profileName]; ok {
		if time.Since(entry.CreatedAt) < 30*24*time.Hour {
			return entry.ARN, true
		}
	}

	return "", false
}

func setCachedProfile(profileName, arn, modelID string) error {
	cache, _ := loadProfileCache()

	cache.Profiles[profileName] = ProfileEntry{
		ARN:       arn,
		CreatedAt: time.Now(),
		ModelID:   modelID,
	}

	return saveProfileCache(cache)
}

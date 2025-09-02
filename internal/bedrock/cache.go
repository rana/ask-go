package bedrock

import (
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// ProfileCache stores cached profile ARNs
type ProfileCache struct {
	Profiles map[string]ProfileEntry `toml:"profiles"`
}

// ProfileEntry stores a cached profile
type ProfileEntry struct {
	ARN       string    `toml:"arn"`
	CreatedAt time.Time `toml:"created_at"`
	ModelID   string    `toml:"model_id"`
	Custom    bool      `toml:"custom"` // true if we created it
}

// loadProfileCache loads the cache from disk
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

// saveProfileCache saves the cache to disk
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

// profileCachePath returns the path to the profile cache
func profileCachePath() string {
	return filepath.Join(os.Getenv("HOME"), ".ask", "cache", "profiles.toml")
}

// getCachedProfile looks up a cached profile
func getCachedProfile(profileName string) (string, bool) {
	cache, err := loadProfileCache()
	if err != nil {
		return "", false
	}

	if entry, ok := cache.Profiles[profileName]; ok {
		// Check if the profile is not too old (30 days)
		if time.Since(entry.CreatedAt) < 30*24*time.Hour {
			return entry.ARN, true
		}
	}

	return "", false
}

// setCachedProfile saves a profile to cache
func setCachedProfile(profileName, arn, modelID string, custom bool) error {
	cache, _ := loadProfileCache()

	cache.Profiles[profileName] = ProfileEntry{
		ARN:       arn,
		CreatedAt: time.Now(),
		ModelID:   modelID,
		Custom:    custom,
	}

	return saveProfileCache(cache)
}

// InvalidateCachedProfile removes a profile from cache (exported)
func InvalidateCachedProfile(profileName string) {
	cache, _ := loadProfileCache()
	delete(cache.Profiles, profileName)
	saveProfileCache(cache)
}

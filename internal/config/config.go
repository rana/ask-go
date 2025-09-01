package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// Config represents the Ask configuration
type Config struct {
	Version     int                    `toml:"version"`
	Model       string                 `toml:"model"`
	Temperature float64                `toml:"temperature"`
	MaxTokens   int                    `toml:"max_tokens"`
	Timeout     string                 `toml:"timeout"`
	Thinking    Thinking               `toml:"thinking"`
	Bedrock     map[string]interface{} `toml:"bedrock,omitempty"`
}

// Thinking represents thinking mode configuration
type Thinking struct {
	Enabled bool    `toml:"enabled"`
	Budget  float64 `toml:"budget"`
}

// Defaults returns a config with sensible defaults
func Defaults() *Config {
	return &Config{
		Version:     1,
		Model:       "opus", // Will resolve to latest Opus model
		Temperature: 1.0,
		MaxTokens:   32000,
		Timeout:     "5m",
		Thinking: Thinking{
			Enabled: false,
			Budget:  0.8,
		},
		Bedrock: make(map[string]interface{}),
	}
}

// Load reads config from ~/.ask/cfg.toml, creating with defaults if needed
func Load() (*Config, error) {
	path := ConfigPath()

	// Create with defaults if doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := Defaults()
		if err := cfg.Save(); err != nil {
			return cfg, fmt.Errorf("failed to save default config: %w", err)
		}
		return cfg, nil
	}

	cfg := &Config{}
	_, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Apply defaults for any missing fields
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 1.0
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 32000
	}
	if cfg.Timeout == "" {
		cfg.Timeout = "5m"
	}
	if cfg.Thinking.Budget == 0 {
		cfg.Thinking.Budget = 0.8
	}
	if cfg.Bedrock == nil {
		cfg.Bedrock = make(map[string]interface{})
	}

	return cfg, nil
}

// Save writes config to ~/.ask/cfg.toml
func (c *Config) Save() error {
	dir := filepath.Dir(ConfigPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	path := ConfigPath()
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	encoder.Indent = ""
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

// ConfigPath returns the path to the config file
func ConfigPath() string {
	return filepath.Join(os.Getenv("HOME"), ".ask", "cfg.toml")
}

// CachePath returns the path to the cache directory
func CachePath() string {
	return filepath.Join(os.Getenv("HOME"), ".ask", "cache")
}

// ParseTimeout returns the timeout as a duration
func (c *Config) ParseTimeout() (time.Duration, error) {
	return time.ParseDuration(c.Timeout)
}

// GetThinkingTokens returns the number of tokens to use for thinking
func (c *Config) GetThinkingTokens() int {
	if !c.Thinking.Enabled {
		return 0
	}
	return int(float64(c.MaxTokens) * c.Thinking.Budget)
}

// ResolveModel returns the full model ID, resolving shortcuts like "opus"
func (c *Config) ResolveModel() (string, error) {
	return SelectModel(c.Model)
}

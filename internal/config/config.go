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
	Context     string                 `toml:"context"`
	Thinking    Thinking               `toml:"thinking"`
	Expand      Expand                 `toml:"expand"`
	Bedrock     map[string]interface{} `toml:"bedrock,omitempty"`
}

// Thinking represents thinking mode configuration
type Thinking struct {
	Enabled bool    `toml:"enabled"`
	Budget  float64 `toml:"budget"`
}

// Expand represents directory expansion configuration
type Expand struct {
	MaxDepth  int         `toml:"max_depth"`
	Recursive bool        `toml:"recursive"`
	Include   IncludeSpec `toml:"include"`
	Exclude   ExcludeSpec `toml:"exclude"`
}

// IncludeSpec defines what to include in expansion
type IncludeSpec struct {
	Extensions []string `toml:"extensions"`
	Patterns   []string `toml:"patterns"`
}

// ExcludeSpec defines what to exclude from expansion
type ExcludeSpec struct {
	Patterns    []string `toml:"patterns"`
	Directories []string `toml:"directories"`
}

// Defaults returns a config with sensible defaults
func Defaults() *Config {
	return &Config{
		Version:     1,
		Model:       "opus", // Will resolve to latest Opus model
		Temperature: 1.0,
		MaxTokens:   32000,
		Timeout:     "5m",
		Context:     "standard",
		Thinking: Thinking{
			Enabled: false,
			Budget:  0.8,
		},
		Expand: Expand{
			MaxDepth:  3,
			Recursive: false,
			Include: IncludeSpec{
				Extensions: []string{"go", "rs", "py", "js", "ts", "jsx", "tsx", "java", "cpp", "c", "h", "hpp", "cs", "rb", "php", "swift", "kt", "scala", "sh", "bash", "zsh", "fish", "ps1", "md", "txt", "json", "yaml", "yml", "toml", "xml", "html", "css", "scss", "sass", "sql", "proto"},
				Patterns:   []string{"Makefile", "Dockerfile", ".gitignore", ".env.example", "README", "LICENSE"},
			},
			Exclude: ExcludeSpec{
				Patterns:    []string{"*_test.go", "*.pb.go", "*_generated.go", "*.min.js", "*.min.css", "*.map"},
				Directories: []string{"vendor", "node_modules", ".git", "dist", "build", "target", "bin", "obj", ".idea", ".vscode", "__pycache__", ".pytest_cache", ".next", ".nuxt", ".output"},
			},
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

	// Track if we need to update the config file
	needsUpdate := false

	// Apply defaults for any missing fields
	if cfg.Version == 0 {
		cfg.Version = 1
		needsUpdate = true
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 1.0
		needsUpdate = true
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 32000
		needsUpdate = true
	}
	if cfg.Timeout == "" {
		cfg.Timeout = "5m"
		needsUpdate = true
	}
	if cfg.Thinking.Budget == 0 {
		cfg.Thinking.Budget = 0.8
		needsUpdate = true
	}
	if cfg.Bedrock == nil {
		cfg.Bedrock = make(map[string]interface{})
	}

	// Apply defaults for expand if missing
	if cfg.Expand.MaxDepth == 0 {
		cfg.Expand.MaxDepth = 3
		needsUpdate = true
	}
	if len(cfg.Expand.Include.Extensions) == 0 {
		defaults := Defaults()
		cfg.Expand.Include = defaults.Expand.Include
		needsUpdate = true
	}
	if len(cfg.Expand.Exclude.Patterns) == 0 {
		defaults := Defaults()
		cfg.Expand.Exclude.Patterns = defaults.Expand.Exclude.Patterns
		needsUpdate = true
	}
	if len(cfg.Expand.Exclude.Directories) == 0 {
		defaults := Defaults()
		cfg.Expand.Exclude.Directories = defaults.Expand.Exclude.Directories
		needsUpdate = true
	}

	// Save back if we added defaults
	if needsUpdate {
		if err := cfg.Save(); err != nil {
			// Non-fatal, just warn
			fmt.Fprintf(os.Stderr, "Warning: couldn't update config with new defaults: %v\n", err)
		}
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

func (c *Config) Uses1MContext() bool {
	return c.Context == "1m"
}

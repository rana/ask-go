package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Version     int                    `toml:"version"`
	Model       string                 `toml:"model"`
	Temperature float64                `toml:"temperature"`
	MaxTokens   int                    `toml:"max_tokens"`
	Timeout     string                 `toml:"timeout"`
	Context     string                 `toml:"context"`
	Thinking    Thinking               `toml:"thinking"`
	Expand      Expand                 `toml:"expand"`
	Filter      Filter                 `toml:"filter"`
	Bedrock     map[string]interface{} `toml:"bedrock,omitempty"`
}

type Thinking struct {
	Enabled bool    `toml:"enabled"`
	Budget  float64 `toml:"budget"`
}

type Expand struct {
	MaxDepth  int         `toml:"max_depth"`
	Recursive bool        `toml:"recursive"`
	Include   IncludeSpec `toml:"include"`
	Exclude   ExcludeSpec `toml:"exclude"`
}

type IncludeSpec struct {
	Extensions []string `toml:"extensions"`
	Patterns   []string `toml:"patterns"`
}

type ExcludeSpec struct {
	Patterns    []string `toml:"patterns"`
	Directories []string `toml:"directories"`
}

type Filter struct {
	Enabled          bool         `toml:"enabled"`
	StripHeaders     bool         `toml:"strip_headers"`
	StripAllComments bool         `toml:"strip_all_comments"`
	Header           HeaderFilter `toml:"header"`
}

type HeaderFilter struct {
	Remove   []HeaderPattern `toml:"remove"`
	Preserve []string        `toml:"preserve"`
}

type HeaderPattern struct {
	Start string `toml:"start"`
	End   string `toml:"end"`
}

func Defaults() *Config {
	return &Config{
		Version:     1,
		Model:       "opus",
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
		Filter: Filter{
			Enabled:          true,
			StripHeaders:     true,
			StripAllComments: false,
			Header: HeaderFilter{
				Remove: []HeaderPattern{
					{Start: "/*", End: "*/"},
					{Start: "/**", End: "*/"},
					{Start: "<!--", End: "-->"},
					{Start: `"""`, End: `"""`},
					{Start: "'''", End: "'''"},
					{Start: "(*", End: "*)"},
					{Start: "{-", End: "-}"},
					{Start: "=begin", End: "=end"},
				},
				Preserve: []string{
					"//go:",
					"// +build",
					"//nolint",
					"//lint:",
					"#!",
					"///<",
					"//go:generate",
					"# -*- coding",
					"# frozen_string_literal",
					`"use strict"`,
					`'use strict'`,
				},
			},
		},
		Bedrock: make(map[string]interface{}),
	}
}

func Load() (*Config, error) {
	path := ConfigPath()

	// Create default config if it doesn't exist
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
	needsUpdate := false

	// Version migration
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

	// Expand defaults
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

	// Filter defaults - migrate from old format
	if len(cfg.Filter.Header.Remove) == 0 {
		defaults := Defaults()
		cfg.Filter.Header = defaults.Filter.Header
		needsUpdate = true
	}

	// Save if we updated defaults
	if needsUpdate {
		if err := cfg.Save(); err != nil {
			// Just warn, don't fail
			fmt.Fprintf(os.Stderr, "Warning: couldn't update config with new defaults: %v\n", err)
		}
	}

	return cfg, nil
}

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

func ConfigPath() string {
	return filepath.Join(os.Getenv("HOME"), ".ask", "cfg.toml")
}

func CachePath() string {
	return filepath.Join(os.Getenv("HOME"), ".ask", "cache")
}

func (c *Config) ParseTimeout() (time.Duration, error) {
	return time.ParseDuration(c.Timeout)
}

func (c *Config) GetThinkingTokens() int {
	if !c.Thinking.Enabled {
		return 0
	}
	return int(float64(c.MaxTokens) * c.Thinking.Budget)
}

func (c *Config) ResolveModel() (string, error) {
	return SelectModel(c.Model)
}

func (c *Config) Uses1MContext() bool {
	return c.Context == "1m"
}

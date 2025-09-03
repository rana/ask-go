package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rana/ask/internal/config"
)

// CfgCmd manages configuration
type CfgCmd struct {
	Show           CfgShowCmd           `cmd:"" help:"Show current configuration"`
	Models         CfgModelsCmd         `cmd:"" help:"List available models"`
	Model          CfgModelCmd          `cmd:"" help:"Set model"`
	Temperature    CfgTemperatureCmd    `cmd:"" help:"Set temperature (0.0-1.0)"`
	MaxTokens      CfgMaxTokensCmd      `cmd:"" help:"Set max tokens"`
	Timeout        CfgTimeoutCmd        `cmd:"" help:"Set timeout duration"`
	Thinking       CfgThinkingCmd       `cmd:"" help:"Enable/disable thinking mode"`
	ThinkingBudget CfgThinkingBudgetCmd `cmd:"" help:"Set thinking budget (0.0-1.0)"`
	Context        CfgContextCmd        `cmd:"" help:"Set context window size"`
	Expand         CfgExpandCmd         `cmd:"" help:"Configure directory expansion"`
	Filter         CfgFilterCmd         `cmd:"" help:"Configure content filtering"`
}

// CfgShowCmd explicitly shows configuration
type CfgShowCmd struct{}

func (c *CfgShowCmd) Run(cmdCtx *Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Current configuration (~/.ask/cfg.toml):\n\n")
	fmt.Printf("Model:           %s\n", cfg.Model)

	// Try to resolve model to show full ID
	if resolved, err := cfg.ResolveModel(); err == nil && resolved != cfg.Model {
		fmt.Printf("                 → %s\n", resolved)
	}

	fmt.Printf("Temperature:     %.1f\n", cfg.Temperature)
	fmt.Printf("Max Tokens:      %d\n", cfg.MaxTokens)
	fmt.Printf("Timeout:         %s\n", cfg.Timeout)
	fmt.Printf("Thinking:        %v\n", cfg.Thinking.Enabled)
	if cfg.Thinking.Enabled {
		fmt.Printf("Thinking Budget: %.0f%% (%d tokens)\n",
			cfg.Thinking.Budget*100,
			cfg.GetThinkingTokens())
	}
	fmt.Printf("Context:         %s\n", cfg.Context)

	fmt.Printf("\nDirectory Expansion:\n")
	fmt.Printf("  Recursive:     %v\n", cfg.Expand.Recursive)
	fmt.Printf("  Max Depth:     %d\n", cfg.Expand.MaxDepth)

	fmt.Printf("\nContent Filtering:\n")
	fmt.Printf("  Enabled:       %v\n", cfg.Filter.Enabled)
	if cfg.Filter.Enabled {
		fmt.Printf("  Strip Headers: %v\n", cfg.Filter.StripHeaders)
		fmt.Printf("  Strip Comments: %v\n", cfg.Filter.StripAllComments)
	}

	return nil
}

// CfgModelsCmd lists available models
type CfgModelsCmd struct{}

func (c *CfgModelsCmd) Run(cmdCtx *Context) error {
	output, err := config.ListModels()
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}
	fmt.Println(output)
	return nil
}

// CfgModelCmd sets the model
type CfgModelCmd struct {
	Model string `arg:"" help:"Model type (opus/sonnet/haiku) or full model ID"`
}

func (c *CfgModelCmd) Run(cmdCtx *Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate that the model exists or can be resolved
	resolved, err := config.SelectModel(c.Model)
	if err != nil {
		return fmt.Errorf("invalid model '%s': %w", c.Model, err)
	}

	cfg.Model = c.Model
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Model set to: %s\n", c.Model)
	if resolved != c.Model {
		fmt.Printf("Resolves to:  %s\n", resolved)
	}
	return nil
}

// CfgTemperatureCmd sets the temperature
type CfgTemperatureCmd struct {
	Temperature float64 `arg:"" help:"Temperature value (0.0-1.0)"`
}

func (c *CfgTemperatureCmd) Run(cmdCtx *Context) error {
	if c.Temperature < 0 || c.Temperature > 1 {
		return fmt.Errorf("temperature must be between 0.0 and 1.0")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.Temperature = c.Temperature
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Temperature set to: %.1f\n", c.Temperature)
	return nil
}

// CfgMaxTokensCmd sets the max tokens
type CfgMaxTokensCmd struct {
	MaxTokens int `arg:"" help:"Maximum tokens"`
}

func (c *CfgMaxTokensCmd) Run(cmdCtx *Context) error {
	if c.MaxTokens <= 0 {
		return fmt.Errorf("max tokens must be positive")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.MaxTokens = c.MaxTokens
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Max tokens set to: %d\n", c.MaxTokens)
	return nil
}

// CfgTimeoutCmd sets the timeout
type CfgTimeoutCmd struct {
	Timeout string `arg:"" help:"Timeout duration (e.g., 5m, 30s)"`
}

func (c *CfgTimeoutCmd) Run(cmdCtx *Context) error {
	// Validate duration format
	if _, err := config.Defaults().ParseTimeout(); err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.Timeout = c.Timeout
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Timeout set to: %s\n", c.Timeout)
	return nil
}

// CfgThinkingCmd enables/disables thinking mode
type CfgThinkingCmd struct {
	Enable string `arg:"" help:"Enable thinking: on/off/true/false"`
}

func (c *CfgThinkingCmd) Run(cmdCtx *Context) error {
	enable := false
	switch strings.ToLower(c.Enable) {
	case "on", "true", "yes", "1":
		enable = true
	case "off", "false", "no", "0":
		enable = false
	default:
		return fmt.Errorf("invalid value: use on/off or true/false")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.Thinking.Enabled = enable
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Thinking mode: %v\n", enable)
	if enable {
		fmt.Printf("Thinking budget: %.0f%% (%d tokens)\n",
			cfg.Thinking.Budget*100,
			cfg.GetThinkingTokens())
	}
	return nil
}

// CfgThinkingBudgetCmd sets the thinking budget
type CfgThinkingBudgetCmd struct {
	Budget string `arg:"" help:"Budget as decimal (0.8) or percentage (80%)"`
}

func (c *CfgThinkingBudgetCmd) Run(cmdCtx *Context) error {
	var budget float64
	var err error

	// Handle percentage format
	if strings.HasSuffix(c.Budget, "%") {
		percentStr := strings.TrimSuffix(c.Budget, "%")
		percent, err := strconv.ParseFloat(percentStr, 64)
		if err != nil {
			return fmt.Errorf("invalid budget format: %w", err)
		}
		budget = percent / 100.0
	} else {
		budget, err = strconv.ParseFloat(c.Budget, 64)
		if err != nil {
			return fmt.Errorf("invalid budget format: %w", err)
		}
	}

	if budget <= 0 || budget > 1 {
		return fmt.Errorf("budget must be between 0.0 and 1.0")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.Thinking.Budget = budget
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Thinking budget set to: %.0f%% (%d tokens)\n",
		budget*100,
		int(float64(cfg.MaxTokens)*budget))
	return nil
}

type CfgContextCmd struct {
	Size string `arg:"" optional:"" help:"Context size: standard or 1m"`
}

func (c *CfgContextCmd) Run(cmdCtx *Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Show current setting if no argument
	if c.Size == "" {
		currentSize := "standard"
		if cfg.Context == "1m" {
			currentSize = "1m (1 million tokens)"
		}
		fmt.Printf("Context window preference: %s\n", currentSize)

		// Show model-specific reality
		switch cfg.Model {
		case "sonnet", "sonnet-4":
			fmt.Println("\nSonnet 4 status:")
			fmt.Println("  - Uses AWS system profiles only")
			fmt.Println("  - 1M context requires AWS to provide it")
			fmt.Println("  - Cannot create custom profiles")
		case "opus":
			fmt.Println("\nOpus status:")
			fmt.Println("  - Supports custom profiles")
			fmt.Println("  - Standard context (200k tokens)")
		}

		return nil
	}

	// Validate and set new size
	switch strings.ToLower(c.Size) {
	case "standard", "200k", "default":
		cfg.Context = "standard"
		fmt.Println("Context preference set to: standard")
	case "1m", "1million", "million":
		cfg.Context = "1m"
		fmt.Println("Context preference set to: 1m")
		fmt.Println("\nNote: 1M context availability depends on:")
		fmt.Println("  - Your AWS tier (requires tier 4)")
		fmt.Println("  - Model support (Sonnet 4 only)")
		fmt.Println("  - AWS providing appropriate system profiles")
	default:
		return fmt.Errorf("invalid context size. Use 'standard' or '1m'")
	}

	return cfg.Save()
}

// CfgExpandCmd manages expansion settings
type CfgExpandCmd struct {
	Recursive CfgExpandRecursiveCmd `cmd:"" help:"Set recursive expansion default"`
	MaxDepth  CfgExpandMaxDepthCmd  `cmd:"" help:"Set maximum recursion depth"`
}

// Run shows current expansion settings
func (c *CfgExpandCmd) Run(cmdCtx *Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Directory expansion settings:\n")
	fmt.Printf("  Recursive: %v\n", cfg.Expand.Recursive)
	fmt.Printf("  Max Depth: %d\n", cfg.Expand.MaxDepth)
	fmt.Printf("\nNote: Use [[dir/**/]] to force recursive expansion\n")

	return nil
}

// CfgExpandRecursiveCmd sets recursive expansion default
type CfgExpandRecursiveCmd struct {
	Enable string `arg:"" help:"Enable recursive: on/off"`
}

func (c *CfgExpandRecursiveCmd) Run(cmdCtx *Context) error {
	enable := false
	switch strings.ToLower(c.Enable) {
	case "on", "true", "yes", "1":
		enable = true
	case "off", "false", "no", "0":
		enable = false
	default:
		return fmt.Errorf("invalid value: use on/off")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.Expand.Recursive = enable
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Recursive expansion default: %v\n", enable)
	if !enable {
		fmt.Println("Tip: Use [[dir/**/]] to force recursive for specific directories")
	}
	return nil
}

// CfgExpandMaxDepthCmd sets max recursion depth
type CfgExpandMaxDepthCmd struct {
	Depth int `arg:"" help:"Maximum depth (1-10)"`
}

func (c *CfgExpandMaxDepthCmd) Run(cmdCtx *Context) error {
	if c.Depth < 1 || c.Depth > 10 {
		return fmt.Errorf("depth must be between 1 and 10")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.Expand.MaxDepth = c.Depth
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Max recursion depth set to: %d\n", c.Depth)
	return nil
}

// CfgFilterCmd manages filter settings
type CfgFilterCmd struct {
	Enable        CfgFilterEnableCmd   `cmd:"" help:"Enable/disable filtering"`
	Headers       CfgFilterHeadersCmd  `cmd:"" help:"Enable/disable header stripping"`
	StripComments CfgFilterCommentsCmd `cmd:"" help:"Enable/disable comment stripping"`
}

// Run shows current filter settings
func (c *CfgFilterCmd) Run(cmdCtx *Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Content filtering settings:\n")
	fmt.Printf("  Enabled:          %v\n", cfg.Filter.Enabled)
	fmt.Printf("  Strip Headers:    %v\n", cfg.Filter.StripHeaders)
	fmt.Printf("  Strip All Comments: %v\n", cfg.Filter.StripAllComments)
	fmt.Printf("\nGo-specific settings:\n")
	fmt.Printf("  Header Lines:     %d\n", cfg.Filter.Go.HeaderLines)
	fmt.Printf("  Header Keywords:  %v\n", cfg.Filter.Go.HeaderKeywords)

	return nil
}

// CfgFilterEnableCmd enables/disables filtering
type CfgFilterEnableCmd struct {
	Enable string `arg:"" help:"Enable filtering: on/off"`
}

func (c *CfgFilterEnableCmd) Run(cmdCtx *Context) error {
	enable := false
	switch strings.ToLower(c.Enable) {
	case "on", "true", "yes", "1":
		enable = true
	case "off", "false", "no", "0":
		enable = false
	default:
		return fmt.Errorf("invalid value: use on/off")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.Filter.Enabled = enable
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Content filtering: %v\n", enable)
	return nil
}

// CfgFilterHeadersCmd enables/disables header stripping
type CfgFilterHeadersCmd struct {
	Enable string `arg:"" help:"Strip headers: on/off"`
}

func (c *CfgFilterHeadersCmd) Run(cmdCtx *Context) error {
	enable := false
	switch strings.ToLower(c.Enable) {
	case "on", "true", "yes", "1":
		enable = true
	case "off", "false", "no", "0":
		enable = false
	default:
		return fmt.Errorf("invalid value: use on/off")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.Filter.StripHeaders = enable
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Header stripping: %v\n", enable)
	return nil
}

// CfgFilterCommentsCmd enables/disables comment stripping
type CfgFilterCommentsCmd struct {
	Enable string `arg:"" help:"Strip all comments: on/off"`
}

func (c *CfgFilterCommentsCmd) Run(cmdCtx *Context) error {
	enable := false
	switch strings.ToLower(c.Enable) {
	case "on", "true", "yes", "1":
		enable = true
	case "off", "false", "no", "0":
		enable = false
	default:
		return fmt.Errorf("invalid value: use on/off")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.Filter.StripAllComments = enable
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Strip all comments: %v\n", enable)
	return nil
}

package cmd

import (
	"fmt"
	"os"

	"github.com/rana/ask/internal/bedrock"
	"github.com/rana/ask/internal/config"
	"github.com/rana/ask/internal/expand"
	"github.com/rana/ask/internal/session"
)

// ChatCmd processes the chat session
type ChatCmd struct{}

// Run executes the chat command
func (c *ChatCmd) Run() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		// Continue with defaults if config fails
		fmt.Printf("Warning: using default configuration: %v\n", err)
	}

	// Check if session.md exists
	content, err := os.ReadFile("session.md")
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no session.md found. Run 'ask init' to start")
		}
		return fmt.Errorf("failed to read session.md: %w", err)
	}

	// Parse all turns from the session
	turns, err := session.ParseAllTurns(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse session: %w", err)
	}

	// Check if there's at least one human turn
	lastHumanIndex := -1
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Role == "Human" {
			lastHumanIndex = i
			break
		}
	}

	if lastHumanIndex == -1 {
		return fmt.Errorf("no human turn found in session.md")
	}

	// Check if the last human turn has content
	if turns[lastHumanIndex].Content == "" {
		return fmt.Errorf("turn %d has no content. Add your thoughts and try again",
			turns[lastHumanIndex].Number)
	}

	// Expand file references in all human turns
	totalExpansions := 0
	var allStats []expand.FileStat
	originalContent := string(content)
	updatedContent := originalContent

	for i, turn := range turns {
		if turn.Role == "Human" {
			expanded, stats, err := expand.ExpandReferences(turn.Content, turn.Number)
			if err != nil {
				return fmt.Errorf("failed to expand references in turn %d: %w", turn.Number, err)
			}

			if len(stats) > 0 {
				// Update the turn with expanded content
				turns[i].Content = expanded
				allStats = append(allStats, stats...)
				totalExpansions += len(stats)

				// Update session.md with expanded content if this is the last human turn
				if i == lastHumanIndex {
					updatedContent = session.ReplaceLastHumanTurn(originalContent, turn.Number, expanded)
				}
			}
		}
	}

	// Show expansion stats
	if totalExpansions > 0 {
		fmt.Printf("Expanding %d file references...\n", totalExpansions)
		for _, stat := range allStats {
			fmt.Printf("- %s (%d tokens)\n", stat.File, stat.Tokens)
		}
	}

	// Calculate token statistics
	totalTokens := 0
	for _, turn := range turns {
		tokens := countTokensApprox(turn.Content)
		totalTokens += tokens
		fmt.Printf("%s turn %d: %d tokens\n", turn.Role, turn.Number, tokens)
	}
	fmt.Printf("Total input: %d tokens\n", totalTokens)

	// Show model being used
	if cfg != nil {
		modelID, _ := cfg.ResolveModel()
		fmt.Printf("Model: %s\n", modelID)
		if cfg.Thinking.Enabled {
			fmt.Printf("Thinking: enabled (budget: %d tokens)\n", cfg.GetThinkingTokens())
		}
	}
	fmt.Println()

	// Send full conversation history to Bedrock
	fmt.Println("Sending conversation to Claude...")
	response, err := bedrock.SendToClaudeWithHistory(turns)
	if err != nil {
		return fmt.Errorf("failed to send to Claude: %w", err)
	}

	aiTokens := countTokensApprox(response)
	fmt.Printf("\nAI response: %d tokens\n", aiTokens)

	// Calculate next turn number
	nextTurnNumber := turns[len(turns)-1].Number + 1

	// Append AI response to session
	updatedContent = session.AppendAIResponse(updatedContent, nextTurnNumber, response)

	// Write updated session atomically
	if err := session.WriteAtomic("session.md", []byte(updatedContent)); err != nil {
		return fmt.Errorf("failed to update session.md: %w", err)
	}

	totalSessionTokens := countTokensApprox(updatedContent)
	fmt.Printf("Total session: %d tokens\n", totalSessionTokens)

	return nil
}

// countTokensApprox provides rough token count (1 token ≈ 4 chars)
func countTokensApprox(text string) int {
	return len(text) / 4
}

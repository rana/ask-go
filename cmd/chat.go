package cmd

import (
	"fmt"
	"os"

	"github.com/rana/ask/internal/bedrock"
	"github.com/rana/ask/internal/expand"
	"github.com/rana/ask/internal/session"
)

// ChatCmd processes the chat session
type ChatCmd struct{}

// Run executes the chat command
func (c *ChatCmd) Run() error {
	// Check if session.md exists
	content, err := os.ReadFile("session.md")
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no session.md found. Run 'ask init' to start")
		}
		return fmt.Errorf("failed to read session.md: %w", err)
	}

	// Parse session to find last human turn
	turnNumber, turnContent := session.FindLastHumanTurn(string(content))
	if turnNumber == 0 {
		return fmt.Errorf("no human turn found in session.md")
	}
	if turnContent == "" {
		return fmt.Errorf("turn %d has no content. Add your thoughts and try again", turnNumber)
	}

	// Expand file references
	expanded, stats, err := expand.ExpandReferences(turnContent, turnNumber)
	if err != nil {
		return fmt.Errorf("failed to expand references: %w", err)
	}

	// Show expansion stats
	if len(stats) > 0 {
		fmt.Printf("Expanding %d file references...\n", len(stats))
		for _, stat := range stats {
			fmt.Printf("- %s (%d tokens)\n", stat.File, stat.Tokens)
		}
	}

	humanTokens := countTokensApprox(expanded)
	fmt.Printf("Human turn: %d tokens\n\n", humanTokens)

	// Send to Bedrock
	fmt.Println("Sending to Claude...")
	response, err := bedrock.SendToClaudeV3(expanded)
	if err != nil {
		return fmt.Errorf("failed to send to Claude: %w", err)
	}

	aiTokens := countTokensApprox(response)
	fmt.Printf("\nAI response: %d tokens\n", aiTokens)

	// Update session.md with expanded content and response
	updatedContent := session.ReplaceLastHumanTurn(string(content), turnNumber, expanded)
	updatedContent = session.AppendAIResponse(updatedContent, turnNumber+1, response)

	// Write updated session atomically
	if err := session.WriteAtomic("session.md", []byte(updatedContent)); err != nil {
		return fmt.Errorf("failed to update session.md: %w", err)
	}

	totalTokens := countTokensApprox(updatedContent)
	fmt.Printf("Total session: %d tokens\n", totalTokens)

	return nil
}

// countTokensApprox provides rough token count (1 token ≈ 4 chars)
func countTokensApprox(text string) int {
	return len(text) / 4
}

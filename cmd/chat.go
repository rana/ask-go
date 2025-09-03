package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rana/ask/internal/bedrock"
	"github.com/rana/ask/internal/config"
	"github.com/rana/ask/internal/expand"
	"github.com/rana/ask/internal/session"
)

// ChatCmd processes the chat session
type ChatCmd struct{}

// Run executes the chat command
func (c *ChatCmd) Run(cmdCtx *Context) error {
	// Use the context from main that has signal handling
	ctx := cmdCtx.Context

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

	// Show expansion stats (only if there are expansions)
	if totalExpansions > 0 {
		fmt.Printf("Expanding %d file references...\n", totalExpansions)
		for _, stat := range allStats {
			// Show directory indicator for multiple files from same dir
			if strings.Contains(stat.File, "/") {
				fmt.Printf("  %s (%d tokens)\n", stat.File, stat.Tokens)
			} else {
				fmt.Printf("  %s (%d tokens)\n", stat.File, stat.Tokens)
			}
		}
		fmt.Println()
	}

	// Show model being used
	if cfg != nil {
		modelID, _ := cfg.ResolveModel()
		fmt.Printf("Model: %s\n", modelID)
		if cfg.Thinking.Enabled {
			fmt.Printf("Thinking: enabled (budget: %d tokens)\n", cfg.GetThinkingTokens())
		}
	}
	fmt.Println()

	// Write expanded content if we had expansions
	if totalExpansions > 0 {
		if err := session.WriteAtomic("session.md", []byte(updatedContent)); err != nil {
			return fmt.Errorf("failed to update session.md: %w", err)
		}
	}

	// Calculate next turn number
	nextTurnNumber := turns[len(turns)-1].Number + 1

	// Stream the response
	fmt.Println("Streaming response... [ctrl+c to interrupt]")

	var finalTokenCount int
	err = session.StreamResponse("session.md", nextTurnNumber, func(writer *session.StreamWriter) (int, error) {
		// Progress indicator in terminal
		lastPrintedTokens := 0

		tokenCount, err := bedrock.StreamToClaudeWithHistory(ctx, turns, func(chunk string, currentTokens int) error {
			// Write chunk to file
			if err := writer.WriteChunk(chunk); err != nil {
				return err
			}

			// Update terminal progress (print every 100 tokens)
			if currentTokens-lastPrintedTokens >= 100 || currentTokens < 100 {
				fmt.Printf("\rStreaming response... %d tokens [ctrl+c to interrupt]", currentTokens)
				lastPrintedTokens = currentTokens
			}

			return nil
		})

		finalTokenCount = tokenCount
		return tokenCount, err
	})

	// Clear the streaming line
	fmt.Print("\r                                                           \r")

	if err != nil {
		if err == context.Canceled {
			fmt.Printf("Response interrupted after %d tokens\n", finalTokenCount)
		} else {
			return fmt.Errorf("streaming failed: %w", err)
		}
	} else {
		fmt.Printf("Response complete: %d tokens\n", finalTokenCount)
	}

	return nil
}

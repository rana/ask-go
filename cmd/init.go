package cmd

import (
	"fmt"
	"os"

	"github.com/rana/ask/internal/session"
)

// InitCmd initializes a new session
type InitCmd struct{}

// Run executes the init command
func (c *InitCmd) Run() error {
	// Check if session.md already exists
	if _, err := os.Stat("session.md"); err == nil {
		return fmt.Errorf("session.md already exists. Delete it to start fresh")
	}

	// Create initial session content
	content := "# [1] Human\n\n"

	// Write session.md
	if err := session.WriteAtomic("session.md", []byte(content)); err != nil {
		return fmt.Errorf("failed to create session.md: %w", err)
	}

	fmt.Println("Created session.md")
	return nil
}

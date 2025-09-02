package cmd

import (
	"fmt"

	"github.com/rana/ask/internal/version"
)

// VersionCmd shows version information
type VersionCmd struct{}

// Run executes the version command
func (c *VersionCmd) Run() error {
	fmt.Println(version.String())
	return nil
}

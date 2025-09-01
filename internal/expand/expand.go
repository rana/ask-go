package expand

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileStat represents statistics about an expanded file
type FileStat struct {
	File   string
	Tokens int
}

// ExpandReferences expands [[file]] references in content
func ExpandReferences(content string, turnNumber int) (string, []FileStat, error) {
	// Pattern to match [[file]] references
	pattern := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	matches := pattern.FindAllStringSubmatch(content, -1)

	if len(matches) == 0 {
		return content, nil, nil
	}

	var stats []FileStat
	expanded := content
	sectionNumber := 1

	for _, match := range matches {
		fullMatch := match[0] // [[file]]
		fileName := match[1]  // file

		// Read the file
		fileContent, err := os.ReadFile(fileName)
		if err != nil {
			if os.IsNotExist(err) {
				return "", nil, fmt.Errorf("cannot find '%s' referenced in turn %d", fileName, turnNumber)
			}
			return "", nil, fmt.Errorf("failed to read '%s': %w", fileName, err)
		}

		// Check if binary
		if isBinary(fileContent) {
			fmt.Printf("Skipping binary file '%s'\n", fileName)
			expanded = strings.Replace(expanded, fullMatch, "", 1)
			continue
		}

		// Create the expanded section
		baseName := filepath.Base(fileName)
		section := fmt.Sprintf("### [%d.%d] %s\n```markdown\n%s\n```",
			turnNumber, sectionNumber, baseName, string(fileContent))

		// Replace the reference with the expanded section
		expanded = strings.Replace(expanded, fullMatch, section, 1)

		// Track stats
		tokens := len(fileContent) / 4 // Rough approximation
		stats = append(stats, FileStat{File: fileName, Tokens: tokens})

		sectionNumber++
	}

	return expanded, stats, nil
}

// isBinary checks if content appears to be binary
func isBinary(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return true
		}
	}
	return false
}

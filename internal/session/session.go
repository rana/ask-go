package session

import (
	"fmt"
	"os"
	"strings"
)

// WriteAtomic writes content to file atomically
func WriteAtomic(path string, content []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReplaceLastHumanTurn replaces the last human turn with expanded content
func ReplaceLastHumanTurn(content string, turnNumber int, expanded string) string {
	header := fmt.Sprintf("## [%d] Human", turnNumber)

	// Find the header position
	headerPos := strings.LastIndex(content, header)
	if headerPos == -1 {
		return content
	}

	// Find the next header (if any)
	afterHeader := content[headerPos+len(header):]
	nextHeaderPos := strings.Index(afterHeader, "\n## [")

	var result string
	if nextHeaderPos == -1 {
		// This is the last section
		result = content[:headerPos] + header + "\n\n" + strings.TrimSpace(expanded) + "\n"
	} else {
		// There's another section after
		endPos := headerPos + len(header) + nextHeaderPos
		result = content[:headerPos] + header + "\n\n" + strings.TrimSpace(expanded) + "\n" + content[endPos:]
	}

	return result
}

// AppendAIResponse appends an AI response to the session
func AppendAIResponse(content string, turnNumber int, response string) string {
	aiSection := fmt.Sprintf("\n## [%d] AI\n\n````markdown\n%s\n````\n", turnNumber, strings.TrimSpace(response))
	return content + aiSection
}

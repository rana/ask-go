package session

import (
	"fmt"
	"regexp"
	"strings"
)

// FindLastHumanTurn finds the last human turn in the session
func FindLastHumanTurn(content string) (turnNumber int, turnContent string) {
	// Pattern to match ## [N] Human headers
	pattern := regexp.MustCompile(`## \[(\d+)\] Human`)
	matches := pattern.FindAllStringSubmatchIndex(content, -1)

	if len(matches) == 0 {
		return 0, ""
	}

	// Get the last match
	lastMatch := matches[len(matches)-1]
	turnNumber = parseIntOrZero(content[lastMatch[2]:lastMatch[3]])

	// Extract content from after the header to the next ## header or EOF
	startPos := lastMatch[1] // End of the match

	// Find the next header
	afterHeader := content[startPos:]
	nextHeaderPos := strings.Index(afterHeader, "\n## [")

	if nextHeaderPos == -1 {
		// This is the last section
		turnContent = strings.TrimSpace(afterHeader)
	} else {
		// There's another section after
		turnContent = strings.TrimSpace(afterHeader[:nextHeaderPos])
	}

	return turnNumber, turnContent
}

func parseIntOrZero(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

package session

import (
	"fmt"
	"regexp"
	"strings"
)

// Turn represents a conversation turn
type Turn struct {
	Number  int
	Role    string // "Human" or "AI"
	Content string
}

// ParseAllTurns extracts all turns from the session
func ParseAllTurns(content string) ([]Turn, error) {
	var turns []Turn

	// Pattern to match both Human and AI headers
	pattern := regexp.MustCompile(`# \[(\d+)\] (Human|AI)`)
	matches := pattern.FindAllStringSubmatchIndex(content, -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no turns found in session")
	}

	for i, match := range matches {
		turnNumber := parseIntOrZero(content[match[2]:match[3]])
		role := content[match[4]:match[5]]

		// Extract content from after header to next header or EOF
		startPos := match[1] // End of the match
		var endPos int

		if i < len(matches)-1 {
			// There's another turn after this one
			endPos = matches[i+1][0]
		} else {
			// This is the last turn
			endPos = len(content)
		}

		turnContent := strings.TrimSpace(content[startPos:endPos])

		// For AI turns, strip the markdown wrapper
		if role == "AI" {
			turnContent = stripMarkdownWrapper(turnContent)
		}

		turns = append(turns, Turn{
			Number:  turnNumber,
			Role:    role,
			Content: turnContent,
		})
	}

	return turns, nil
}

// stripMarkdownWrapper removes ````markdown wrapper from AI responses
func stripMarkdownWrapper(content string) string {
	// Remove leading ````markdown
	if strings.HasPrefix(content, "````markdown\n") {
		content = strings.TrimPrefix(content, "````markdown\n")
	} else if strings.HasPrefix(content, "````markdown") {
		content = strings.TrimPrefix(content, "````markdown")
	}

	// Remove trailing ````
	if strings.HasSuffix(content, "\n````") {
		content = strings.TrimSuffix(content, "\n````")
	} else if strings.HasSuffix(content, "````") {
		content = strings.TrimSuffix(content, "````")
	}

	return strings.TrimSpace(content)
}

// FindLastHumanTurn finds the last human turn in the session
func FindLastHumanTurn(content string) (turnNumber int, turnContent string) {
	// Pattern to match # [N] Human headers
	pattern := regexp.MustCompile(`# \[(\d+)\] Human`)
	matches := pattern.FindAllStringSubmatchIndex(content, -1)

	if len(matches) == 0 {
		return 0, ""
	}

	// Get the last match
	lastMatch := matches[len(matches)-1]
	turnNumber = parseIntOrZero(content[lastMatch[2]:lastMatch[3]])

	// Extract content from after the header to the next # header or EOF
	startPos := lastMatch[1] // End of the match

	// Find the next header
	afterHeader := content[startPos:]
	nextHeaderPos := strings.Index(afterHeader, "\n# [")

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

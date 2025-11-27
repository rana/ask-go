package expand

import (
	"fmt"
	"regexp"
	"strings"
)

type MarkdownContext struct {
	HeaderLevel  int    // 1-6 for #-######
	NumberPrefix string // "1.1" from [1.1]
}

var defaultContext = MarkdownContext{HeaderLevel: 2, NumberPrefix: ""}

// detectMarkdownContext looks backward from the reference position to find the nearest heading
func detectMarkdownContext(content string, referencePos int) MarkdownContext {
	if referencePos <= 0 || referencePos > len(content) {
		return defaultContext
	}

	// Get content before the reference
	beforeRef := content[:referencePos]

	// Find the last line that starts with # (heading)
	lines := strings.Split(beforeRef, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "#") {
			return parseHeading(line)
		}
	}

	return defaultContext
}

// parseHeading extracts header level and section number from a markdown heading
func parseHeading(line string) MarkdownContext {
	ctx := defaultContext

	// Count the # symbols
	hashCount := 0
	for _, ch := range line {
		if ch == '#' {
			hashCount++
		} else {
			break
		}
	}

	if hashCount > 0 && hashCount <= 6 {
		// Set next level (one deeper than found)
		ctx.HeaderLevel = hashCount + 1
		if ctx.HeaderLevel > 6 {
			ctx.HeaderLevel = 6 // Max markdown header level
		}
	}

	// Extract section number pattern like [1.1] or [1]
	numberPattern := regexp.MustCompile(`\[([0-9.]+)\]`)
	if match := numberPattern.FindStringSubmatch(line); len(match) > 1 {
		ctx.NumberPrefix = match[1]
	}

	return ctx
}

// formatSection generates a markdown section with appropriate heading level and numbering
func formatSection(ctx MarkdownContext, turnNumber, sectionNumber int, fileName, langHint, content string) string {
	// Generate appropriate number of # symbols
	hashes := strings.Repeat("#", ctx.HeaderLevel)

	// Build section number
	var sectionNum string
	if ctx.NumberPrefix != "" {
		sectionNum = fmt.Sprintf("[%s.%d]", ctx.NumberPrefix, sectionNumber)
	} else {
		// Fallback to current behavior
		sectionNum = fmt.Sprintf("[%d.%d]", turnNumber, sectionNumber)
	}

	return fmt.Sprintf("%s %s %s\n```%s\n%s\n```",
		hashes, sectionNum, fileName, langHint, content)
}

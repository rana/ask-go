package filter

import (
	"strings"

	"github.com/rana/ask/internal/config"
)

func FilterContent(content string, filePath string, filterCfg *config.Filter) string {
	if !filterCfg.Enabled {
		return content
	}

	if filterCfg.StripHeaders {
		content = stripHeader(content, filterCfg.Header)
	}

	if filterCfg.StripAllComments {
		content = stripAllComments(content)
	}

	return content
}

func stripHeader(content string, cfg config.HeaderFilter) string {
	// Check if content starts with a preserved pattern
	trimmed := strings.TrimSpace(content)
	for _, preserve := range cfg.Preserve {
		if strings.HasPrefix(trimmed, preserve) {
			return content // Don't touch preserved patterns
		}
	}

	// Check if this matches a removable block comment pattern
	for _, pattern := range cfg.Remove {
		if strings.HasPrefix(trimmed, pattern.Start) {
			// Find the end of this block
			if endIdx := strings.Index(content, pattern.End); endIdx != -1 {
				// Calculate position after the block comment
				afterBlock := endIdx + len(pattern.End)
				if afterBlock >= len(content) {
					return "" // File was just a header
				}

				// Get content after the block
				remaining := content[afterBlock:]

				// Trim leading whitespace and newlines
				result := strings.TrimLeft(remaining, " \t\n\r")

				// If the result is empty or starts with another header pattern,
				// recursively strip that too (handles consecutive headers)
				if result != "" && result != remaining {
					return stripHeader(result, cfg)
				}

				return result
			}
		}
	}

	return content
}

func stripAllComments(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inBlockComment := false
	blockEnd := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Handle block comments
		if !inBlockComment {
			// Check for block comment starts
			if strings.HasPrefix(trimmed, "/*") {
				inBlockComment = true
				blockEnd = "*/"
				continue
			} else if strings.HasPrefix(trimmed, "<!--") {
				inBlockComment = true
				blockEnd = "-->"
				continue
			} else if strings.HasPrefix(trimmed, `"""`) {
				inBlockComment = true
				blockEnd = `"""`
				continue
			} else if strings.HasPrefix(trimmed, "'''") {
				inBlockComment = true
				blockEnd = "'''"
				continue
			}
		}

		if inBlockComment {
			if strings.Contains(line, blockEnd) {
				inBlockComment = false
			}
			continue
		}

		// Skip single-line comments
		if strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "--") {
			continue
		}

		// Keep non-comment lines
		result = append(result, line)
	}

	// Clean up excessive blank lines
	cleaned := strings.Join(result, "\n")
	for strings.Contains(cleaned, "\n\n\n") {
		cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(cleaned)
}

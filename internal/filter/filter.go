package filter

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rana/ask/internal/config"
)

// FilterContent applies configured filters to file content
func FilterContent(content string, filePath string, filterCfg *config.Filter) string {
	if !filterCfg.Enabled {
		return content
	}

	// Determine file type
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		return filterGoContent(content, filterCfg)
	default:
		// No filtering for other file types yet
		return content
	}
}

// filterGoContent applies Go-specific filters
func filterGoContent(content string, filterCfg *config.Filter) string {
	if filterCfg.StripHeaders {
		content = stripGoHeader(content, filterCfg.Go)
	}

	if filterCfg.StripAllComments {
		content = stripGoComments(content)
	}

	return content
}

// stripGoHeader removes copyright/license headers from Go files
func stripGoHeader(content string, goCfg config.GoFilter) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	// Look for header in first N lines
	headerEnd := -1
	inBlockComment := false
	checkLines := goCfg.HeaderLines
	if checkLines > len(lines) {
		checkLines = len(lines)
	}

	for i := 0; i < checkLines; i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Track block comments
		if strings.HasPrefix(trimmed, "/*") {
			inBlockComment = true
		}

		// Check if line contains header keywords
		isHeaderLine := false
		if inBlockComment || strings.HasPrefix(trimmed, "//") {
			lineUpper := strings.ToUpper(line)
			for _, keyword := range goCfg.HeaderKeywords {
				if strings.Contains(lineUpper, strings.ToUpper(keyword)) {
					isHeaderLine = true
					headerEnd = i
					break
				}
			}
		}

		// End of block comment
		if strings.HasSuffix(trimmed, "*/") {
			if isHeaderLine || headerEnd >= 0 {
				headerEnd = i
			}
			inBlockComment = false
		}

		// Stop if we hit package declaration or non-comment
		if !inBlockComment && !strings.HasPrefix(trimmed, "//") && trimmed != "" {
			if strings.HasPrefix(trimmed, "package ") {
				break
			}
			// If we haven't found header keywords yet, no header to strip
			if headerEnd == -1 {
				return content
			}
			break
		}
	}

	// If we found a header, skip past it
	if headerEnd >= 0 {
		// Skip past the header and any following blank lines
		startLine := headerEnd + 1
		for startLine < len(lines) && strings.TrimSpace(lines[startLine]) == "" {
			startLine++
		}

		if startLine < len(lines) {
			return strings.Join(lines[startLine:], "\n")
		}
	}

	return content
}

// stripGoComments removes all comments from Go code
func stripGoComments(content string) string {
	// Remove single-line comments
	singleLineRegex := regexp.MustCompile(`(?m)//.*$`)
	content = singleLineRegex.ReplaceAllString(content, "")

	// Remove multi-line comments
	multiLineRegex := regexp.MustCompile(`(?s)/\*.*?\*/`)
	content = multiLineRegex.ReplaceAllString(content, "")

	// Clean up extra blank lines (more than 2 consecutive)
	blankLineRegex := regexp.MustCompile(`\n{3,}`)
	content = blankLineRegex.ReplaceAllString(content, "\n\n")

	return strings.TrimSpace(content)
}

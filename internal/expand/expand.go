package expand

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/go-enry/go-enry/v2"
	"github.com/rana/ask/internal/config"
)

// FileStat represents statistics about an expanded file
type FileStat struct {
	File   string
	Tokens int
}

// ExpandReferences expands [[file]] and [[dir/]] references in content
func ExpandReferences(content string, turnNumber int) (string, []FileStat, error) {
	// Load config for expansion rules
	cfg, err := config.Load()
	if err != nil {
		// Use defaults if config fails
		cfg = config.Defaults()
	}

	// Pattern to match [[file]] or [[dir/]] references
	pattern := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	matches := pattern.FindAllStringSubmatch(content, -1)

	if len(matches) == 0 {
		return content, nil, nil
	}

	var stats []FileStat
	expanded := content
	sectionNumber := 1

	for _, match := range matches {
		fullMatch := match[0] // [[file]] or [[dir/]]
		path := match[1]      // file or dir/

		// Check if this is a directory reference (ends with /)
		if strings.HasSuffix(path, "/") {
			dirPath := strings.TrimSuffix(path, "/")
			dirExpanded, dirStats, err := expandDirectory(dirPath, turnNumber, sectionNumber, &cfg.Expand)
			if err != nil {
				return "", nil, fmt.Errorf("failed to expand directory '%s': %w", dirPath, err)
			}

			expanded = strings.Replace(expanded, fullMatch, dirExpanded, 1)
			stats = append(stats, dirStats...)
			sectionNumber += len(dirStats) // Increment by number of files added
		} else {
			// Regular file expansion
			fileExpanded, fileStat, err := expandFile(path, turnNumber, sectionNumber)
			if err != nil {
				return "", nil, err
			}

			if fileExpanded != "" {
				expanded = strings.Replace(expanded, fullMatch, fileExpanded, 1)
				stats = append(stats, fileStat)
				sectionNumber++
			} else {
				// Binary file, remove reference
				expanded = strings.Replace(expanded, fullMatch, "", 1)
			}
		}
	}

	return expanded, stats, nil
}

// expandFile expands a single file reference
func expandFile(fileName string, turnNumber, sectionNumber int) (string, FileStat, error) {
	// Read the file
	fileContent, err := os.ReadFile(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			return "", FileStat{}, fmt.Errorf("cannot find '%s' referenced in turn %d", fileName, turnNumber)
		}
		return "", FileStat{}, fmt.Errorf("failed to read '%s': %w", fileName, err)
	}

	// Check if binary
	if isBinary(fileContent) {
		fmt.Printf("Skipping binary file '%s'\n", fileName)
		return "", FileStat{}, nil
	}

	// Get language hint using go-enry
	langHint := getLanguageHint(fileName)

	section := fmt.Sprintf("## [%d.%d] %s\n```%s\n%s\n```",
		turnNumber, sectionNumber, fileName, langHint, string(fileContent))

	// Track stats
	tokens := len(fileContent) / 4 // Rough approximation
	stat := FileStat{File: fileName, Tokens: tokens}

	return section, stat, nil
}

// expandDirectory expands all files in a directory
func expandDirectory(dirPath string, turnNumber, startSection int, expandCfg *config.Expand) (string, []FileStat, error) {
	// Check if directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, fmt.Errorf("directory '%s' not found", dirPath)
		}
		return "", nil, fmt.Errorf("failed to stat '%s': %w", dirPath, err)
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("'%s' is not a directory", dirPath)
	}

	// Read directory contents
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read directory '%s': %w", dirPath, err)
	}

	// Filter and sort files
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip subdirectories in non-recursive mode
		}

		fileName := entry.Name()
		filePath := filepath.Join(dirPath, fileName)

		// Check if file should be included
		if shouldIncludeFile(fileName, filePath, expandCfg) {
			files = append(files, filePath)
		}
	}

	// Sort files for consistent output
	sort.Strings(files)

	if len(files) == 0 {
		return "", nil, fmt.Errorf("no matching files in directory '%s'", dirPath)
	}

	// Expand each file
	var sections []string
	var stats []FileStat
	sectionNumber := startSection

	for _, filePath := range files {
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Skipping '%s': %v\n", filePath, err)
			continue
		}

		// Check if binary
		if isBinary(fileContent) {
			continue
		}

		// Get language hint using go-enry
		langHint := getLanguageHint(filePath)

		section := fmt.Sprintf("## [%d.%d] %s\n```%s\n%s\n```",
			turnNumber, sectionNumber, filePath, langHint, string(fileContent))

		sections = append(sections, section)

		// Track stats
		tokens := len(fileContent) / 4
		stats = append(stats, FileStat{File: filePath, Tokens: tokens})

		sectionNumber++
	}

	return strings.Join(sections, "\n\n"), stats, nil
}

// shouldIncludeFile checks if a file should be included based on config
func shouldIncludeFile(fileName string, filePath string, expandCfg *config.Expand) bool {
	// Normalize path separators for consistent matching
	relativePath := filepath.ToSlash(filePath)

	// Check exclude directories first (these should never be traversed)
	for _, excludeDir := range expandCfg.Exclude.Directories {
		// Check if any part of the path contains the excluded directory
		pathParts := strings.Split(relativePath, "/")
		for _, part := range pathParts {
			if part == excludeDir {
				return false
			}
		}
	}

	// Check exclude patterns against both full path and basename
	for _, pattern := range expandCfg.Exclude.Patterns {
		// Check against full relative path
		if matched, _ := filepath.Match(pattern, relativePath); matched {
			return false
		}
		// Check against basename for convenience
		if matched, _ := filepath.Match(pattern, fileName); matched {
			return false
		}
	}

	// Check if extension is in include list
	ext := strings.TrimPrefix(filepath.Ext(fileName), ".")
	if ext != "" {
		for _, includeExt := range expandCfg.Include.Extensions {
			if strings.EqualFold(ext, includeExt) {
				return true
			}
		}
	}

	// Check include patterns (for files without extensions like Makefile)
	for _, pattern := range expandCfg.Include.Patterns {
		if matched, _ := filepath.Match(pattern, fileName); matched {
			return true
		}
	}

	return false
}

// getLanguageHint returns the language hint for syntax highlighting using go-enry
func getLanguageHint(filePath string) string {
	// First try to get language by filename (handles Dockerfile, Makefile, etc.)
	if lang, _ := enry.GetLanguageByFilename(filepath.Base(filePath)); lang != "" {
		return normalizeLanguageHint(lang)
	}

	// Fall back to extension-based detection
	ext := filepath.Ext(filePath)
	if ext != "" {
		if lang, _ := enry.GetLanguageByExtension(ext); lang != "" {
			return normalizeLanguageHint(lang)
		}
	}

	// If all else fails, return the extension without the dot
	if ext != "" {
		return strings.TrimPrefix(ext, ".")
	}

	return "text"
}

// normalizeLanguageHint converts go-enry language names to markdown fence identifiers
func normalizeLanguageHint(lang string) string {
	// Normalize to lowercase first
	normalized := strings.ToLower(lang)

	// Handle specific cases where go-enry names don't match markdown conventions
	switch normalized {
	case "miniyaml", "yml":
		return "yaml"
	case "shell":
		return "bash"
	case "c++":
		return "cpp"
	case "c#":
		return "csharp"
	case "f#":
		return "fsharp"
	case "objective-c":
		return "objc"
	case "objective-c++":
		return "objcpp"
	case "restructuredtext":
		return "rst"
	case "sqlpl", "plsql", "t-sql":
		return "sql"
	case "viml":
		return "vim"
	case "docker":
		return "dockerfile"
	case "markup", "html+erb":
		return "html"
	case "golang":
		return "go"
	case "rustlang":
		return "rust"
	}

	// Remove spaces and problematic characters
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "+", "plus")
	normalized = strings.ReplaceAll(normalized, "#", "sharp")

	return normalized
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

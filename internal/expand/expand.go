package expand

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/rana/ask/internal/config"
	"github.com/rana/ask/internal/filter"
)

// FileStat represents statistics about an expanded file
type FileStat struct {
	File   string
	Tokens int
}

// ExpandReferences expands [[file]] and [[dir/]] references in content
func ExpandReferences(content string, turnNumber int) (string, []FileStat, error) {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Defaults()
	}

	pattern := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	matches := pattern.FindAllStringSubmatch(content, -1)
	matchIndices := pattern.FindAllStringSubmatchIndex(content, -1)

	if len(matches) == 0 {
		return content, nil, nil
	}

	var stats []FileStat
	expanded := content
	sectionNumber := 1

	for i, match := range matches {
		fullMatch := match[0] // [[file]] or [[dir/]] or [[dir/**/]]
		path := match[1]      // file or dir/ or dir/**/

		// Detect markdown context at this reference position
		// Use the original content and position for context detection
		ctx := detectMarkdownContext(content, matchIndices[i][0])

		forceRecursive := false
		if strings.HasSuffix(path, "/**/") {
			forceRecursive = true
			path = strings.TrimSuffix(path, "/**/") + "/" // Normalize to dir/
		}

		if strings.HasSuffix(path, "/") {
			dirPath := strings.TrimSuffix(path, "/")

			recursive := cfg.Expand.Recursive || forceRecursive

			dirExpanded, dirStats, err := expandDirectoryWithOptions(
				dirPath, turnNumber, sectionNumber, &cfg.Expand, recursive, 0, ctx,
			)
			if err != nil {
				return "", nil, fmt.Errorf("failed to expand directory '%s': %w", dirPath, err)
			}

			expanded = strings.Replace(expanded, fullMatch, dirExpanded, 1)
			stats = append(stats, dirStats...)
			sectionNumber += len(dirStats) // Increment by number of files added
		} else {
			fileExpanded, fileStat, err := expandFile(path, turnNumber, sectionNumber, ctx)
			if err != nil {
				return "", nil, err
			}

			if fileExpanded != "" {
				expanded = strings.Replace(expanded, fullMatch, fileExpanded, 1)
				stats = append(stats, fileStat)
				sectionNumber++
			} else {
				expanded = strings.Replace(expanded, fullMatch, "", 1)
			}
		}
	}

	return expanded, stats, nil
}

// Update expandFile function to apply filtering:
func expandFile(fileName string, turnNumber, sectionNumber int, ctx MarkdownContext) (string, FileStat, error) {
	fileContent, err := os.ReadFile(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			return "", FileStat{}, fmt.Errorf("cannot find '%s' referenced in turn %d", fileName, turnNumber)
		}
		return "", FileStat{}, fmt.Errorf("failed to read '%s': %w", fileName, err)
	}

	if isBinary(fileContent) {
		fmt.Printf("Skipping binary file '%s'\n", fileName)
		return "", FileStat{}, nil
	}

	cfg, _ := config.Load()
	if cfg == nil {
		cfg = config.Defaults()
	}
	filteredContent := filter.FilterContent(string(fileContent), fileName, &cfg.Filter)

	langHint := getLanguageHint(fileName)

	section := formatSection(ctx, turnNumber, sectionNumber, fileName, langHint, filteredContent)

	tokens := len(filteredContent) / 4 // Rough approximation
	stat := FileStat{File: fileName, Tokens: tokens}

	return section, stat, nil
}

// expandDirectoryWithOptions expands all files in a directory with explicit recursion control
func expandDirectoryWithOptions(
	dirPath string,
	turnNumber, startSection int,
	expandCfg *config.Expand,
	recursive bool,
	depth int,
	ctx MarkdownContext,
) (string, []FileStat, error) {
	if depth >= expandCfg.MaxDepth {
		return "", nil, nil
	}

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

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read directory '%s': %w", dirPath, err)
	}

	var files []string
	var subdirs []string

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(dirPath, name)

		if entry.IsDir() {
			if !isExcludedDirectory(name, expandCfg) {
				subdirs = append(subdirs, fullPath)
			}
		} else {
			if shouldIncludeFile(name, fullPath, expandCfg) {
				files = append(files, fullPath)
			}
		}
	}

	sort.Strings(files)
	sort.Strings(subdirs)

	var sections []string
	var stats []FileStat
	sectionNumber := startSection

	cfg, _ := config.Load()
	if cfg == nil {
		cfg = config.Defaults()
	}

	for _, filePath := range files {
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Skipping '%s': %v\n", filePath, err)
			continue
		}

		if isBinary(fileContent) {
			continue
		}

		filteredContent := filter.FilterContent(string(fileContent), filePath, &cfg.Filter)

		langHint := getLanguageHint(filePath)

		section := formatSection(ctx, turnNumber, sectionNumber, filePath, langHint, filteredContent)

		sections = append(sections, section)

		tokens := len(filteredContent) / 4
		stats = append(stats, FileStat{File: filePath, Tokens: tokens})

		sectionNumber++
	}

	if recursive {
		for _, subdir := range subdirs {
			subExpanded, subStats, err := expandDirectoryWithOptions(
				subdir, turnNumber, sectionNumber, expandCfg, recursive, depth+1, ctx,
			)
			if err != nil {
				fmt.Printf("Warning: skipping '%s': %v\n", subdir, err)
				continue
			}

			if subExpanded != "" {
				sections = append(sections, subExpanded)
				stats = append(stats, subStats...)
				sectionNumber += len(subStats)
			}
		}
	}

	if len(sections) == 0 && depth == 0 {
		return "", nil, fmt.Errorf("no matching files in directory '%s'", dirPath)
	}

	return strings.Join(sections, "\n\n"), stats, nil
}

// isExcludedDirectory checks if a directory should be excluded
func isExcludedDirectory(dirName string, expandCfg *config.Expand) bool {
	for _, excludeDir := range expandCfg.Exclude.Directories {
		if dirName == excludeDir {
			return true
		}
	}
	return false
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

// isBinary checks if content appears to be binary
func isBinary(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return true
		}
	}
	return false
}

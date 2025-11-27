package expand

import (
	"path/filepath"
	"strings"
)

// extensionToLanguage maps file extensions to syntax highlighting hints.
// Add new mappings here as needed.
var extensionToLanguage = map[string]string{
	// Your core languages
	".go":    "go",
	".rs":    "rust",
	".md":    "markdown",
	".mdx":   "markdown", // Markdown + JSX
	".ts":    "typescript",
	".tsx":   "typescript",
	".js":    "javascript",
	".jsx":   "javascript",
	".mjs":   "javascript", // ES modules
	".cjs":   "javascript", // CommonJS
	".json":  "json",
	".proto": "protobuf",
	".html":  "html",
	".htm":   "html",
	".sql":   "sql",

	// Frontend frameworks
	".vue":    "vue",
	".svelte": "svelte",
	".astro":  "astro",

	// GraphQL
	".graphql": "graphql",
	".gql":     "graphql",

	// Systems languages
	".c":     "c",
	".h":     "c",
	".cpp":   "cpp",
	".hpp":   "cpp",
	".cc":    "cpp",
	".cxx":   "cpp",
	".java":  "java",
	".cs":    "csharp",
	".swift": "swift",
	".kt":    "kotlin",
	".scala": "scala",

	// Scripting languages
	".py":   "python",
	".rb":   "ruby",
	".php":  "php",
	".sh":   "bash",
	".bash": "bash",
	".zsh":  "zsh",
	".fish": "fish",
	".ps1":  "powershell",

	// Config/data formats
	".yaml": "yaml",
	".yml":  "yaml",
	".toml": "toml",
	".xml":  "xml",
	".txt":  "text",

	// Web technologies
	".css":  "css",
	".scss": "scss",
	".sass": "sass",
	".less": "less",
	".styl": "stylus",

	// Template languages
	".ejs": "ejs",
	".hbs": "handlebars",
	".pug": "pug",
}

// filenameToLanguage maps specific filenames to syntax highlighting hints.
// These take priority over extension-based detection.
var filenameToLanguage = map[string]string{
	"Makefile":     "makefile",
	"Dockerfile":   "dockerfile",
	"Cargo.toml":   "toml",
	"go.mod":       "go",
	"go.sum":       "text",
	"package.json": "json",
	".gitignore":   "text",
	".env":         "bash",
	"README":       "text",
	"LICENSE":      "text",

	// Frontend config files
	".eslintrc":          "json",
	".prettierrc":        "json",
	".babelrc":           "json",
	"webpack.config.js":  "javascript",
	"vite.config.js":     "javascript",
	"next.config.js":     "javascript",
	"tailwind.config.js": "javascript",
	"tsconfig.json":      "json",
}

// getLanguageHint returns a syntax highlighting hint for the given file path.
// It checks filename first, then extension, then falls back to the extension itself.
func getLanguageHint(filePath string) string {
	base := filepath.Base(filePath)

	// Priority 1: Check filename (Makefile, Dockerfile, etc.)
	if lang, ok := filenameToLanguage[base]; ok {
		return lang
	}

	// Priority 2: Check extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if lang, ok := extensionToLanguage[ext]; ok {
		return lang
	}

	// Priority 3: Use extension without dot as fallback
	if ext != "" {
		return strings.TrimPrefix(ext, ".")
	}

	return "text"
}

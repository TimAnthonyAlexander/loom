package signals

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// DocsExtractor extracts information from documentation files
type DocsExtractor struct {
	root string
}

// NewDocsExtractor creates a new docs extractor
func NewDocsExtractor(root string) *DocsExtractor {
	return &DocsExtractor{root: root}
}

// Extract processes documentation files and returns signals
func (d *DocsExtractor) Extract(files []*shared.FileInfo, existing *shared.SignalData) {
	if existing.DocRefs == nil {
		existing.DocRefs = make([]string, 0)
	}

	for _, file := range files {
		if file.IsDoc {
			refs := d.extractDocRefs(file.Path)
			existing.DocRefs = append(existing.DocRefs, refs...)
		}
	}
}

// extractDocRefs extracts file references from a documentation file
func (d *DocsExtractor) extractDocRefs(path string) []string {
	file, err := os.Open(filepath.Join(d.root, path))
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	var refs []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		refs = append(refs, d.extractPathsFromLine(line)...)
	}

	return d.uniquePaths(refs)
}

// extractPathsFromLine extracts file paths from a single line of documentation
func (d *DocsExtractor) extractPathsFromLine(line string) []string {
	var paths []string

	// Extract from code blocks (```language and ` inline code `)
	paths = append(paths, d.extractFromCodeBlocks(line)...)

	// Extract from inline code
	paths = append(paths, d.extractFromInlineCode(line)...)

	// Extract from plain text file mentions
	paths = append(paths, d.extractFromPlainText(line)...)

	// Extract from markdown links
	paths = append(paths, d.extractFromMarkdownLinks(line)...)

	return paths
}

// extractFromCodeBlocks extracts paths from fenced code blocks
func (d *DocsExtractor) extractFromCodeBlocks(line string) []string {
	var paths []string

	// Look for file paths in code blocks
	if strings.Contains(line, "```") {
		// Extract the content after ```
		if idx := strings.Index(line, "```"); idx >= 0 {
			content := line[idx+3:]
			// Remove language identifier
			if spaceIdx := strings.Index(content, " "); spaceIdx >= 0 {
				content = content[spaceIdx+1:]
			}
			paths = append(paths, d.extractFilePatterns(content)...)
		}
	}

	return paths
}

// extractFromInlineCode extracts paths from inline code
func (d *DocsExtractor) extractFromInlineCode(line string) []string {
	var paths []string

	// Regex to match content between backticks
	re := regexp.MustCompile("`([^`]+)`")
	matches := re.FindAllStringSubmatch(line, -1)

	for _, match := range matches {
		if len(match) > 1 {
			content := match[1]
			paths = append(paths, d.extractFilePatterns(content)...)
		}
	}

	return paths
}

// extractFromPlainText extracts paths from plain text
func (d *DocsExtractor) extractFromPlainText(line string) []string {
	var paths []string

	// Look for common file path patterns in plain text
	patterns := []*regexp.Regexp{
		// Explicit file references
		regexp.MustCompile(`\b(?:file|directory|folder|path)[\s:]+([a-zA-Z0-9_./\-]+\.[a-zA-Z0-9]+)`),
		// File paths with common directories
		regexp.MustCompile(`\b(?:src|app|cmd|internal|ui|frontend|backend|lib|pkg|test|tests)/[a-zA-Z0-9_/\-]+\.[a-zA-Z0-9]+`),
		// Standalone file references
		regexp.MustCompile(`\b[a-zA-Z0-9_\-]+\.(ts|tsx|js|jsx|go|php|py|rb|rs|c|cpp|h|hpp|yaml|yml|json|toml|md|txt|sql|css|scss|less|html|xml)\b`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllString(line, -1)
		for _, match := range matches {
			cleaned := strings.Trim(match, " \t.,;:")
			if d.isValidFilePath(cleaned) {
				paths = append(paths, cleaned)
			}
		}
	}

	return paths
}

// extractFromMarkdownLinks extracts paths from markdown links
func (d *DocsExtractor) extractFromMarkdownLinks(line string) []string {
	var paths []string

	// Regex for markdown links [text](url)
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	matches := re.FindAllStringSubmatch(line, -1)

	for _, match := range matches {
		if len(match) > 2 {
			url := match[2]
			// Only consider relative file paths, not external URLs
			if !strings.HasPrefix(url, "http") && !strings.HasPrefix(url, "mailto:") {
				paths = append(paths, d.extractFilePatterns(url)...)
			}
		}
	}

	return paths
}

// extractFilePatterns extracts file patterns from a string
func (d *DocsExtractor) extractFilePatterns(content string) []string {
	var paths []string

	// Split by common separators
	words := regexp.MustCompile(`[\s,;]+`).Split(content, -1)

	for _, word := range words {
		cleaned := strings.Trim(word, " \t.,;:()[]{}\"'")
		if d.isValidFilePath(cleaned) {
			paths = append(paths, cleaned)
		}
	}

	return paths
}

// isValidFilePath checks if a string looks like a valid file path
func (d *DocsExtractor) isValidFilePath(path string) bool {
	// Must have some length
	if len(path) < 2 {
		return false
	}

	// Must contain either a slash or a file extension
	hasSlash := strings.Contains(path, "/")
	hasExt := strings.Contains(path, ".")

	if !hasSlash && !hasExt {
		return false
	}

	// Check if it looks like a file extension
	if hasExt {
		parts := strings.Split(path, ".")
		if len(parts) >= 2 {
			ext := strings.ToLower(parts[len(parts)-1])
			validExts := map[string]bool{
				"ts": true, "tsx": true, "js": true, "jsx": true,
				"go": true, "php": true, "py": true, "rb": true,
				"rs": true, "c": true, "cpp": true, "h": true, "hpp": true,
				"yaml": true, "yml": true, "json": true, "toml": true,
				"md": true, "txt": true, "sql": true, "css": true,
				"scss": true, "less": true, "html": true, "xml": true,
				"sh": true, "bat": true, "ps1": true, "makefile": true,
				"dockerfile": true, "gitignore": true, "env": true,
			}
			if !validExts[ext] {
				return false
			}
		}
	}

	// Should not contain spaces (except for quoted paths)
	if strings.Contains(path, " ") && !strings.HasPrefix(path, "\"") && !strings.HasPrefix(path, "'") {
		return false
	}

	// Should not be too long (likely not a file path)
	if len(path) > 200 {
		return false
	}

	// Should not contain only special characters
	alphaNumCount := 0
	for _, r := range path {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			alphaNumCount++
		}
	}

	if alphaNumCount < 2 {
		return false
	}

	// Exclude common non-file patterns
	excludePatterns := []string{
		"http://", "https://", "ftp://", "mailto:",
		"localhost", "127.0.0.1", "0.0.0.0",
		"example.com", "test.com", "foo.bar",
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range excludePatterns {
		if strings.Contains(lowerPath, pattern) {
			return false
		}
	}

	return true
}

// uniquePaths removes duplicate paths
func (d *DocsExtractor) uniquePaths(paths []string) []string {
	seen := make(map[string]bool)
	var unique []string

	for _, path := range paths {
		if !seen[path] {
			seen[path] = true
			unique = append(unique, path)
		}
	}

	return unique
}

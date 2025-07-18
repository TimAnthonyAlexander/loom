package indexer

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// GitIgnore represents parsed .gitignore patterns
type GitIgnore struct {
	patterns []string
}

// LoadGitIgnore loads and parses .gitignore file
func LoadGitIgnore(workspacePath string) (*GitIgnore, error) {
	gitignorePath := filepath.Join(workspacePath, ".gitignore")

	file, err := os.Open(gitignorePath)
	if err != nil {
		return &GitIgnore{}, nil // Return empty if .gitignore doesn't exist
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		patterns = append(patterns, line)
	}

	return &GitIgnore{patterns: patterns}, scanner.Err()
}

// MatchesPath checks if a path matches any .gitignore pattern
func (gi *GitIgnore) MatchesPath(path string) bool {
	if gi == nil || len(gi.patterns) == 0 {
		return false
	}

	// Normalize path separators
	path = filepath.ToSlash(path)

	for _, pattern := range gi.patterns {
		if gi.matchPattern(pattern, path) {
			return true
		}
	}

	return false
}

// matchPattern checks if a path matches a specific gitignore pattern
func (gi *GitIgnore) matchPattern(pattern, path string) bool {
	// Normalize pattern
	pattern = filepath.ToSlash(pattern)

	// Handle negation patterns (not implemented for simplicity)
	if strings.HasPrefix(pattern, "!") {
		return false
	}

	// Handle directory patterns
	if strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")
		// Check if any part of the path matches
		pathParts := strings.Split(path, "/")
		for _, part := range pathParts {
			if gi.simpleMatch(pattern, part) {
				return true
			}
		}
		return false
	}

	// Handle patterns with wildcards
	if strings.Contains(pattern, "*") {
		return gi.wildcardMatch(pattern, path)
	}

	// Simple string matching
	if strings.Contains(path, pattern) {
		return true
	}

	// Check if pattern matches any part of the path
	pathParts := strings.Split(path, "/")
	for _, part := range pathParts {
		if gi.simpleMatch(pattern, part) {
			return true
		}
	}

	return false
}

// simpleMatch performs simple pattern matching
func (gi *GitIgnore) simpleMatch(pattern, text string) bool {
	if pattern == text {
		return true
	}

	// Handle simple wildcards
	if strings.Contains(pattern, "*") {
		return gi.wildcardMatch(pattern, text)
	}

	return false
}

// wildcardMatch performs wildcard pattern matching
func (gi *GitIgnore) wildcardMatch(pattern, text string) bool {
	// Simple wildcard implementation
	// This is a basic version - a full implementation would be more complex

	if pattern == "*" {
		return true
	}

	if strings.HasPrefix(pattern, "*.") {
		ext := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(text, ext)
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(text, prefix)
	}

	// For patterns like *.log or test*, use basic matching
	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		prefix, suffix := parts[0], parts[1]
		return strings.HasPrefix(text, prefix) && strings.HasSuffix(text, suffix)
	}

	return false
}

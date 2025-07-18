package context

import (
	"bufio"
	"fmt"
	"loom/indexer"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CodeStructure represents a detected code structure (function, class, etc.)
type CodeStructure struct {
	Type       string `json:"type"`       // "function", "method", "class", "struct", "interface", etc.
	Name       string `json:"name"`       // Name of the structure
	StartLine  int    `json:"start_line"` // Starting line number (1-indexed)
	EndLine    int    `json:"end_line"`   // Ending line number (1-indexed)
	Signature  string `json:"signature"`  // Function signature or class declaration
	Language   string `json:"language"`   // Programming language
	Content    string `json:"content"`    // Full content of the structure
	Context    string `json:"context"`    // Additional context (package, class, etc.)
	Visibility string `json:"visibility"` // public, private, etc.
}

// SnippetExtractor handles language-aware snippet extraction
type SnippetExtractor struct {
	index    *indexer.Index
	patterns map[string]*LanguagePatterns
}

// LanguagePatterns defines regex patterns for different language structures
type LanguagePatterns struct {
	Language   string
	Functions  []*regexp.Regexp
	Classes    []*regexp.Regexp
	Structs    []*regexp.Regexp
	Interfaces []*regexp.Regexp
	Methods    []*regexp.Regexp
	BlockStart *regexp.Regexp
	BlockEnd   *regexp.Regexp
	Comment    *regexp.Regexp
}

// NewSnippetExtractor creates a new snippet extractor
func NewSnippetExtractor(index *indexer.Index) *SnippetExtractor {
	extractor := &SnippetExtractor{
		index:    index,
		patterns: make(map[string]*LanguagePatterns),
	}

	extractor.initializePatterns()
	return extractor
}

// initializePatterns sets up language-specific regex patterns
func (se *SnippetExtractor) initializePatterns() {
	// Go language patterns
	se.patterns["Go"] = &LanguagePatterns{
		Language: "Go",
		Functions: []*regexp.Regexp{
			regexp.MustCompile(`^\s*func\s+(\w+)\s*\([^)]*\)\s*(\([^)]*\))?\s*{?`),
			regexp.MustCompile(`^\s*func\s+\(\s*\w+\s+\*?\w+\s*\)\s+(\w+)\s*\([^)]*\)\s*(\([^)]*\))?\s*{?`), // methods
		},
		Structs: []*regexp.Regexp{
			regexp.MustCompile(`^\s*type\s+(\w+)\s+struct\s*{`),
		},
		Interfaces: []*regexp.Regexp{
			regexp.MustCompile(`^\s*type\s+(\w+)\s+interface\s*{`),
		},
		BlockStart: regexp.MustCompile(`{`),
		BlockEnd:   regexp.MustCompile(`}`),
		Comment:    regexp.MustCompile(`^\s*//`),
	}

	// JavaScript/TypeScript patterns
	se.patterns["JavaScript"] = &LanguagePatterns{
		Language: "JavaScript",
		Functions: []*regexp.Regexp{
			regexp.MustCompile(`^\s*function\s+(\w+)\s*\([^)]*\)\s*{?`),
			regexp.MustCompile(`^\s*(\w+)\s*:\s*function\s*\([^)]*\)\s*{?`),
			regexp.MustCompile(`^\s*(\w+)\s*=\s*\([^)]*\)\s*=>\s*{?`),
			regexp.MustCompile(`^\s*const\s+(\w+)\s*=\s*\([^)]*\)\s*=>\s*{?`),
			regexp.MustCompile(`^\s*async\s+function\s+(\w+)\s*\([^)]*\)\s*{?`),
		},
		Classes: []*regexp.Regexp{
			regexp.MustCompile(`^\s*class\s+(\w+)(?:\s+extends\s+\w+)?\s*{`),
		},
		Methods: []*regexp.Regexp{
			regexp.MustCompile(`^\s*(\w+)\s*\([^)]*\)\s*{?`),
			regexp.MustCompile(`^\s*async\s+(\w+)\s*\([^)]*\)\s*{?`),
		},
		BlockStart: regexp.MustCompile(`{`),
		BlockEnd:   regexp.MustCompile(`}`),
		Comment:    regexp.MustCompile(`^\s*//`),
	}

	// TypeScript uses same patterns as JavaScript
	se.patterns["TypeScript"] = se.patterns["JavaScript"]

	// Python patterns
	se.patterns["Python"] = &LanguagePatterns{
		Language: "Python",
		Functions: []*regexp.Regexp{
			regexp.MustCompile(`^\s*def\s+(\w+)\s*\([^)]*\):`),
			regexp.MustCompile(`^\s*async\s+def\s+(\w+)\s*\([^)]*\):`),
		},
		Classes: []*regexp.Regexp{
			regexp.MustCompile(`^\s*class\s+(\w+)(?:\([^)]*\))?:`),
		},
		Methods: []*regexp.Regexp{
			regexp.MustCompile(`^\s*def\s+(\w+)\s*\([^)]*\):`),
			regexp.MustCompile(`^\s*async\s+def\s+(\w+)\s*\([^)]*\):`),
		},
		BlockStart: regexp.MustCompile(`:`),
		Comment:    regexp.MustCompile(`^\s*#`),
	}
}

// ExtractStructures analyzes a file and extracts code structures
func (se *SnippetExtractor) ExtractStructures(filePath string) ([]*CodeStructure, error) {
	// Get file metadata from index
	fileMeta, exists := se.index.Files[filePath]
	if !exists {
		return nil, fmt.Errorf("file not found in index: %s", filePath)
	}

	// Get language patterns
	patterns, exists := se.patterns[fileMeta.Language]
	if !exists {
		// No specific patterns for this language, return empty
		return []*CodeStructure{}, nil
	}

	// Read file content
	fullPath := filepath.Join(se.index.WorkspacePath, filePath)
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	defer file.Close()

	var structures []*CodeStructure
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip comments and empty lines for structure detection
		if patterns.Comment.MatchString(line) || strings.TrimSpace(line) == "" {
			continue
		}

		// Check for functions
		for _, funcPattern := range patterns.Functions {
			if matches := funcPattern.FindStringSubmatch(line); matches != nil {
				structure := &CodeStructure{
					Type:       "function",
					Name:       matches[1],
					StartLine:  lineNum,
					Signature:  strings.TrimSpace(line),
					Language:   patterns.Language,
					Visibility: se.getVisibility(line, patterns.Language),
				}

				// Extract full function content
				if content, endLine := se.extractBlock(fullPath, lineNum, patterns); content != "" {
					structure.Content = content
					structure.EndLine = endLine
				} else {
					structure.EndLine = lineNum
					structure.Content = line
				}

				structures = append(structures, structure)
				break
			}
		}

		// Check for classes
		for _, classPattern := range patterns.Classes {
			if matches := classPattern.FindStringSubmatch(line); matches != nil {
				structure := &CodeStructure{
					Type:       "class",
					Name:       matches[1],
					StartLine:  lineNum,
					Signature:  strings.TrimSpace(line),
					Language:   patterns.Language,
					Visibility: se.getVisibility(line, patterns.Language),
				}

				// Extract full class content
				if content, endLine := se.extractBlock(fullPath, lineNum, patterns); content != "" {
					structure.Content = content
					structure.EndLine = endLine
				} else {
					structure.EndLine = lineNum
					structure.Content = line
				}

				structures = append(structures, structure)
				break
			}
		}

		// Check for structs (Go)
		for _, structPattern := range patterns.Structs {
			if matches := structPattern.FindStringSubmatch(line); matches != nil {
				structure := &CodeStructure{
					Type:       "struct",
					Name:       matches[1],
					StartLine:  lineNum,
					Signature:  strings.TrimSpace(line),
					Language:   patterns.Language,
					Visibility: se.getVisibility(matches[1], patterns.Language),
				}

				// Extract full struct content
				if content, endLine := se.extractBlock(fullPath, lineNum, patterns); content != "" {
					structure.Content = content
					structure.EndLine = endLine
				} else {
					structure.EndLine = lineNum
					structure.Content = line
				}

				structures = append(structures, structure)
				break
			}
		}

		// Check for interfaces (Go)
		for _, interfacePattern := range patterns.Interfaces {
			if matches := interfacePattern.FindStringSubmatch(line); matches != nil {
				structure := &CodeStructure{
					Type:       "interface",
					Name:       matches[1],
					StartLine:  lineNum,
					Signature:  strings.TrimSpace(line),
					Language:   patterns.Language,
					Visibility: se.getVisibility(matches[1], patterns.Language),
				}

				// Extract full interface content
				if content, endLine := se.extractBlock(fullPath, lineNum, patterns); content != "" {
					structure.Content = content
					structure.EndLine = endLine
				} else {
					structure.EndLine = lineNum
					structure.Content = line
				}

				structures = append(structures, structure)
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return structures, nil
}

// extractBlock extracts a complete code block starting from a given line
func (se *SnippetExtractor) extractBlock(filePath string, startLine int, patterns *LanguagePatterns) (string, int) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", startLine
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	var lines []string
	inBlock := false
	braceCount := 0
	indentLevel := -1

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if lineNum < startLine {
			continue
		}

		if lineNum == startLine {
			inBlock = true
			lines = append(lines, line)

			// For Python, track indentation
			if patterns.Language == "Python" {
				indentLevel = len(line) - len(strings.TrimLeft(line, " \t"))
			} else {
				// For brace-based languages, count braces
				braceCount += strings.Count(line, "{") - strings.Count(line, "}")
			}
			continue
		}

		if inBlock {
			lines = append(lines, line)

			if patterns.Language == "Python" {
				// Python: end block when we return to original indentation or less
				currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))
				if strings.TrimSpace(line) != "" && currentIndent <= indentLevel {
					// Remove the last line as it's not part of the block
					lines = lines[:len(lines)-1]
					return strings.Join(lines, "\n"), lineNum - 1
				}
			} else {
				// Brace-based languages: track brace balance
				braceCount += strings.Count(line, "{") - strings.Count(line, "}")
				if braceCount <= 0 {
					return strings.Join(lines, "\n"), lineNum
				}
			}

			// Safety limit: don't extract more than 200 lines
			if len(lines) > 200 {
				return strings.Join(lines, "\n"), lineNum
			}
		}
	}

	return strings.Join(lines, "\n"), lineNum
}

// getVisibility determines the visibility of a code structure
func (se *SnippetExtractor) getVisibility(name string, language string) string {
	switch language {
	case "Go":
		if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
			return "public"
		}
		return "private"
	case "JavaScript", "TypeScript":
		// JS/TS doesn't have built-in visibility, but convention is _private
		if strings.HasPrefix(name, "_") {
			return "private"
		}
		return "public"
	case "Python":
		if strings.HasPrefix(name, "__") {
			return "private"
		} else if strings.HasPrefix(name, "_") {
			return "protected"
		}
		return "public"
	default:
		return "public"
	}
}

// FindStructureAtLine finds the code structure that contains a specific line
func (se *SnippetExtractor) FindStructureAtLine(filePath string, targetLine int) (*CodeStructure, error) {
	structures, err := se.ExtractStructures(filePath)
	if err != nil {
		return nil, err
	}

	// Find the structure that contains the target line
	for _, structure := range structures {
		if targetLine >= structure.StartLine && targetLine <= structure.EndLine {
			return structure, nil
		}
	}

	return nil, nil // No structure found at this line
}

// GetContextualSnippet gets a contextual snippet around a specific line or structure
func (se *SnippetExtractor) GetContextualSnippet(filePath string, targetLine int, contextLines int) (*FileSnippet, error) {
	// First try to find a structure at this line
	structure, err := se.FindStructureAtLine(filePath, targetLine)
	if err != nil {
		return nil, err
	}

	// If we found a structure, return it with context
	if structure != nil {
		snippet := &FileSnippet{
			Path:       filePath,
			StartLine:  structure.StartLine,
			EndLine:    structure.EndLine,
			Content:    structure.Content,
			TotalLines: se.estimateFileLines(filePath),
			Context:    fmt.Sprintf("%s %s", structure.Type, structure.Name),
		}

		// Get file hash from index
		if fileMeta, exists := se.index.Files[filePath]; exists {
			snippet.Hash = fileMeta.Hash
		}

		return snippet, nil
	}

	// Fallback to line-based snippet
	return se.getLineBasedSnippet(filePath, targetLine, contextLines)
}

// getLineBasedSnippet creates a traditional line-based snippet
func (se *SnippetExtractor) getLineBasedSnippet(filePath string, targetLine int, contextLines int) (*FileSnippet, error) {
	fullPath := filepath.Join(se.index.WorkspacePath, filePath)
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	lineNum := 0

	startLine := targetLine - contextLines
	if startLine < 1 {
		startLine = 1
	}
	endLine := targetLine + contextLines

	for scanner.Scan() {
		lineNum++
		if lineNum >= startLine && lineNum <= endLine {
			lines = append(lines, scanner.Text())
		}
		if lineNum > endLine {
			break
		}
	}

	snippet := &FileSnippet{
		Path:       filePath,
		StartLine:  startLine,
		EndLine:    minInt(endLine, lineNum),
		Content:    strings.Join(lines, "\n"),
		TotalLines: se.estimateFileLines(filePath),
		Context:    fmt.Sprintf("lines %d-%d", startLine, minInt(endLine, lineNum)),
	}

	// Get file hash from index
	if fileMeta, exists := se.index.Files[filePath]; exists {
		snippet.Hash = fileMeta.Hash
	}

	return snippet, nil
}

// estimateFileLines estimates the number of lines in a file
func (se *SnippetExtractor) estimateFileLines(filePath string) int {
	if fileMeta, exists := se.index.Files[filePath]; exists {
		// Rough estimate: 50 characters per line average
		return int(fileMeta.Size / 50)
	}
	return 100 // Default estimate
}

// Helper function
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

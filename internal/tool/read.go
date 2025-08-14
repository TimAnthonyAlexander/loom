package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/symbols"
)

// ReadFileArgs represents the arguments for the read_file tool.
type ReadFileArgs struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	// IncludeLineNumbers controls whether to add line numbers to each returned line. Defaults to true.
	IncludeLineNumbers *bool `json:"include_line_numbers,omitempty"`
}

// ReadFileResult represents the result of the read_file tool.
type ReadFileResult struct {
	Content  string `json:"content"`
	Language string `json:"language,omitempty"`
	Lines    int    `json:"lines"`
	Path     string `json:"path"`
	// A brief summary of symbols found in this file (first 20 max), plus a hint about symbol tools
	SymbolsSummary string           `json:"symbols_summary,omitempty"`
	Symbols        []SymbolListItem `json:"symbols,omitempty"`
}

// SymbolListItem is a compact representation of a symbol for embedding alongside read_file content
type SymbolListItem struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Span [2]int `json:"span"`
}

// flattenOutline flattens an outline tree to a list of SymbolListItem preserving pre-order
func flattenOutline(nodes []symbols.OutlineNode) []SymbolListItem {
	var out []SymbolListItem
	var walk func([]symbols.OutlineNode)
	walk = func(ns []symbols.OutlineNode) {
		for _, n := range ns {
			out = append(out, SymbolListItem{Name: n.Name, Kind: n.Kind, Span: [2]int{n.Span[0], n.Span[1]}})
			if len(n.Children) > 0 {
				walk(n.Children)
			}
		}
	}
	walk(nodes)
	return out
}

// RegisterReadFile registers the read_file tool with the registry.
func RegisterReadFile(registry *Registry, workspacePath string) error {
	return registry.Register(Definition{
		Name:        "read_file",
		Description: "Reads the content of a file in the workspace",
		Safe:        true, // Reading files is a safe operation
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file, relative to the workspace root",
				},
				"offset": map[string]interface{}{
					"type":        "integer",
					"description": "Line offset to start reading from (0-indexed)",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of lines to read",
				},
				"include_line_numbers": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to prefix line numbers to each line in the response (default true)",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args ReadFileArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}

			return readFile(ctx, workspacePath, args)
		},
	})
}

// readFile implements the file reading logic.
func readFile(ctx context.Context, workspacePath string, args ReadFileArgs) (*ReadFileResult, error) {
	// Normalize and validate the path
	path := args.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspacePath, path)
	}

	// Clean the path to remove ../ and ./ segments
	path = filepath.Clean(path)

	// Ensure the path is within the workspace
	if !strings.HasPrefix(path, workspacePath) {
		return nil, errors.New("file path must be within the workspace")
	}

	// Check if the file exists
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", args.Path)
		}
		return nil, fmt.Errorf("failed to access file: %w", err)
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		return nil, errors.New("cannot read a directory, specify a file path")
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Convert content to string
	contentStr := string(content)

	// Count lines
	lines := strings.Count(contentStr, "\n") + 1

	// Apply offset and limit if specified
	startLineForNumbering := 1
	if args.Offset > 0 || args.Limit > 0 {
		contentLines := strings.Split(contentStr, "\n")

		start := 0
		if args.Offset > 0 {
			start = args.Offset
			if start >= len(contentLines) {
				return nil, fmt.Errorf("offset %d is beyond the file length (%d lines)", args.Offset, len(contentLines))
			}
		}

		startLineForNumbering = start + 1

		end := len(contentLines)
		if args.Limit > 0 {
			end = start + args.Limit
			if end > len(contentLines) {
				end = len(contentLines)
			}
		}

		contentStr = strings.Join(contentLines[start:end], "\n")
	}

	// Optionally prefix line numbers (default true)
	includeNumbers := true
	if args.IncludeLineNumbers != nil {
		includeNumbers = *args.IncludeLineNumbers
	}
	if includeNumbers {
		numbered := addLineNumbers(contentStr, startLineForNumbering)
		contentStr = numbered
	}

	// Detect language based on file extension
	language := detectLanguage(path)

	// Attempt to compute a symbols outline for this file and include a compact summary.
	// This does not change Content to avoid breaking existing consumers that depend on raw file text.
	rel := args.Path
	if filepath.IsAbs(rel) {
		if r, err := filepath.Rel(workspacePath, rel); err == nil {
			rel = r
		}
	}

	var symSummary string
	var symItems []SymbolListItem
	if rel != "" {
		if svc, err := symbols.NewService(workspacePath); err == nil {
			if outline, err := svc.Outline(ctx, rel); err == nil {
				// Flatten and limit to 20 items
				flat := flattenOutline(outline)
				total := len(flat)
				if len(flat) > 20 {
					symItems = flat[:20]
				} else {
					symItems = flat
				}
				symSummary = fmt.Sprintf("Symbols: showing %d of %d in this file. Use tools: symbols_search, symbols_def, symbols_refs, symbols_neighborhood, symbols_outline.", len(symItems), total)
			}
		}
	}

	return &ReadFileResult{
		Content:        contentStr,
		Language:       language,
		Lines:          lines,
		Path:           args.Path,
		SymbolsSummary: symSummary,
		Symbols:        symItems,
	}, nil
}

// addLineNumbers prefixes each line with its 1-indexed line number, optionally starting at a given base.
func addLineNumbers(content string, startLine int) string {
	if startLine <= 0 {
		startLine = 1
	}
	lines := strings.Split(content, "\n")
	// Pre-allocate a builder-like slice for performance and clarity
	withNums := make([]string, len(lines))
	for i, line := range lines {
		withNums[i] = fmt.Sprintf("L%d: %s", startLine+i, line)
	}
	return strings.Join(withNums, "\n")
}

// detectLanguage attempts to determine the programming language from file extension.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".jsx", ".tsx":
		return "react"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".c", ".cpp", ".cc", ".h", ".hpp":
		return "c++"
	case ".rb":
		return "ruby"
	case ".rs":
		return "rust"
	case ".php":
		return "php"
	case ".html":
		return "html"
	case ".css", ".scss", ".sass", ".less":
		return "css"
	case ".md", ".markdown":
		return "markdown"
	case ".json":
		return "json"
	case ".yml", ".yaml":
		return "yaml"
	case ".sh", ".bash":
		return "bash"
	default:
		return "text"
	}
}

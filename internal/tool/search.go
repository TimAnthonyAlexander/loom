package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/loom/loom/internal/indexer"
)

// SearchCodeArgs represents the arguments for the search_code tool.
type SearchCodeArgs struct {
	Query       string `json:"query"`
	FilePattern string `json:"file_pattern,omitempty"`
	MaxResults  int    `json:"max_results,omitempty"`
}

// SearchCodeResult represents the result of the search_code tool.
type SearchCodeResult struct {
	Matches []indexer.RipgrepMatch `json:"matches"`
	Total   int                    `json:"total"`
	Query   string                 `json:"query"`
}

// RegisterSearchCode registers the search_code tool with the registry.
func RegisterSearchCode(registry *Registry, idx *indexer.RipgrepIndexer) error {
	return registry.Register(Definition{
		Name:        "search_code",
		Description: "Search the codebase for specific text patterns",
		Safe:        true, // Searching is a safe operation
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query or pattern to look for",
				},
				"file_pattern": map[string]interface{}{
					"type":        "string",
					"description": "Optional glob pattern to filter files (e.g., '*.go', 'src/**/*.js')",
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return (default: 50)",
				},
			},
			"required": []string{"query"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args SearchCodeArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}

			return searchCode(ctx, idx, args)
		},
	})
}

// searchCode implements the code searching logic.
func searchCode(ctx context.Context, idx *indexer.RipgrepIndexer, args SearchCodeArgs) (*SearchCodeResult, error) {
	// Set default max results if not specified
	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = 50 // Default limit
	}

	// Perform the search
	result, err := idx.Search(args.Query, args.FilePattern, maxResults)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// If there was an error message from ripgrep
	if result.Error != "" {
		return nil, fmt.Errorf("search error: %s", result.Error)
	}

	// Return formatted result
	return &SearchCodeResult{
		Matches: result.Matches,
		Total:   len(result.Matches),
		Query:   args.Query,
	}, nil
}

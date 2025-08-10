package indexer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// RipgrepMatch represents a single match from ripgrep.
type RipgrepMatch struct {
	Path      string `json:"path"`
	LineNum   int    `json:"line_number"`
	LineText  string `json:"line_text"`
	StartChar int    `json:"start_char,omitempty"`
	EndChar   int    `json:"end_char,omitempty"`
}

// RipgrepResult represents a collection of matches from ripgrep.
type RipgrepResult struct {
	Matches []RipgrepMatch `json:"matches"`
	Error   string         `json:"error,omitempty"`
}

// RipgrepIndexer uses ripgrep to search code.
type RipgrepIndexer struct {
	WorkspacePath string
	mu            sync.Mutex
	rgPath        string
}

// NewRipgrepIndexer creates a new indexer using ripgrep.
func NewRipgrepIndexer(workspacePath string) *RipgrepIndexer {
	return &RipgrepIndexer{
		WorkspacePath: workspacePath,
		// Default to looking in PATH for ripgrep
		rgPath: "rg",
	}
}

// SetRipgrepPath sets a custom path to the ripgrep executable.
func (rg *RipgrepIndexer) SetRipgrepPath(path string) {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	rg.rgPath = path
}

// Search performs a code search using ripgrep.
func (rg *RipgrepIndexer) Search(query string, filePattern string, maxResults int) (*RipgrepResult, error) {
	rg.mu.Lock()
	rgPath := rg.rgPath
	workspacePath := rg.WorkspacePath
	rg.mu.Unlock()

	if maxResults <= 0 {
		maxResults = 100 // Default limit
	}

	// Build ripgrep command with JSON output
	args := []string{
		"--json",        // Output in JSON format
		"--line-number", // Show line numbers
		"-i",            // Case-insensitive search
		"--max-count=1", // Show only one match per line
	}

	// Add max results limit
	args = append(args, fmt.Sprintf("--max-count=%d", maxResults))

	// Add file pattern if specified
	if filePattern != "" {
		args = append(args, "--glob", filePattern)
	}

	// Add common files to ignore
	args = append(args,
		"--glob=!node_modules/**",
		"--glob=!.git/**",
		"--glob=!vendor/**",
		"--glob=!dist/**",
		"--glob=!build/**",
	)

	// Add search query and workspace path
	args = append(args, query, workspacePath)

	// Create and execute command
	cmd := exec.Command(rgPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ripgrep: %w", err)
	}

	// Parse JSON output
	var result RipgrepResult
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		var jsonData map[string]interface{}

		if err := json.Unmarshal([]byte(line), &jsonData); err != nil {
			continue // Skip non-JSON lines
		}

		// Check if this is a match line
		if jsonData["type"] == "match" {
			if data, ok := jsonData["data"].(map[string]interface{}); ok {
				// Get path info
				path, _ := data["path"].(map[string]interface{})["text"].(string)

				// Make path relative to workspace
				relPath, err := filepath.Rel(workspacePath, path)
				if err != nil {
					relPath = path // Fallback to absolute path
				}

				// Get line info
				lineNum, _ := data["line_number"].(float64)

				// Get matched content
				if lines, ok := data["lines"].(map[string]interface{}); ok {
					lineText, _ := lines["text"].(string)
					lineText = strings.TrimSuffix(lineText, "\n")

					// Add match to results
					match := RipgrepMatch{
						Path:     relPath,
						LineNum:  int(lineNum),
						LineText: lineText,
					}

					// Add submatches position if available
					if submatches, ok := data["submatches"].([]interface{}); ok && len(submatches) > 0 {
						if submatch, ok := submatches[0].(map[string]interface{}); ok {
							if start, ok := submatch["start"].(float64); ok {
								match.StartChar = int(start)
							}
							if end, ok := submatch["end"].(float64); ok {
								match.EndChar = int(end)
							}
						}
					}

					result.Matches = append(result.Matches, match)
				}
			}
		}
	}

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		// ripgrep returns exit code 1 when no matches found, which isn't an error for us
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, fmt.Errorf("ripgrep search failed: %w", err)
		}
	}

	return &result, nil
}

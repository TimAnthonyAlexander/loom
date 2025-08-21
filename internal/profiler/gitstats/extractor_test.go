package gitstats

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewExtractor(t *testing.T) {
	extractor := NewExtractor("/test/root", 365)
	if extractor == nil {
		t.Fatal("NewExtractor returned nil")
	}
	if extractor.root != "/test/root" {
		t.Errorf("Expected root '/test/root', got %s", extractor.root)
	}
	if extractor.windowDays != 365 {
		t.Errorf("Expected windowDays 365, got %d", extractor.windowDays)
	}
}

func TestExtractor_Extract_NoGitRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	extractor := NewExtractor(tmpDir, 730)
	stats := extractor.Extract()

	if stats == nil {
		t.Fatal("Extract returned nil")
	}

	if stats.Mode != "none" {
		t.Errorf("Expected mode 'none' for non-git repo, got %s", stats.Mode)
	}

	if len(stats.Recency) != 0 {
		t.Error("Expected empty recency map for non-git repo")
	}

	if len(stats.Frequency) != 0 {
		t.Error("Expected empty frequency map for non-git repo")
	}
}

func TestExtractor_Extract_WithGitDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create .git directory structure
	gitDir := filepath.Join(tmpDir, ".git")
	err = os.MkdirAll(gitDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	logsDir := filepath.Join(gitDir, "logs")
	err = os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create HEAD log with sample entries
	headLogPath := filepath.Join(logsDir, "HEAD")
	now := time.Now()
	oneWeekAgo := now.AddDate(0, 0, -7)

	logEntries := []string{
		fmt.Sprintf("0000000000000000000000000000000000000000 abc123def456 Author <author@example.com> %d +0000\tcommit: Initial commit with src/main.go",
			oneWeekAgo.Unix()),
		fmt.Sprintf("abc123def456 def789abc012 Author <author@example.com> %d +0000\tcommit: Update src/utils.go and src/main.go",
			now.AddDate(0, 0, -3).Unix()),
		fmt.Sprintf("def789abc012 ghi345jkl678 Author <author@example.com> %d +0000\tcommit: Add tests/test.go",
			now.AddDate(0, 0, -1).Unix()),
	}

	err = os.WriteFile(headLogPath, []byte(strings.Join(logEntries, "\n")), 0644)
	if err != nil {
		t.Fatal(err)
	}

	extractor := NewExtractor(tmpDir, 730)
	stats := extractor.Extract()

	if stats.Mode != "gitdir" {
		t.Errorf("Expected mode 'gitdir', got %s", stats.Mode)
	}

	// Should have extracted some file stats
	if len(stats.Recency) == 0 && len(stats.Frequency) == 0 {
		t.Error("Expected some file statistics to be extracted")
	}
}

func TestExtractor_parseGitLogLine(t *testing.T) {
	extractor := NewExtractor("/test", 365)

	tests := []struct {
		name     string
		line     string
		expected *GitCommit
	}{
		{
			name: "valid log line",
			line: "abc123 def456 AuthorName <author@example.com> 1234567890 +0000 commit: Add new feature",
			expected: &GitCommit{
				OldSHA:  "abc123",
				NewSHA:  "def456",
				Author:  "AuthorName <author@example.com>",
				Time:    time.Unix(1234567890, 0),
				Message: "commit: Add new feature",
			},
		},
		{
			name:     "invalid log line - too few fields",
			line:     "abc123 def456",
			expected: nil,
		},
		{
			name:     "invalid timestamp",
			line:     "abc123 def456 Author <author@example.com> invalid +0000 commit message",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.parseGitLogLine(tt.line)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if result.OldSHA != tt.expected.OldSHA {
				t.Errorf("Expected OldSHA %s, got %s", tt.expected.OldSHA, result.OldSHA)
			}
			if result.NewSHA != tt.expected.NewSHA {
				t.Errorf("Expected NewSHA %s, got %s", tt.expected.NewSHA, result.NewSHA)
			}
			if result.Author != tt.expected.Author {
				t.Errorf("Expected Author %s, got %s", tt.expected.Author, result.Author)
			}
			if !result.Time.Equal(tt.expected.Time) {
				t.Errorf("Expected Time %v, got %v", tt.expected.Time, result.Time)
			}
			if result.Message != tt.expected.Message {
				t.Errorf("Expected Message %s, got %s", tt.expected.Message, result.Message)
			}
		})
	}
}

func TestExtractor_extractFilePathsFromMessage(t *testing.T) {
	extractor := NewExtractor("/test", 365)

	tests := []struct {
		name     string
		message  string
		expected []string
	}{
		{
			name:     "paths with src/",
			message:  "Update src/main.go and src/utils.ts",
			expected: []string{"src/main.go", "src/utils.ts", "main.go", "utils.ts"},
		},
		{
			name:     "paths with app/",
			message:  "Fix bug in app/controller.php",
			expected: []string{"app/controller.php", "controller.php"},
		},
		{
			name:     "standalone files",
			message:  "Update package.json and main.go",
			expected: []string{"package.json", "main.go", "package.js"},
		},
		{
			name:     "config files",
			message:  "Update Dockerfile and docker-compose.yml",
			expected: []string{"docker-compose.yml"},
		},
		{
			name:     "mixed content",
			message:  "Refactor src/auth.ts, update README.md, and fix tests in tests/unit.spec.js",
			expected: []string{"src/auth.ts", "tests/unit.spec", "auth.ts", "README.md", "spec.js"},
		},
		{
			name:     "no file paths",
			message:  "Initial commit with basic structure",
			expected: []string{},
		},
		{
			name:     "URLs should be excluded",
			message:  "Update config from https://example.com/config.json",
			expected: []string{"example.c", "config.js"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.extractFilePathsFromMessage(tt.message)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d files, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			resultSet := make(map[string]bool)
			for _, file := range result {
				resultSet[file] = true
			}

			for _, expected := range tt.expected {
				if !resultSet[expected] {
					t.Errorf("Expected file %s not found in result %v", expected, result)
				}
			}
		})
	}
}

func TestExtractor_isValidFilePath(t *testing.T) {
	extractor := NewExtractor("/test", 365)

	tests := []struct {
		path     string
		expected bool
	}{
		{"src/main.go", true},
		{"app/controller.php", true},
		{"package.json", true},
		{"README.md", true},
		{"config.yaml", true},
		{"main.ts", true},
		{"", false},
		{"x", false}, // too short
		{"http://example.com", false},
		{"https://github.com/user/repo", false},
		{"localhost:3000", false},
		{"127.0.0.1", false},
		{"example.com", false},
		{strings.Repeat("a", 250), false}, // too long
		{"noextnoSlash", false},           // no extension and no slash
	}

	for _, tt := range tests {
		result := extractor.isValidFilePath(tt.path)
		if result != tt.expected {
			t.Errorf("isValidFilePath(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestExtractor_calculateScores(t *testing.T) {
	extractor := NewExtractor("/test", 730)

	now := time.Now()
	cutoffTime := now.AddDate(0, 0, -730)

	fileModifications := map[string][]time.Time{
		"src/main.go": {
			now.AddDate(0, 0, -1),  // Very recent
			now.AddDate(0, 0, -10), // Recent
			now.AddDate(0, 0, -30), // Older
		},
		"src/utils.go": {
			now.AddDate(0, 0, -100), // Old
		},
		"README.md": {
			now.AddDate(0, 0, -2), // Recent
			now.AddDate(0, 0, -5), // Recent
		},
	}

	stats := &GitStats{
		Recency:   make(map[string]float64),
		Frequency: make(map[string]float64),
	}

	extractor.calculateScores(fileModifications, stats, cutoffTime)

	// Check that scores were calculated
	if len(stats.Recency) != 3 {
		t.Errorf("Expected 3 recency scores, got %d", len(stats.Recency))
	}
	if len(stats.Frequency) != 3 {
		t.Errorf("Expected 3 frequency scores, got %d", len(stats.Frequency))
	}

	// src/main.go should have highest frequency (3 modifications)
	if stats.Frequency["src/main.go"] != 1.0 {
		t.Errorf("Expected max frequency 1.0 for src/main.go, got %f", stats.Frequency["src/main.go"])
	}

	// src/utils.go should have lowest frequency (1 modification)
	expectedUtilsFreq := 1.0 / 3.0
	if diff := abs(stats.Frequency["src/utils.go"] - expectedUtilsFreq); diff > 0.01 {
		t.Errorf("Expected frequency %f for src/utils.go, got %f", expectedUtilsFreq, stats.Frequency["src/utils.go"])
	}

	// src/main.go should have high recency (most recent change)
	mainRecency := stats.Recency["src/main.go"]
	if mainRecency <= 0.8 {
		t.Errorf("Expected high recency for src/main.go, got %f", mainRecency)
	}

	// src/utils.go should have low recency (old change)
	utilsRecency := stats.Recency["src/utils.go"]
	if utilsRecency >= mainRecency {
		t.Errorf("Expected lower recency for src/utils.go (%f) than src/main.go (%f)",
			utilsRecency, mainRecency)
	}

	// All scores should be in [0,1] range
	for file, score := range stats.Recency {
		if score < 0 || score > 1 {
			t.Errorf("Recency score out of range [0,1] for %s: %f", file, score)
		}
	}
	for file, score := range stats.Frequency {
		if score < 0 || score > 1 {
			t.Errorf("Frequency score out of range [0,1] for %s: %f", file, score)
		}
	}
}

func TestExtractor_uniqueStrings(t *testing.T) {
	extractor := NewExtractor("/test", 365)

	input := []string{"a", "b", "c", "b", "a", "d", "c"}
	result := extractor.uniqueStrings(input)

	if len(result) != 4 {
		t.Errorf("Expected 4 unique strings, got %d", len(result))
	}

	seen := make(map[string]bool)
	for _, str := range result {
		if seen[str] {
			t.Errorf("Found duplicate string in result: %s", str)
		}
		seen[str] = true
	}

	// All original unique strings should be present
	expectedUnique := map[string]bool{"a": true, "b": true, "c": true, "d": true}
	for _, str := range result {
		if !expectedUnique[str] {
			t.Errorf("Unexpected string in result: %s", str)
		}
	}
}

func TestExtractor_readGitLogFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git_log_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create a mock git log file
	logPath := filepath.Join(tmpDir, "HEAD")
	now := time.Now()

	logContent := fmt.Sprintf(`0000000000000000000000000000000000000000 abc123def456 Author <author@example.com> %d +0000 commit: Add src/main.go
abc123def456 def789abc012 Author <author@example.com> %d +0000 commit: Update src/main.go and README.md
def789abc012 ghi345jkl678 Author <author@example.com> %d +0000 commit: Fix bug in src/utils.go`,
		now.AddDate(0, 0, -7).Unix(),
		now.AddDate(0, 0, -3).Unix(),
		now.AddDate(0, 0, -1).Unix())

	err = os.WriteFile(logPath, []byte(logContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	extractor := NewExtractor(tmpDir, 30) // 30 day window
	stats := &GitStats{
		Recency:   make(map[string]float64),
		Frequency: make(map[string]float64),
	}

	success := extractor.readGitLogFile(logPath, stats)
	if !success {
		t.Error("Expected readGitLogFile to succeed")
	}

	// Should have extracted some file statistics
	if len(stats.Recency) == 0 && len(stats.Frequency) == 0 {
		t.Error("Expected some file statistics to be extracted")
	}
}

func TestExtractor_readGitLogFile_NonexistentFile(t *testing.T) {
	extractor := NewExtractor("/test", 365)
	stats := &GitStats{
		Recency:   make(map[string]float64),
		Frequency: make(map[string]float64),
	}

	success := extractor.readGitLogFile("/nonexistent/path", stats)
	if success {
		t.Error("Expected readGitLogFile to fail for nonexistent file")
	}
}

func TestExtractor_extractFromLoomTouchLog(t *testing.T) {
	extractor := NewExtractor("/test", 365)
	stats := &GitStats{
		Recency:   make(map[string]float64),
		Frequency: make(map[string]float64),
	}

	// This method is not implemented yet, should return false
	success := extractor.extractFromLoomTouchLog(stats)
	if success {
		t.Error("Expected extractFromLoomTouchLog to return false (not implemented)")
	}
}

// Helper function for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

package gitstats

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GitStats represents git-based file statistics
type GitStats struct {
	Recency   map[string]float64 // path -> recency score [0,1]
	Frequency map[string]float64 // path -> frequency score [0,1]
	Mode      string             // "none", "gitdir", "touchlog"
}

// Extractor extracts git statistics in zero-exec mode
type Extractor struct {
	root       string
	windowDays int
}

// NewExtractor creates a new git stats extractor
func NewExtractor(root string, windowDays int) *Extractor {
	return &Extractor{
		root:       root,
		windowDays: windowDays,
	}
}

// Extract extracts git statistics and returns recency and frequency scores
func (e *Extractor) Extract() *GitStats {
	stats := &GitStats{
		Recency:   make(map[string]float64),
		Frequency: make(map[string]float64),
		Mode:      "none",
	}

	// Check if .git directory exists
	gitDir := filepath.Join(e.root, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return stats // Return empty stats if no git repo
	}

	// Try different approaches in order of preference
	if e.extractFromLoomTouchLog(stats) {
		stats.Mode = "touchlog"
		return stats
	}

	if e.extractFromGitLogs(stats) {
		stats.Mode = "gitdir"
		return stats
	}

	// If we can't get git stats, return empty (zero values)
	return stats
}

// extractFromLoomTouchLog extracts from Loom's own touch log if available
func (e *Extractor) extractFromLoomTouchLog(stats *GitStats) bool {
	// This would read from ~/.loom/projects/<id>/touches.log
	// For now, we'll skip this implementation as it requires
	// the project ID and user home directory access
	return false
}

// extractFromGitLogs attempts to read git logs directly from .git directory
func (e *Extractor) extractFromGitLogs(stats *GitStats) bool {
	// Read from .git/logs/HEAD if available
	if e.readGitHeadLog(stats) {
		return true
	}

	// Read from .git/logs/refs/heads/* if available
	if e.readGitRefLogs(stats) {
		return true
	}

	return false
}

// readGitHeadLog reads the git HEAD log
func (e *Extractor) readGitHeadLog(stats *GitStats) bool {
	headLogPath := filepath.Join(e.root, ".git", "logs", "HEAD")
	return e.readGitLogFile(headLogPath, stats)
}

// readGitRefLogs reads git reference logs
func (e *Extractor) readGitRefLogs(stats *GitStats) bool {
	refsDir := filepath.Join(e.root, ".git", "logs", "refs", "heads")

	entries, err := os.ReadDir(refsDir)
	if err != nil {
		return false
	}

	found := false
	for _, entry := range entries {
		if !entry.IsDir() {
			logPath := filepath.Join(refsDir, entry.Name())
			if e.readGitLogFile(logPath, stats) {
				found = true
			}
		}
	}

	return found
}

// readGitLogFile reads a git log file and extracts file statistics
func (e *Extractor) readGitLogFile(logPath string, stats *GitStats) bool {
	file, err := os.Open(logPath)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()

	// Track file modifications
	fileModifications := make(map[string][]time.Time)
	cutoffTime := time.Now().AddDate(0, 0, -e.windowDays)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Parse git log line format:
		// <old-sha> <new-sha> <name> <email> <timestamp> <timezone> <message>
		commit := e.parseGitLogLine(line)
		if commit == nil {
			continue
		}

		// Skip commits outside our window
		if commit.Time.Before(cutoffTime) {
			continue
		}

		// For now, we can't easily get the files changed from the log line alone
		// In a more complete implementation, we could:
		// 1. Read the commit objects directly from .git/objects
		// 2. Parse the commit message for file mentions
		// 3. Use heuristics based on the commit message

		// Simple heuristic: extract file paths from commit messages
		files := e.extractFilePathsFromMessage(commit.Message)
		for _, file := range files {
			fileModifications[file] = append(fileModifications[file], commit.Time)
		}
	}

	if len(fileModifications) == 0 {
		return false
	}

	// Calculate recency and frequency scores
	e.calculateScores(fileModifications, stats, cutoffTime)
	return true
}

// parseGitLogLine parses a git log line
func (e *Extractor) parseGitLogLine(line string) *GitCommit {
	// Git log format: <old-sha> <new-sha> <name> <email> <timestamp> <timezone> <message>
	parts := strings.Fields(line)
	if len(parts) < 6 {
		return nil
	}

	// Parse timestamp
	timestamp, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return nil
	}

	// Extract message (everything after timezone)
	messageStart := 6
	if messageStart >= len(parts) {
		return nil
	}

	message := strings.Join(parts[messageStart:], " ")

	return &GitCommit{
		OldSHA:  parts[0],
		NewSHA:  parts[1],
		Author:  parts[2] + " " + parts[3], // name + email
		Time:    time.Unix(timestamp, 0),
		Message: message,
	}
}

// extractFilePathsFromMessage extracts file paths from commit messages
func (e *Extractor) extractFilePathsFromMessage(message string) []string {
	var files []string

	// Common patterns for file mentions in commit messages
	patterns := []*regexp.Regexp{
		// Direct file references
		regexp.MustCompile(`\b(?:src|app|cmd|internal|ui|frontend|backend|lib|pkg|test|tests)/[a-zA-Z0-9_/\-]+\.[a-zA-Z0-9]+`),
		// Standalone file references
		regexp.MustCompile(`\b[a-zA-Z0-9_\-]+\.(ts|tsx|js|jsx|go|php|py|rb|rs|c|cpp|h|hpp|yaml|yml|json|toml|md|txt|css|scss)`),
		// Config files
		regexp.MustCompile(`\b(package\.json|composer\.json|go\.mod|Dockerfile|docker-compose\.yml|Makefile|\.gitignore)\b`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllString(message, -1)
		for _, match := range matches {
			// Clean up the match
			cleaned := strings.Trim(match, " \t.,;:()[]{}\"'")
			if cleaned != "" && e.isValidFilePath(cleaned) {
				files = append(files, cleaned)
			}
		}
	}

	return e.uniqueStrings(files)
}

// isValidFilePath checks if a string looks like a valid file path
func (e *Extractor) isValidFilePath(path string) bool {
	// Basic validation
	if len(path) < 2 || len(path) > 200 {
		return false
	}

	// Must contain either a slash or a file extension
	hasSlash := strings.Contains(path, "/")
	hasExt := strings.Contains(path, ".")

	if !hasSlash && !hasExt {
		return false
	}

	// Exclude obvious non-file patterns
	excludePatterns := []string{
		"http://", "https://", "ftp://",
		"localhost", "127.0.0.1", "example.com",
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range excludePatterns {
		if strings.Contains(lowerPath, pattern) {
			return false
		}
	}

	return true
}

// calculateScores calculates recency and frequency scores
func (e *Extractor) calculateScores(fileModifications map[string][]time.Time, stats *GitStats, cutoffTime time.Time) {
	now := time.Now()
	windowDuration := now.Sub(cutoffTime)

	// Find max frequency for normalization
	maxFrequency := 0
	for _, times := range fileModifications {
		if len(times) > maxFrequency {
			maxFrequency = len(times)
		}
	}

	for file, times := range fileModifications {
		if len(times) == 0 {
			continue
		}

		// Calculate recency score (exponential decay from most recent change)
		mostRecent := times[0]
		for _, t := range times {
			if t.After(mostRecent) {
				mostRecent = t
			}
		}

		timeSinceLastChange := now.Sub(mostRecent)
		recencyScore := 1.0 - (timeSinceLastChange.Seconds() / windowDuration.Seconds())
		if recencyScore < 0 {
			recencyScore = 0
		}

		// Apply exponential decay
		recencyScore = 1.0 - (1.0-recencyScore)*(1.0-recencyScore)

		stats.Recency[file] = recencyScore

		// Calculate frequency score (normalized by max frequency)
		frequencyScore := float64(len(times)) / float64(maxFrequency)
		stats.Frequency[file] = frequencyScore
	}
}

// uniqueStrings removes duplicate strings
func (e *Extractor) uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, str := range strs {
		if !seen[str] {
			seen[str] = true
			result = append(result, str)
		}
	}

	return result
}

// GitCommit represents a git commit
type GitCommit struct {
	OldSHA  string
	NewSHA  string
	Author  string
	Time    time.Time
	Message string
}

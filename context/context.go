package context

import (
	"fmt"
	"loom/indexer"
	"loom/llm"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TokenEstimator provides rough token count estimates
type TokenEstimator struct {
	// Rough estimate: ~4 characters per token for code/text
	CharsPerToken float64
}

// NewTokenEstimator creates a new token estimator
func NewTokenEstimator() *TokenEstimator {
	return &TokenEstimator{
		CharsPerToken: 4.0, // Conservative estimate
	}
}

// EstimateTokens estimates token count for text
func (te *TokenEstimator) EstimateTokens(text string) int {
	return int(float64(len(text)) / te.CharsPerToken)
}

// FileReference represents a file summary without full content
type FileReference struct {
	Path         string    `json:"path"`
	Hash         string    `json:"hash"`
	Size         int64     `json:"size"`
	Language     string    `json:"language"`
	LastModified time.Time `json:"last_modified"`
	Summary      string    `json:"summary"`
	LineCount    int       `json:"line_count"`
}

// FileSnippet represents a focused view of a file around specific lines
type FileSnippet struct {
	Path       string `json:"path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Content    string `json:"content"`
	TotalLines int    `json:"total_lines"`
	Context    string `json:"context"` // Why this snippet was included
	Hash       string `json:"hash"`    // Hash of full file for change detection
}

// ContextManager manages context optimization for LLM interactions
type ContextManager struct {
	tokenEstimator *TokenEstimator
	maxTokens      int
	snippetPadding int // Lines to include before/after target lines
	index          *indexer.Index
	fileCache      map[string]*FileReference // Cache of file references
}

// NewContextManager creates a new context manager
func NewContextManager(index *indexer.Index, maxTokens int) *ContextManager {
	if maxTokens <= 0 {
		maxTokens = 6000 // Conservative default for most models
	}

	return &ContextManager{
		tokenEstimator: NewTokenEstimator(),
		maxTokens:      maxTokens,
		snippetPadding: 30, // ±30 lines around target
		index:          index,
		fileCache:      make(map[string]*FileReference),
	}
}

// OptimizeMessages optimizes a list of messages for context window constraints
func (cm *ContextManager) OptimizeMessages(messages []llm.Message) ([]llm.Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	optimized := make([]llm.Message, 0, len(messages))
	totalTokens := 0

	// Always preserve system messages (first)
	systemMessages := []llm.Message{}
	otherMessages := []llm.Message{}

	for _, msg := range messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg)
		} else {
			otherMessages = append(otherMessages, msg)
		}
	}

	// Add system messages first
	for _, msg := range systemMessages {
		tokens := cm.tokenEstimator.EstimateTokens(msg.Content)
		if totalTokens+tokens < cm.maxTokens {
			optimized = append(optimized, msg)
			totalTokens += tokens
		}
	}

	// Add other messages from most recent backward
	for i := len(otherMessages) - 1; i >= 0; i-- {
		msg := otherMessages[i]
		optimizedMsg, tokens := cm.optimizeMessage(msg)

		if totalTokens+tokens < cm.maxTokens {
			optimized = append([]llm.Message{optimizedMsg}, optimized[len(systemMessages):]...)
			totalTokens += tokens
		} else {
			// Try to summarize older messages if we're running out of space
			if i < len(otherMessages)-5 { // Keep last 5 messages full
				summary := cm.summarizeOlderMessages(otherMessages[:i+1])
				summaryTokens := cm.tokenEstimator.EstimateTokens(summary.Content)
				if totalTokens+summaryTokens < cm.maxTokens {
					optimized = append([]llm.Message{summary}, optimized[len(systemMessages):]...)
					totalTokens += summaryTokens
				}
				break
			}
		}
	}

	return optimized, nil
}

// optimizeMessage optimizes a single message for token efficiency
func (cm *ContextManager) optimizeMessage(msg llm.Message) (llm.Message, int) {
	// For now, return the message as-is
	// Future: Could implement file content optimization here
	tokens := cm.tokenEstimator.EstimateTokens(msg.Content)
	return msg, tokens
}

// summarizeOlderMessages creates a summary of older chat messages
func (cm *ContextManager) summarizeOlderMessages(messages []llm.Message) llm.Message {
	var content strings.Builder
	content.WriteString("## Previous Conversation Summary\n")

	userQuestions := []string{}
	assistantActions := []string{}

	for _, msg := range messages {
		if msg.Role == "user" {
			userQuestions = append(userQuestions, strings.TrimSpace(msg.Content))
		} else if msg.Role == "assistant" {
			// Extract key actions/topics from assistant messages
			if len(msg.Content) > 100 {
				// Truncate long responses
				summary := msg.Content[:100] + "..."
				assistantActions = append(assistantActions, summary)
			} else {
				assistantActions = append(assistantActions, msg.Content)
			}
		}
	}

	if len(userQuestions) > 0 {
		content.WriteString("User asked about: ")
		content.WriteString(strings.Join(userQuestions[:min(len(userQuestions), 3)], "; "))
		content.WriteString("\n")
	}

	if len(assistantActions) > 0 {
		content.WriteString("Assistant discussed: ")
		content.WriteString(strings.Join(assistantActions[:min(len(assistantActions), 3)], "; "))
		content.WriteString("\n")
	}

	return llm.Message{
		Role:      "system",
		Content:   content.String(),
		Timestamp: time.Now(),
	}
}

// CreateFileReference creates a reference to a file without including full content
func (cm *ContextManager) CreateFileReference(filePath string) (*FileReference, error) {
	// Check cache first
	if ref, exists := cm.fileCache[filePath]; exists {
		return ref, nil
	}

	// Get file metadata from index
	fileMeta, exists := cm.index.Files[filePath]
	if !exists {
		return nil, fmt.Errorf("file not found in index: %s", filePath)
	}

	// Create reference
	ref := &FileReference{
		Path:         filePath,
		Hash:         fileMeta.Hash,
		Size:         fileMeta.Size,
		Language:     fileMeta.Language,
		LastModified: fileMeta.ModTime,
		Summary:      cm.generateFileSummary(filePath, fileMeta),
		LineCount:    cm.estimateLineCount(fileMeta.Size),
	}

	// Cache the reference
	cm.fileCache[filePath] = ref

	return ref, nil
}

// CreateFileSnippet creates a focused snippet of a file around specific lines
func (cm *ContextManager) CreateFileSnippet(filePath string, targetLine int, context string) (*FileSnippet, error) {
	// Get file metadata
	fileMeta, exists := cm.index.Files[filePath]
	if !exists {
		return nil, fmt.Errorf("file not found in index: %s", filePath)
	}

	// Calculate snippet bounds with padding
	startLine := max(1, targetLine-cm.snippetPadding)
	endLine := targetLine + cm.snippetPadding

	// Read the file content (this would need to be implemented)
	// For now, create a placeholder
	snippet := &FileSnippet{
		Path:       filePath,
		StartLine:  startLine,
		EndLine:    endLine,
		Content:    fmt.Sprintf("// File snippet for %s (lines %d-%d)\n// Context: %s", filePath, startLine, endLine, context),
		TotalLines: cm.estimateLineCount(fileMeta.Size),
		Context:    context,
		Hash:       fileMeta.Hash,
	}

	return snippet, nil
}

// CreateFileSnippetRange creates a snippet for a specific line range
func (cm *ContextManager) CreateFileSnippetRange(filePath string, startLine, endLine int, context string) (*FileSnippet, error) {
	fileMeta, exists := cm.index.Files[filePath]
	if !exists {
		return nil, fmt.Errorf("file not found in index: %s", filePath)
	}

	snippet := &FileSnippet{
		Path:       filePath,
		StartLine:  startLine,
		EndLine:    endLine,
		Content:    fmt.Sprintf("// File snippet for %s (lines %d-%d)\n// Context: %s", filePath, startLine, endLine, context),
		TotalLines: cm.estimateLineCount(fileMeta.Size),
		Context:    context,
		Hash:       fileMeta.Hash,
	}

	return snippet, nil
}

// generateFileSummary creates a brief summary of a file based on its metadata
func (cm *ContextManager) generateFileSummary(filePath string, fileMeta *indexer.FileMeta) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	base := filepath.Base(filePath)

	var summary strings.Builder

	// File type and purpose inference
	switch {
	case ext == ".go":
		if strings.Contains(base, "_test.go") {
			summary.WriteString("Go test file")
		} else if strings.Contains(base, "main.go") {
			summary.WriteString("Go main entry point")
		} else {
			summary.WriteString("Go source file")
		}
	case ext == ".js" || ext == ".ts":
		summary.WriteString("JavaScript/TypeScript file")
	case ext == ".py":
		summary.WriteString("Python script")
	case ext == ".md":
		summary.WriteString("Markdown documentation")
	case ext == ".json":
		summary.WriteString("JSON configuration/data")
	case ext == ".yaml" || ext == ".yml":
		summary.WriteString("YAML configuration")
	case strings.Contains(base, "Dockerfile"):
		summary.WriteString("Docker container definition")
	case strings.Contains(base, "README"):
		summary.WriteString("Project README")
	default:
		summary.WriteString(fmt.Sprintf("%s file", fileMeta.Language))
	}

	summary.WriteString(fmt.Sprintf(" (%.1fKB, ~%d lines)",
		float64(fileMeta.Size)/1024,
		cm.estimateLineCount(fileMeta.Size)))

	return summary.String()
}

// estimateLineCount estimates line count based on file size
func (cm *ContextManager) estimateLineCount(fileSize int64) int {
	// Rough estimate: 50 characters per line on average
	return int(fileSize / 50)
}

// GetContextBudget returns the remaining token budget for context
func (cm *ContextManager) GetContextBudget(currentTokens int) int {
	remaining := cm.maxTokens - currentTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// FormatFileReferences formats file references for inclusion in prompts
func (cm *ContextManager) FormatFileReferences(refs []*FileReference) string {
	if len(refs) == 0 {
		return ""
	}

	var content strings.Builder
	content.WriteString("## Workspace File References\n\n")

	// Group by language/type
	langGroups := make(map[string][]*FileReference)
	for _, ref := range refs {
		lang := ref.Language
		if lang == "" {
			lang = "Other"
		}
		langGroups[lang] = append(langGroups[lang], ref)
	}

	// Sort languages by file count
	type langCount struct {
		lang  string
		count int
	}

	var sortedLangs []langCount
	for lang, files := range langGroups {
		sortedLangs = append(sortedLangs, langCount{lang, len(files)})
	}

	sort.Slice(sortedLangs, func(i, j int) bool {
		return sortedLangs[i].count > sortedLangs[j].count
	})

	for _, lc := range sortedLangs {
		files := langGroups[lc.lang]
		content.WriteString(fmt.Sprintf("### %s Files (%d)\n", lc.lang, len(files)))

		for _, ref := range files {
			content.WriteString(fmt.Sprintf("- `%s`: %s\n", ref.Path, ref.Summary))
		}
		content.WriteString("\n")
	}

	return content.String()
}

// FormatFileSnippets formats file snippets for inclusion in prompts
func (cm *ContextManager) FormatFileSnippets(snippets []*FileSnippet) string {
	if len(snippets) == 0 {
		return ""
	}

	var content strings.Builder
	content.WriteString("## Relevant File Snippets\n\n")

	for _, snippet := range snippets {
		content.WriteString(fmt.Sprintf("### %s (lines %d-%d of %d)\n",
			snippet.Path, snippet.StartLine, snippet.EndLine, snippet.TotalLines))

		if snippet.Context != "" {
			content.WriteString(fmt.Sprintf("*Context: %s*\n\n", snippet.Context))
		}

		content.WriteString("```")
		if ext := filepath.Ext(snippet.Path); ext != "" {
			content.WriteString(ext[1:]) // Remove the dot
		}
		content.WriteString("\n")
		content.WriteString(snippet.Content)
		content.WriteString("\n```\n\n")
	}

	return content.String()
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

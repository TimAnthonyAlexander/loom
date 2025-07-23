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
	tokenEstimator   *TokenEstimator
	maxTokens        int
	snippetPadding   int // Lines to include before/after target lines
	index            *indexer.Index
	fileCache        map[string]*FileReference // Cache of file references
	snippetExtractor *SnippetExtractor         // Language-aware snippet extraction
}

// NewContextManager creates a new context manager
func NewContextManager(index *indexer.Index, maxTokens int) *ContextManager {
	if maxTokens <= 0 {
		maxTokens = 6000 // Conservative default for most models
	}

	return &ContextManager{
		tokenEstimator:   NewTokenEstimator(),
		maxTokens:        maxTokens,
		snippetPadding:   30, // ±30 lines around target
		index:            index,
		fileCache:        make(map[string]*FileReference),
		snippetExtractor: NewSnippetExtractor(index),
	}
}

// OptimizeMessages optimizes a list of messages for context window constraints
func (cm *ContextManager) OptimizeMessages(messages []llm.Message) ([]llm.Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	// Categorize messages by role
	systemMessages := []llm.Message{}
	userMessages := []llm.Message{}
	assistantMessages := []llm.Message{}

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			systemMessages = append(systemMessages, msg)
		case "user":
			userMessages = append(userMessages, msg)
		case "assistant":
			assistantMessages = append(assistantMessages, msg)
		}
	}

	// Initialize optimized messages array
	optimized := make([]llm.Message, 0, len(messages))
	totalTokens := 0

	// ALWAYS include system messages (highest priority)
	for _, msg := range systemMessages {
		tokens := cm.tokenEstimator.EstimateTokens(msg.Content)
		if totalTokens+tokens < cm.maxTokens {
			optimized = append(optimized, msg)
			totalTokens += tokens
		} else {
			// System messages are critical - if we can't include all of them,
			// we're likely over capacity and need to trim somewhere else
		}
	}

	// Add the initial user message if it exists (important for establishing context)
	if len(userMessages) > 0 {
		initialMsg := userMessages[0]
		tokens := cm.tokenEstimator.EstimateTokens(initialMsg.Content)
		if totalTokens+tokens < cm.maxTokens {
			optimized = append(optimized, initialMsg)
			totalTokens += tokens
		}
	}

	// Define the number of recent messages to always include
	const recentMessagesToKeep = 10

	// Calculate remaining messages that need optimization
	var remainingMessages []llm.Message
	if len(messages) > recentMessagesToKeep+len(systemMessages)+1 { // +1 for initial message
		// We need to exclude the system messages, initial message, and N recent messages
		startIdx := 0
		for _, msg := range messages {
			if msg.Role == "system" {
				startIdx++
			}
		}
		startIdx++ // Skip the initial user message too

		// All messages except system, initial and recent
		if startIdx+recentMessagesToKeep < len(messages) {
			endIdx := len(messages) - recentMessagesToKeep
			remainingMessages = messages[startIdx:endIdx]
		}
	}

	// Create a summary of older messages if needed
	if len(remainingMessages) > 0 {
		summary := cm.summarizeOlderMessages(remainingMessages)
		summaryTokens := cm.tokenEstimator.EstimateTokens(summary.Content)
		if totalTokens+summaryTokens < cm.maxTokens {
			optimized = append(optimized, summary)
			totalTokens += summaryTokens
		}
	}

	// Include the current objective/task if it can be identified
	objective := cm.extractCurrentObjective(messages)
	if objective != nil {
		objectiveTokens := cm.tokenEstimator.EstimateTokens(objective.Content)
		if totalTokens+objectiveTokens < cm.maxTokens {
			optimized = append(optimized, *objective)
			totalTokens += objectiveTokens
		}
	}

	// Add recent messages (always keep these)
	if len(messages) > recentMessagesToKeep {
		recentMessages := messages[len(messages)-recentMessagesToKeep:]
		for _, msg := range recentMessages {
			tokens := cm.tokenEstimator.EstimateTokens(msg.Content)
			if totalTokens+tokens < cm.maxTokens {
				optimized = append(optimized, msg)
				totalTokens += tokens
			} else {
				// If we can't include a recent message, try to condense it
				condensed, condensedTokens := cm.condenseMessage(msg)
				if totalTokens+condensedTokens < cm.maxTokens {
					optimized = append(optimized, condensed)
					totalTokens += condensedTokens
				}
			}
		}
	} else {
		// If we have fewer messages than our target, just include them all
		for i, msg := range messages {
			// Skip messages we've already added (system messages and initial message)
			if msg.Role == "system" || (msg.Role == "user" && i == 0) {
				continue
			}
			tokens := cm.tokenEstimator.EstimateTokens(msg.Content)
			if totalTokens+tokens < cm.maxTokens {
				optimized = append(optimized, msg)
				totalTokens += tokens
			} else {
				condensed, condensedTokens := cm.condenseMessage(msg)
				if totalTokens+condensedTokens < cm.maxTokens {
					optimized = append(optimized, condensed)
					totalTokens += condensedTokens
				}
			}
		}
	}

	return optimized, nil
}

// extractCurrentObjective tries to find the current objective from recent assistant messages
func (cm *ContextManager) extractCurrentObjective(messages []llm.Message) *llm.Message {
	// Search for OBJECTIVE pattern in recent assistant messages
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == "assistant" {
			if strings.Contains(msg.Content, "OBJECTIVE:") {
				// Extract objective line and create a system message
				lines := strings.Split(msg.Content, "\n")
				for _, line := range lines {
					if strings.Contains(line, "OBJECTIVE:") {
						return &llm.Message{
							Role:      "system",
							Content:   "Current " + strings.TrimSpace(line),
							Timestamp: time.Now(),
						}
					}
				}
			}
		}
	}
	return nil
}

// condenseMessage tries to shorten a message while preserving key information
func (cm *ContextManager) condenseMessage(msg llm.Message) (llm.Message, int) {
	// Don't attempt to condense system messages
	if msg.Role == "system" {
		return msg, cm.tokenEstimator.EstimateTokens(msg.Content)
	}

	// For long messages, extract key parts
	content := msg.Content
	if len(content) > 1000 {
		// Keep first and last parts
		firstPart := content[:500]
		lastPart := content[len(content)-500:]
		condensed := firstPart + "\n\n...[content truncated]...\n\n" + lastPart
		return llm.Message{
			Role:      msg.Role,
			Content:   condensed,
			Timestamp: msg.Timestamp,
		}, cm.tokenEstimator.EstimateTokens(condensed)
	}

	return msg, cm.tokenEstimator.EstimateTokens(content)
}

// summarizeOlderMessages creates a summary of older chat messages
func (cm *ContextManager) summarizeOlderMessages(messages []llm.Message) llm.Message {
	var content strings.Builder
	content.WriteString("## Previous Conversation Summary\n")

	// Extract user questions
	userQuestions := []string{}
	for _, msg := range messages {
		if msg.Role == "user" {
			// Take just the first line or a portion for brevity
			question := msg.Content
			if idx := strings.Index(question, "\n"); idx > 0 {
				question = question[:idx]
			}
			if len(question) > 100 {
				question = question[:100] + "..."
			}
			userQuestions = append(userQuestions, strings.TrimSpace(question))
		}
	}

	// Extract assistant actions (focus on tasks and objectives)
	assistantActions := []string{}
	objectives := []string{}
	tasks := []string{}
	taskResults := make(map[string]string) // Key: task description, Value: summarized result

	for _, msg := range messages {
		if msg.Role == "assistant" {
			// Extract objectives
			if strings.Contains(msg.Content, "OBJECTIVE:") {
				lines := strings.Split(msg.Content, "\n")
				for _, line := range lines {
					if strings.Contains(line, "OBJECTIVE:") {
						objectives = append(objectives, strings.TrimSpace(line))
						break
					}
				}
			}

			// Extract tasks (look for the wrench emoji pattern or TASK_RESULT)
			lines := strings.Split(msg.Content, "\n")
			var currentTask string
			var currentResult strings.Builder
			inResult := false
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "🔧") || strings.HasPrefix(trimmed, "TASK_RESULT:") {
					if currentTask != "" && currentResult.Len() > 0 {
						taskResults[currentTask] = strings.TrimSpace(currentResult.String())
					}
					currentTask = trimmed
					currentResult.Reset()
					inResult = true
					tasks = append(tasks, trimmed)
				} else if inResult {
					if trimmed == "" {
						inResult = false
						if currentTask != "" && currentResult.Len() > 0 {
							taskResults[currentTask] = strings.TrimSpace(currentResult.String())
						}
					} else {
						currentResult.WriteString(line + "\n")
					}
				}
			}
			// Add any remaining result
			if currentTask != "" && currentResult.Len() > 0 {
				taskResults[currentTask] = strings.TrimSpace(currentResult.String())
			}

			// Extract general actions for context
			if len(msg.Content) > 200 {
				// Get a summarized action
				action := msg.Content[:100] + "..."
				assistantActions = append(assistantActions, action)
			} else {
				assistantActions = append(assistantActions, msg.Content)
			}
		}
	}

	// Add user questions (up to 5)
	if len(userQuestions) > 0 {
		content.WriteString("### User asked about:\n")
		for i, q := range userQuestions {
			if i >= 5 {
				content.WriteString(fmt.Sprintf("- ...and %d more questions\n", len(userQuestions)-5))
				break
			}
			content.WriteString("- " + q + "\n")
		}
		content.WriteString("\n")
	}

	// Add objectives (up to 3)
	if len(objectives) > 0 {
		content.WriteString("### Previous objectives:\n")
		for i, obj := range objectives {
			if i >= 3 {
				content.WriteString(fmt.Sprintf("- ...and %d more objectives\n", len(objectives)-3))
				break
			}
			content.WriteString("- " + obj + "\n")
		}
		content.WriteString("\n")
	}

	// Add tasks with results (up to 10 most recent, in reverse order to prioritize recent)
	if len(tasks) > 0 {
		content.WriteString("### Previously Executed Tasks (most recent first):\n")
		// Reverse tasks to show most recent first
		reversedTasks := make([]string, len(tasks))
		copy(reversedTasks, tasks)
		sort.Sort(sort.Reverse(sort.StringSlice(reversedTasks)))

		for i, task := range reversedTasks {
			if i >= 10 {
				content.WriteString(fmt.Sprintf("- ...and %d more tasks\n", len(tasks)-10))
				break
			}
			result := taskResults[task]
			if result != "" {
				// Summarize long results
				if len(result) > 200 {
					result = result[:200] + "... (truncated)"
				}
				content.WriteString(fmt.Sprintf("- %s\n  Result: %s\n", task, result))
			} else {
				content.WriteString(fmt.Sprintf("- %s\n", task))
			}
		}
		content.WriteString("\n")
	}

	// Add high-level summary
	content.WriteString(fmt.Sprintf("This summary replaces %d older messages to save context space.\n", len(messages)))

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

// CreateEnhancedFileSnippet creates a language-aware file snippet
func (cm *ContextManager) CreateEnhancedFileSnippet(filePath string, targetLine int, context string) (*FileSnippet, error) {
	// Use the snippet extractor to get a contextual snippet
	return cm.snippetExtractor.GetContextualSnippet(filePath, targetLine, cm.snippetPadding)
}

// GetCodeStructures returns all code structures in a file
func (cm *ContextManager) GetCodeStructures(filePath string) ([]*CodeStructure, error) {
	return cm.snippetExtractor.ExtractStructures(filePath)
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

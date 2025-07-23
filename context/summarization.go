package context

import (
	"context"
	"fmt"
	"loom/llm"
	"strings"
	"time"
)

// SummaryType represents different types of summaries
type SummaryType string

const (
	SummaryTypeSession    SummaryType = "session"    // Full session summary
	SummaryTypeRecent     SummaryType = "recent"     // Recent messages summary
	SummaryTypeActionPlan SummaryType = "actionplan" // Action plan summary
	SummaryTypeProgress   SummaryType = "progress"   // Progress/changes summary
)

// Summary represents a generated summary
type Summary struct {
	Type         SummaryType            `json:"type"`
	Title        string                 `json:"title"`
	Content      string                 `json:"content"`
	MessageRange string                 `json:"message_range"` // e.g., "messages 1-25"
	CreatedAt    time.Time              `json:"created_at"`
	TokensSaved  int                    `json:"tokens_saved"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Summarizer handles LLM-based summarization of chat history
type Summarizer struct {
	llmAdapter     llm.LLMAdapter
	tokenEstimator *TokenEstimator
}

// NewSummarizer creates a new summarizer
func NewSummarizer(llmAdapter llm.LLMAdapter) *Summarizer {
	return &Summarizer{
		llmAdapter:     llmAdapter,
		tokenEstimator: NewTokenEstimator(),
	}
}

// SummarizeMessages creates a summary of a message sequence
func (s *Summarizer) SummarizeMessages(messages []llm.Message, summaryType SummaryType) (*Summary, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages to summarize")
	}

	// Create summarization prompt based on type
	prompt := s.createSummarizationPrompt(messages, summaryType)

	// Get summary from LLM
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	summaryMsg, err := s.llmAdapter.Send(ctx, []llm.Message{prompt})
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	// Calculate tokens saved
	originalTokens := s.estimateMessagesTokens(messages)
	summaryTokens := s.tokenEstimator.EstimateTokens(summaryMsg.Content)
	tokensSaved := originalTokens - summaryTokens

	summary := &Summary{
		Type:         summaryType,
		Title:        s.extractSummaryTitle(summaryMsg.Content),
		Content:      summaryMsg.Content,
		MessageRange: fmt.Sprintf("messages 1-%d", len(messages)),
		CreatedAt:    time.Now(),
		TokensSaved:  tokensSaved,
		Metadata:     make(map[string]interface{}),
	}

	// Add type-specific metadata
	switch summaryType {
	case SummaryTypeSession:
		summary.Metadata["total_messages"] = len(messages)
		summary.Metadata["time_span"] = s.calculateTimeSpan(messages)
	case SummaryTypeActionPlan:
		summary.Metadata["action_plans"] = s.countActionPlans(messages)
		summary.Metadata["tasks_executed"] = s.countTasks(messages)
	}

	return summary, nil
}

// SummarizeRecentHistory creates a summary of recent conversation history
func (s *Summarizer) SummarizeRecentHistory(messages []llm.Message, keepLastN int) (*Summary, error) {
	if len(messages) <= keepLastN {
		return nil, fmt.Errorf("not enough messages to summarize (have %d, keeping %d)", len(messages), keepLastN)
	}

	// Summarize all but the last N messages
	toSummarize := messages[:len(messages)-keepLastN]
	return s.SummarizeMessages(toSummarize, SummaryTypeRecent)
}

// SummarizeActionPlans creates a summary focused on action plans and changes
func (s *Summarizer) SummarizeActionPlans(messages []llm.Message) (*Summary, error) {
	// Filter messages for action plan related content
	actionMessages := s.filterActionPlanMessages(messages)

	if len(actionMessages) == 0 {
		return &Summary{
			Type:         SummaryTypeActionPlan,
			Title:        "No Action Plans",
			Content:      "No action plans or significant changes were made in this session.",
			MessageRange: "N/A",
			CreatedAt:    time.Now(),
			TokensSaved:  0,
		}, nil
	}

	return s.SummarizeMessages(actionMessages, SummaryTypeActionPlan)
}

// SummarizeProgress creates a progress-focused summary
func (s *Summarizer) SummarizeProgress(messages []llm.Message) (*Summary, error) {
	// Filter messages for progress indicators
	progressMessages := s.filterProgressMessages(messages)

	if len(progressMessages) == 0 {
		return &Summary{
			Type:         SummaryTypeProgress,
			Title:        "No Progress to Report",
			Content:      "No significant progress or achievements were recorded in this session.",
			MessageRange: "N/A",
			CreatedAt:    time.Now(),
			TokensSaved:  0,
		}, nil
	}

	return s.SummarizeMessages(progressMessages, SummaryTypeProgress)
}

// createSummarizationPrompt creates a prompt for the LLM to generate summaries
func (s *Summarizer) createSummarizationPrompt(messages []llm.Message, summaryType SummaryType) llm.Message {
	var promptText strings.Builder

	switch summaryType {
	case SummaryTypeSession:
		promptText.WriteString("Create a comprehensive summary of this coding session. Focus on:\n")
		promptText.WriteString("- Main goals and objectives discussed\n")
		promptText.WriteString("- Key decisions and architectural choices\n")
		promptText.WriteString("- Files modified and major changes made\n")
		promptText.WriteString("- Important learnings or insights\n")
		promptText.WriteString("- Current status and next steps\n\n")

	case SummaryTypeRecent:
		promptText.WriteString("Summarize this recent conversation. Include:\n")
		promptText.WriteString("- Key topics discussed\n")
		promptText.WriteString("- Any decisions made\n")
		promptText.WriteString("- Action items or tasks\n")
		promptText.WriteString("- Current context for continuation\n\n")

	case SummaryTypeActionPlan:
		promptText.WriteString("Summarize the action plans and changes made. Focus on:\n")
		promptText.WriteString("- What was implemented or modified\n")
		promptText.WriteString("- Rationale for changes\n")
		promptText.WriteString("- Files affected\n")
		promptText.WriteString("- Test status and outcomes\n\n")

	case SummaryTypeProgress:
		promptText.WriteString("Summarize the progress made in this session. Highlight:\n")
		promptText.WriteString("- Goals achieved\n")
		promptText.WriteString("- Challenges overcome\n")
		promptText.WriteString("- Current milestone status\n")
		promptText.WriteString("- What's planned next\n\n")
	}

	promptText.WriteString("Conversation to summarize:\n\n")

	// Add messages to prompt (excluding system messages)
	for i, msg := range messages {
		if msg.Role != "system" {
			promptText.WriteString(fmt.Sprintf("[%s %d]: %s\n\n",
				strings.Title(msg.Role), i+1, msg.Content))
		}
	}

	promptText.WriteString("\nProvide a clear, structured summary that captures the essential information and context.")

	return llm.Message{
		Role:      "user",
		Content:   promptText.String(),
		Timestamp: time.Now(),
	}
}

// Helper methods

func (s *Summarizer) extractSummaryTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 10 && len(line) < 100 {
			// Remove common prefixes
			line = strings.TrimPrefix(line, "Summary: ")
			line = strings.TrimPrefix(line, "## ")
			line = strings.TrimPrefix(line, "# ")
			if line != "" {
				return line
			}
		}
	}
	return "Session Summary"
}

func (s *Summarizer) estimateMessagesTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += s.tokenEstimator.EstimateTokens(msg.Content)
	}
	return total
}

func (s *Summarizer) calculateTimeSpan(messages []llm.Message) string {
	if len(messages) < 2 {
		return "N/A"
	}

	start := messages[0].Timestamp
	end := messages[len(messages)-1].Timestamp
	duration := end.Sub(start)

	if duration < time.Minute {
		return fmt.Sprintf("%.0f seconds", duration.Seconds())
	} else if duration < time.Hour {
		return fmt.Sprintf("%.0f minutes", duration.Minutes())
	} else {
		return fmt.Sprintf("%.1f hours", duration.Hours())
	}
}

func (s *Summarizer) countActionPlans(messages []llm.Message) int {
	count := 0
	for _, msg := range messages {
		if strings.Contains(msg.Content, "Action Plan") ||
			strings.Contains(msg.Content, "```json") {
			count++
		}
	}
	return count
}

func (s *Summarizer) countTasks(messages []llm.Message) int {
	count := 0
	for _, msg := range messages {
		count += strings.Count(msg.Content, "Task:")
		count += strings.Count(msg.Content, "✅")
		count += strings.Count(msg.Content, "🔧")
	}
	return count
}

func (s *Summarizer) filterActionPlanMessages(messages []llm.Message) []llm.Message {
	var filtered []llm.Message
	for _, msg := range messages {
		if msg.Role == "system" {
			continue
		}

		// Include messages with action plans or task execution
		if strings.Contains(msg.Content, "```json") ||
			strings.Contains(msg.Content, "Action Plan") ||
			strings.Contains(msg.Content, "Task:") ||
			strings.Contains(msg.Content, "ReadFile") ||
			strings.Contains(msg.Content, "✅") ||
			strings.Contains(msg.Content, "🔧") {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

func (s *Summarizer) filterProgressMessages(messages []llm.Message) []llm.Message {
	var filtered []llm.Message
	progressKeywords := []string{
		"completed", "finished", "implemented", "added", "created",
		"fixed", "resolved", "milestone", "progress", "achievement",
		"success", "working", "tested", "verified",
	}

	for _, msg := range messages {
		if msg.Role == "system" {
			continue
		}

		content := strings.ToLower(msg.Content)
		for _, keyword := range progressKeywords {
			if strings.Contains(content, keyword) {
				filtered = append(filtered, msg)
				break
			}
		}
	}
	return filtered
}

// IsAvailable checks if the summarizer can be used
func (s *Summarizer) IsAvailable() bool {
	return s.llmAdapter != nil && s.llmAdapter.IsAvailable()
}

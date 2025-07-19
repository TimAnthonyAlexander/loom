package task

import (
	"strings"
)

// CompletionDetector analyzes LLM responses to determine if work is complete (simplified)
type CompletionDetector struct{}

// NewCompletionDetector creates a new completion detector
func NewCompletionDetector() *CompletionDetector {
	return &CompletionDetector{}
}

// IsComplete checks if the LLM response indicates the work is finished (simplified)
func (cd *CompletionDetector) IsComplete(response string) bool {
	if response == "" {
		return false
	}

	lowerResponse := strings.ToLower(strings.TrimSpace(response))

	// Simple completion check
	completionWords := []string{"done", "complete", "finished"}
	for _, word := range completionWords {
		if strings.Contains(lowerResponse, word) {
			return true
		}
	}

	return false
}

// GenerateCompletionCheckPrompt creates a simple prompt to check if work is complete
func (cd *CompletionDetector) GenerateCompletionCheckPrompt() string {
	return "Is this task complete?"
}

// HasInfiniteLoopPattern detects if recent responses show repetitive behavior (simplified)
func (cd *CompletionDetector) HasInfiniteLoopPattern(responses []string) bool {
	if len(responses) < 3 {
		return false
	}

	// Simple check: if last two responses are very similar, it's likely a loop
	last := responses[len(responses)-1]
	secondLast := responses[len(responses)-2]

	return strings.Contains(last, secondLast) || strings.Contains(secondLast, last)
}

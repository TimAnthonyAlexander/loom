package task

import (
	"math/rand"
	"strings"
	"time"
)

// CompletionDetector analyzes LLM responses to determine if work is complete
type CompletionDetector struct{}

// NewCompletionDetector creates a new completion detector
func NewCompletionDetector() *CompletionDetector {
	return &CompletionDetector{}
}

// IsComplete checks if the LLM response indicates the work is finished
func (cd *CompletionDetector) IsComplete(response string) bool {
	if response == "" {
		return false
	}

	lowerResponse := strings.ToLower(strings.TrimSpace(response))
	
	// Explicit completion signals
	completionSignals := []string{
		"done", "task completed", "finished", "complete", 
		"all done", "task finished", "implementation complete",
		"work complete", "everything is done", "fully implemented",
		"ready to use", "setup complete", "installation complete",
		"successfully completed", "implementation finished",
		"all set", "work finished", "task complete",
	}
	
	for _, signal := range completionSignals {
		if strings.Contains(lowerResponse, signal) {
			return true
		}
	}
	
	// Check if response ends with completion phrase
	completionEndings := []string{
		"done.", "done!", "completed.", "completed!", 
		"finished.", "finished!", "complete.", "complete!",
		"ready.", "ready!", "all set.", "all set!",
		"task completed.", "task finished.", "work complete.",
	}
	
	for _, ending := range completionEndings {
		if strings.HasSuffix(lowerResponse, ending) {
			return true
		}
	}
	
	return false
}

// GenerateContinuePrompt creates a prompt to continue the work
func (cd *CompletionDetector) GenerateContinuePrompt() string {
	prompts := []string{
		"Continue with the next step.",
		"What's next?", 
		"Please proceed.",
		"Keep going.",
		"Continue the work.",
		"Please continue.",
		"What should we do next?",
		"Proceed with the implementation.",
	}
	
	// Use time-based seeding for variety
	rand.Seed(time.Now().UnixNano())
	return prompts[rand.Intn(len(prompts))]
}

// HasInfiniteLoopPattern detects if recent responses show repetitive behavior
func (cd *CompletionDetector) HasInfiniteLoopPattern(responses []string) bool {
	if len(responses) < 3 {
		return false
	}
	
	// Check last 3 responses for similar patterns
	recent := responses[len(responses)-3:]
	
	// Simple similarity check
	for i := 0; i < len(recent)-1; i++ {
		for j := i + 1; j < len(recent); j++ {
			if cd.areSimilar(recent[i], recent[j]) {
				return true
			}
		}
	}
	
	return false
}

// areSimilar checks if two responses are significantly similar
func (cd *CompletionDetector) areSimilar(a, b string) bool {
	wordsA := strings.Fields(strings.ToLower(a))
	wordsB := strings.Fields(strings.ToLower(b))
	
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return false
	}
	
	// Count common words
	commonWords := 0
	for _, wordA := range wordsA {
		for _, wordB := range wordsB {
			if wordA == wordB && len(wordA) > 3 { // Only count meaningful words
				commonWords++
				break
			}
		}
	}
	
	// Calculate similarity percentage
	maxLen := len(wordsA)
	if len(wordsB) > maxLen {
		maxLen = len(wordsB)
	}
	
	similarity := float64(commonWords) / float64(maxLen)
	return similarity > 0.7 // 70% similar indicates possible loop
}

// ShouldForceCompletion determines if we should force a completion check
func (cd *CompletionDetector) ShouldForceCompletion(depth int, duration time.Duration) (bool, string) {
	// Force completion after reasonable limits
	if depth >= 15 {
		return true, "Maximum thinking depth reached."
	}
	
	if duration > 30*time.Minute {
		return true, "Auto-continuation timeout reached."
	}
	
	return false, ""
} 
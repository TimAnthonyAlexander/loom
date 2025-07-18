package task

import (
	"fmt"
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
	
	// Debug output can be enabled by setting environment variable
	debugMode := false // Set to true for debugging
	
	// Explicit completion signals - expanded list
	completionSignals := []string{
		"done", "task completed", "finished", "complete", 
		"all done", "task finished", "implementation complete",
		"work complete", "everything is done", "fully implemented",
		"ready to use", "setup complete", "installation complete",
		"successfully completed", "implementation finished",
		"all set", "work finished", "task complete",
		"completed successfully", "finished successfully",
		"implementation is complete", "work is complete",
		"task is complete", "task is finished", "task is done",
		"everything is complete", "all finished", "all complete",
		"fully done", "completely done", "totally complete",
		"project completed", "feature completed", "feature finished",
		"system completed", "system finished", "api completed",
		"api finished", "completed the", "finished the",
	}
	
	for _, signal := range completionSignals {
		if strings.Contains(lowerResponse, signal) {
			if debugMode {
				fmt.Printf("✅ DEBUG: Found completion signal: '%s'\n", signal)
			}
			return true
		}
	}
	
	// Check if response ends with completion phrase
	completionEndings := []string{
		"done.", "done!", "completed.", "completed!", 
		"finished.", "finished!", "complete.", "complete!",
		"ready.", "ready!", "all set.", "all set!",
		"task completed.", "task finished.", "work complete.",
		"implementation complete.", "feature complete.",
		"system complete.", "api complete.", "project complete.",
	}
	
	for _, ending := range completionEndings {
		if strings.HasSuffix(lowerResponse, ending) {
			if debugMode {
				fmt.Printf("✅ DEBUG: Found completion ending: '%s'\n", ending)
			}
			return true
		}
	}
	
	// Check for completion in the last sentence
	sentences := strings.Split(lowerResponse, ".")
	if len(sentences) > 0 {
		lastSentence := strings.TrimSpace(sentences[len(sentences)-1])
		if lastSentence != "" {
			for _, signal := range completionSignals {
				if strings.Contains(lastSentence, signal) {
					if debugMode {
						fmt.Printf("✅ DEBUG: Found completion in last sentence: '%s' contains '%s'\n", lastSentence, signal)
					}
					return true
				}
			}
		}
	}
	
	// Check for common completion patterns with regex-like matching
	completionPatterns := []string{
		"is now complete", "is now finished", "is now done",
		"is fully complete", "is fully implemented", "is fully functional",
		"is ready to use", "is ready for use", "is working",
		"has been completed", "has been finished", "has been implemented",
		"have been completed", "have been finished", "have been implemented",
		"now complete", "now finished", "now ready",
		"successfully created", "successfully implemented", "successfully built",
		"all requirements", "all features", "all functionality",
		"entire system", "entire application", "entire project",
		"should be working", "should now work", "is working correctly",
		"is functioning", "is operational", "are operational",
	}
	
	for _, pattern := range completionPatterns {
		if strings.Contains(lowerResponse, pattern) {
			if debugMode {
				fmt.Printf("✅ DEBUG: Found completion pattern: '%s'\n", pattern)
			}
			return true
		}
	}
	
	// Check if the response is very short and likely a completion acknowledgment
	if len(strings.Fields(lowerResponse)) <= 3 {
		shortCompletions := []string{
			"done", "finished", "complete", "ready", "all set",
		}
		for _, short := range shortCompletions {
			if strings.Contains(lowerResponse, short) {
				if debugMode {
					fmt.Printf("✅ DEBUG: Found short completion: '%s'\n", short)
				}
				return true
			}
		}
	}
	
	if debugMode {
		fmt.Printf("❌ DEBUG: No completion signals found\n")
	}
	return false
}

// Helper function for debug output
func (cd *CompletionDetector) truncateForDebug(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return "..." + text[len(text)-maxLen:]
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
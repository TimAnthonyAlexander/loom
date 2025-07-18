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
		// Responses to completion check questions
		"yes, the task is complete", "yes, i'm finished", "yes, everything is done",
		"yes, it's complete", "yes, that's everything", "yes, we're done",
		"yes, finished", "yes, all done", "yes, complete",
		"the task is complete", "i'm finished", "everything is done",
		"that's everything", "we're done", "nothing else needed",
		"no more work needed", "all requirements met", "fully functional",
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
	
	// Check for Q&A completion patterns (when answering questions)
	qaCompletionPatterns := []string{
		"this project uses", "the license is", "according to the",
		"based on the", "the answer is", "to answer your question",
		"the current setup", "the configuration shows", "this is configured",
		"the file indicates", "looking at the", "from what i can see",
		"it appears to be", "it looks like", "the status is",
		"this codebase", "the workspace", "the repository",
		"currently using", "currently configured", "currently set",
		"you need to run", "you can run", "you should run",
		"the system uses", "the code uses", "authentication system uses",
		"according to", "based on", "looking at",
	}
	
	for _, pattern := range qaCompletionPatterns {
		if strings.Contains(lowerResponse, pattern) {
			if debugMode {
				fmt.Printf("🔍 DEBUG: Found Q&A pattern '%s'\n", pattern)
			}
			// If it's a Q&A response and doesn't mention future work, it's complete
			if !cd.mentionsFutureWork(lowerResponse) {
				if debugMode {
					fmt.Printf("✅ DEBUG: Found Q&A completion pattern: '%s'\n", pattern)
				}
				return true
			} else {
				if debugMode {
					fmt.Printf("❌ DEBUG: Q&A pattern found but mentions future work\n")
				}
			}
		}
	}
	
	// Check for "No" responses indicating more work is needed
	continuationSignals := []string{
		"no, i still need", "no, i need to", "no, there's more",
		"not yet", "not finished", "not complete", "not done",
		"no, i should", "no, we need", "no, the task",
		"still need to", "still working on", "more work",
		"not everything", "incomplete", "unfinished",
	}
	
	for _, signal := range continuationSignals {
		if strings.Contains(lowerResponse, signal) {
			if debugMode {
				fmt.Printf("❌ DEBUG: Found continuation signal: '%s'\n", signal)
			}
			return false // Explicitly not complete
		}
	}
	
	// Check if this is a pure informational response (no action words)
	if cd.isPureInformationalResponse(lowerResponse) {
		if debugMode {
			fmt.Printf("✅ DEBUG: Pure informational response detected\n")
		}
		return true
	}
	
	if debugMode {
		fmt.Printf("❌ DEBUG: No completion signals found\n")
	}
	return false
}

// mentionsFutureWork checks if response mentions future actions (helper for completion detector)
func (cd *CompletionDetector) mentionsFutureWork(response string) bool {
	debugMode := false // Temporary debug
	
	// First-person future work indicators (AI saying it will do something)
	firstPersonFuture := []string{
		"i'll", "i will", "let me", "i should", "i need to", "i want to",
		"i'm going to", "i plan to", "let's", "i could", "i would",
	}
	
	for _, phrase := range firstPersonFuture {
		if strings.Contains(response, phrase) {
			if debugMode {
				fmt.Printf("🔍 DEBUG: Found first-person future: '%s'\n", phrase)
			}
			return true
		}
	}
	
	// Action-oriented future indicators (suggesting work to be done)
	actionFuture := []string{
		"next step", " then ", "after that", "continuing", "proceeding",
		"moving forward", "we should", "we could", "we need", "we might",
		"should implement", "should add", "should create", "should build",
	}
	
	for _, phrase := range actionFuture {
		if strings.Contains(response, phrase) {
			if debugMode {
				fmt.Printf("🔍 DEBUG: Found action future: '%s'\n", phrase)
			}
			return true
		}
	}
	
	// Check for "then" at start/end of sentence (word boundaries)
	if strings.HasPrefix(response, "then ") || strings.HasSuffix(response, " then") || 
	   strings.Contains(response, ". then ") || strings.Contains(response, ", then ") {
		if debugMode {
			fmt.Printf("🔍 DEBUG: Found 'then' as complete word\n")
		}
		return true
	}
	
	// General future words, but only if they're in action context
	generalFuture := []string{"next", "then", "after"}
	for _, word := range generalFuture {
		if strings.Contains(response, word) {
			// Check if it's in an action context
			if strings.Contains(response, word+" ") || strings.Contains(response, word+",") {
				if debugMode {
					fmt.Printf("🔍 DEBUG: Found general future in action context: '%s'\n", word)
				}
				return true
			}
		}
	}
	
	if debugMode {
		fmt.Printf("🔍 DEBUG: No future work patterns found\n")
	}
	return false
}

// isPureInformationalResponse checks if response is purely informational with no action intent
func (cd *CompletionDetector) isPureInformationalResponse(response string) bool {
	// Count words
	wordCount := len(strings.Fields(response))
	
	// If it's a short response with no action words, it's likely informational
	if wordCount < 30 {
		actionWords := []string{
			"create", "add", "implement", "build", "make", "write", "edit",
			"modify", "update", "change", "fix", "improve", "enhance",
			"install", "setup", "configure", "deploy", "run", "execute",
			"start", "begin", "initiate", "proceed", "continue",
		}
		
		hasActionWords := false
		for _, action := range actionWords {
			if strings.Contains(response, action) {
				hasActionWords = true
				break
			}
		}
		
		// If short response with no action words and no future work mentions, it's complete
		if !hasActionWords && !cd.mentionsFutureWork(response) {
			return true
		}
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

// GenerateContinuePrompt creates a prompt to check if work is complete
func (cd *CompletionDetector) GenerateCompletionCheckPrompt() string {
	prompts := []string{
		"Is this task complete?",
		"Are you finished with this work?", 
		"Is there anything else you need to do?",
		"Have you completed everything that was requested?",
		"Is this implementation finished?",
		"Are you done, or is there more work to do?",
		"Is the task fully complete?",
		"Do you need to do anything else?",
	}
	
	// Use time-based seeding for variety
	rand.Seed(time.Now().UnixNano())
	return prompts[rand.Intn(len(prompts))]
}

// GenerateContinuePrompt creates a prompt to continue the work (legacy - kept for compatibility)
func (cd *CompletionDetector) GenerateContinuePrompt() string {
	return cd.GenerateCompletionCheckPrompt()
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
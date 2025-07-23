package task

import (
	"fmt"
	"regexp"
	"strings"
)

// CompletionDetector analyzes LLM responses to determine if work is complete
type CompletionDetector struct {
	completionPatterns []*regexp.Regexp
	incompletePatterns []*regexp.Regexp
	// Objective tracking
	originalObjective     string
	objectiveSet          bool
	objectiveChangeCount  int
	allowObjectiveChanges bool
	// Context tracking to detect responses to completion checks
	lastCompletionCheckSent bool
	lastPromptWasCheck      bool
}

// ObjectiveValidationResult represents the result of objective validation
type ObjectiveValidationResult struct {
	IsValid           bool
	OriginalObjective string
	NewObjective      string
	ChangeDetected    bool
	ValidationError   string
	SuggestedFix      string
}

// completionDebugLog sends completion detection debug messages using the unified debug system
func completionDebugLog(message string) {
	// Use the same debug system but prefix with completion detection
	if debugHandler != nil {
		debugHandler("🎯 COMPLETION: " + message)
	}
}

// NewCompletionDetector creates a new completion detector with comprehensive patterns
func NewCompletionDetector() *CompletionDetector {
	// Patterns that indicate completion
	completionPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bOBJECTIVE_COMPLETE\b`),
		regexp.MustCompile(`(?i)\bTASK_COMPLETE\b`),
		regexp.MustCompile(`(?i)\bEXPLORATION_COMPLETE\b`),
		regexp.MustCompile(`(?i)\bANALYSIS_COMPLETE\b`),
		regexp.MustCompile(`(?i)\bthe\s+(?:task|objective|work|implementation|feature|analysis|exploration)\s+is\s+(?:now\s+)?complete`),
		regexp.MustCompile(`(?i)\b(?:all\s+)?(?:tasks|objectives|work|steps)\s+(?:are\s+)?(?:now\s+)?complete`),
		regexp.MustCompile(`(?i)\b(?:successfully|completely)\s+(?:implemented|finished|completed|done)`),
		regexp.MustCompile(`(?i)\bevery(?:thing)?\s+(?:has\s+been\s+)?(?:implemented|completed|finished|done)`),
		regexp.MustCompile(`(?i)\bno\s+(?:further|additional|more)\s+(?:action|work|tasks|steps)\s+(?:is\s+)?(?:required|needed)`),
		regexp.MustCompile(`(?i)\b(?:ready\s+)?(?:for\s+)?(?:review|testing|deployment|use)`),
		regexp.MustCompile(`(?i)\ball\s+(?:requirements|specifications|features)\s+(?:have\s+been\s+)?(?:met|implemented|satisfied)`),
		// Context-specific completion patterns (longer YES responses that are unambiguous)
		regexp.MustCompile(`^(?i)\s*YES,?\s+(?:the\s+)?(?:objective|task|work)\s+is\s+complete`),
		regexp.MustCompile(`^(?i)\s*YES,?\s+(?:all\s+)?(?:work|tasks)\s+(?:is|are)\s+(?:finished|done|complete)`),
		regexp.MustCompile(`(?i)^YES.*(?:complete|finished|done).*$`),
	}

	// Patterns that indicate work is still in progress
	incompletePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bnext\s+(?:step|task|action)`),
		regexp.MustCompile(`(?i)\bstill\s+need\s+to\b`),
		regexp.MustCompile(`(?i)\bwill\s+(?:also\s+)?(?:need|require)\b`),
		regexp.MustCompile(`(?i)\bshould\s+(?:also\s+)?(?:add|implement|create|modify)`),
		regexp.MustCompile(`(?i)\bin\s+progress\b`),
		regexp.MustCompile(`(?i)\bworking\s+on\b`),
		regexp.MustCompile(`(?i)\bcontinuing\s+(?:with|to)\b`),
		regexp.MustCompile(`(?i)\b(?:let\s+me|i'll|i\s+will)\s+(?:also\s+)?(?:add|implement|create|modify|continue)`),
		regexp.MustCompile(`(?i)\b(?:additionally|furthermore|moreover)\b`),
		regexp.MustCompile(`(?i)\b(?:remaining|pending)\s+(?:work|tasks|steps)`),
		// Context-specific incomplete patterns (longer NO responses that are unambiguous)
		regexp.MustCompile(`^(?i)\s*NO,?\s+(?:more|additional)\s+work\s+(?:is\s+)?(?:needed|required)`),
		regexp.MustCompile(`(?i)^NO.*(?:still|more|additional|remaining).*$`),
	}

	return &CompletionDetector{
		completionPatterns: completionPatterns,
		incompletePatterns: incompletePatterns,
	}
}

// IsComplete checks if the LLM response indicates the work is finished
func (cd *CompletionDetector) IsComplete(response string) bool {
	if response == "" {
		completionDebugLog("Empty response, not complete")
		return false
	}

	cleanResponse := strings.TrimSpace(response)

	// Extract objective if present - if this is a response that's setting an objective
	// then we know it's not complete yet
	objective := cd.ExtractObjective(cleanResponse)
	if objective != "" && !cd.objectiveSet {
		// This is an initial objective-setting message, so it's definitely not complete
		completionDebugLog("Found initial objective setting, not complete")
		return false
	}

	// Check if this is a response to a completion check
	if cd.lastPromptWasCheck {
		completionDebugLog("Evaluating response to completion check")
		// Reset the flag after use
		cd.lastPromptWasCheck = false

		// For completion check responses, prioritize YES/NO detection
		if cd.isCompletionCheckResponse(cleanResponse) {
			completionDebugLog("Found completion check response indicating completion")
			return true
		}

		if cd.isIncompleteCheckResponse(cleanResponse) {
			completionDebugLog("Found completion check response indicating more work needed")
			return false
		}
	}

	// First check for explicit incomplete signals
	for _, pattern := range cd.incompletePatterns {
		if pattern.MatchString(cleanResponse) {
			completionDebugLog("Found incompletion pattern, work not complete")
			return false
		}
	}

	// Then check for completion signals
	for _, pattern := range cd.completionPatterns {
		if pattern.MatchString(cleanResponse) {
			completionDebugLog("Found completion pattern, work appears complete")
			return true
		}
	}

	// Additional heuristics for completion detection
	result := cd.hasCompletionHeuristics(cleanResponse)
	if result {
		completionDebugLog("Heuristics indicate completion")
	} else {
		completionDebugLog("No completion signals detected")
	}
	return result
}

// hasCompletionHeuristics applies additional logic to detect completion
func (cd *CompletionDetector) hasCompletionHeuristics(response string) bool {
	lowerResponse := strings.ToLower(response)

	// Check for summary-style responses that indicate completion
	summaryIndicators := []string{
		"in summary",
		"to summarize",
		"in conclusion",
		"overall",
		"final result",
		"here's what",
		"the project",
		"this codebase",
	}

	summaryCount := 0
	for _, indicator := range summaryIndicators {
		if strings.Contains(lowerResponse, indicator) {
			summaryCount++
		}
	}

	// If response contains multiple summary indicators and no future tense, likely complete
	if summaryCount >= 2 && !cd.hasFutureTense(lowerResponse) {
		return true
	}

	// Check for responses that end with conclusions rather than next steps
	lines := strings.Split(response, "\n")
	lastLine := ""
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			lastLine = strings.ToLower(trimmed)
			break
		}
	}

	// If the last substantial line indicates completion
	conclusionEndings := []string{
		"implementation is complete",
		"task is finished",
		"work is done",
		"ready for use",
		"successfully implemented",
		"analysis complete",
	}

	for _, ending := range conclusionEndings {
		if strings.Contains(lastLine, ending) {
			return true
		}
	}

	return false
}

// hasFutureTense checks if the response contains future tense indicating more work
func (cd *CompletionDetector) hasFutureTense(response string) bool {
	futureTensePatterns := []string{
		"will need",
		"should implement",
		"next step",
		"would be to",
		"plan to",
		"going to",
		"will add",
		"will create",
		"will modify",
		"need to",
		"should add",
		"could implement",
		"might want to",
	}

	for _, pattern := range futureTensePatterns {
		if strings.Contains(response, pattern) {
			return true
		}
	}

	return false
}

// isCompletionCheckResponse checks if response indicates completion in response to a direct check
func (cd *CompletionDetector) isCompletionCheckResponse(response string) bool {
	// Trim and normalize response
	trimmed := strings.TrimSpace(response)
	if len(trimmed) == 0 {
		return false
	}

	// Check for simple YES responses
	if strings.ToUpper(trimmed) == "YES" {
		return true
	}

	// Check for YES with additional completion text
	patterns := []string{
		`^(?i)\s*YES\s*[,.]?\s*(?:the\s+)?(?:objective|task|work)\s+is\s+complete`,
		`^(?i)\s*YES\s*[,.]?\s*(?:all\s+)?(?:work|tasks)\s+(?:is|are)\s+(?:finished|done|complete)`,
		`^(?i)\s*YES\s*[,.]?\s*(?:everything|all)\s+(?:is\s+)?(?:finished|done|complete)`,
		`^(?i)\s*YES\s*[,.]?\s*(?:i've|i\s+have)\s+(?:finished|completed|done)`,
		`^(?i)\s*YES\s*[,.]?\s*(?:no\s+)?(?:further|additional|more)\s+work\s+(?:is\s+)?(?:needed|required)`,
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, trimmed); matched {
			return true
		}
	}

	return false
}

// isIncompleteCheckResponse checks if response indicates work is not complete in response to a direct check
func (cd *CompletionDetector) isIncompleteCheckResponse(response string) bool {
	// Trim and normalize response
	trimmed := strings.TrimSpace(response)
	if len(trimmed) == 0 {
		return false
	}

	// Check for simple NO responses
	if strings.ToUpper(trimmed) == "NO" {
		return true
	}

	// Check for NO with additional explanation text
	patterns := []string{
		`^(?i)\s*NO\s*[,.]?\s*(?:more|additional|further)\s+work\s+(?:is\s+)?(?:needed|required)`,
		`^(?i)\s*NO\s*[,.]?\s*(?:i\s+)?(?:still|also)\s+need\s+to`,
		`^(?i)\s*NO\s*[,.]?\s*(?:there|i)\s+(?:is|are|have)\s+(?:still|more)`,
		`^(?i)\s*NO\s*[,.]?\s*(?:not\s+)?(?:yet|quite|completely)\s+(?:finished|done|complete)`,
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, trimmed); matched {
			return true
		}
	}

	return false
}

// GenerateCompletionCheckPrompt creates a comprehensive prompt to check if work is complete
func (cd *CompletionDetector) GenerateCompletionCheckPrompt(userObjective string) string {
	// Set flag to indicate we just sent a completion check
	cd.lastPromptWasCheck = true
	cd.lastCompletionCheckSent = true

	if userObjective != "" {
		return `COMPLETION_CHECK: Has your stated objective been fully achieved?

Original objective: ` + userObjective + `

Please answer clearly:
- YES if the objective is complete and no further work is needed
- NO if more work is required, and explain what specific steps remain

Be specific about whether you've accomplished what was requested.`
	}

	return `COMPLETION_CHECK: Is your current work complete?

Please answer clearly:
- YES if you've finished all the work you intended to do
- NO if there are still tasks or steps remaining

If NO, please specify what additional work needs to be done.`
}

// ExtractObjective attempts to extract the objective from a response
func (cd *CompletionDetector) ExtractObjective(response string) string {
	// Look for OBJECTIVE: pattern
	objectivePattern := regexp.MustCompile(`(?i)^(?:\s*)?OBJECTIVE:\s*(.+)`)

	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := objectivePattern.FindStringSubmatch(line); len(matches) > 1 {
			objective := strings.TrimSpace(matches[1])
			completionDebugLog("Extracted objective: " + objective)
			return objective
		}
	}

	completionDebugLog("No objective pattern found in response")
	return ""
}

// SetOriginalObjective sets the original objective that must be maintained
func (cd *CompletionDetector) SetOriginalObjective(objective string) {
	if !cd.objectiveSet && objective != "" {
		cd.originalObjective = objective
		cd.objectiveSet = true
		cd.objectiveChangeCount = 0
		completionDebugLog("🎯 Original objective set: " + objective)
	}
}

// ValidateObjectiveConsistency checks if the LLM is trying to change the objective
func (cd *CompletionDetector) ValidateObjectiveConsistency(response string) *ObjectiveValidationResult {
	result := &ObjectiveValidationResult{
		IsValid:           true,
		OriginalObjective: cd.originalObjective,
	}

	// Extract objective from current response
	newObjective := cd.ExtractObjective(response)
	if newObjective == "" {
		// No objective found in response - this is fine for continuation responses
		return result
	}

	result.NewObjective = newObjective

	// If this is the first objective, set it as original
	if !cd.objectiveSet {
		cd.SetOriginalObjective(newObjective)
		return result
	}

	return result
}

// isObjectiveEquivalent checks if two objectives are semantically equivalent
func (cd *CompletionDetector) isObjectiveEquivalent(original, new string) bool {
	// Normalize objectives for comparison
	orig := cd.normalizeObjective(original)
	newObj := cd.normalizeObjective(new)

	// Exact match
	if orig == newObj {
		return true
	}

	// Check if new objective is just a minor rewording of the original
	// Calculate similarity based on shared keywords
	similarity := cd.calculateObjectiveSimilarity(orig, newObj)

	// Consider objectives equivalent if they're 80% similar
	// This allows for minor rewording but catches scope changes
	return similarity >= 0.8
}

// normalizeObjective normalizes an objective for comparison
func (cd *CompletionDetector) normalizeObjective(objective string) string {
	// Convert to lowercase and remove common variations
	normalized := strings.ToLower(strings.TrimSpace(objective))

	// Remove common prefixes that don't change meaning
	prefixesToRemove := []string{
		"to ", "i will ", "i'll ", "let me ", "first, ", "now ", "currently ", "next, ",
	}

	for _, prefix := range prefixesToRemove {
		if strings.HasPrefix(normalized, prefix) {
			normalized = strings.TrimSpace(normalized[len(prefix):])
		}
	}

	// Remove common suffixes
	suffixesToRemove := []string{
		" completely", " thoroughly", " carefully", " properly", " correctly",
	}

	for _, suffix := range suffixesToRemove {
		if strings.HasSuffix(normalized, suffix) {
			normalized = strings.TrimSpace(normalized[:len(normalized)-len(suffix)])
		}
	}

	return normalized
}

// calculateObjectiveSimilarity calculates similarity between two objectives (0.0 to 1.0)
func (cd *CompletionDetector) calculateObjectiveSimilarity(obj1, obj2 string) float64 {
	words1 := strings.Fields(obj1)
	words2 := strings.Fields(obj2)

	if len(words1) == 0 && len(words2) == 0 {
		return 1.0
	}

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Count shared words
	wordSet1 := make(map[string]bool)
	for _, word := range words1 {
		wordSet1[word] = true
	}

	sharedWords := 0
	for _, word := range words2 {
		if wordSet1[word] {
			sharedWords++
		}
	}

	// Calculate Jaccard similarity
	totalUniqueWords := len(wordSet1)
	for _, word := range words2 {
		if !wordSet1[word] {
			totalUniqueWords++
		}
	}

	if totalUniqueWords == 0 {
		return 1.0
	}

	return float64(sharedWords) / float64(totalUniqueWords)
}

// GetObjectiveStatus returns the current objective tracking status
func (cd *CompletionDetector) GetObjectiveStatus() (string, bool, int) {
	return cd.originalObjective, cd.objectiveSet, cd.objectiveChangeCount
}

// ResetObjective resets the objective tracking (for new conversations)
func (cd *CompletionDetector) ResetObjective() {
	cd.originalObjective = ""
	cd.objectiveSet = false
	cd.objectiveChangeCount = 0
	cd.lastCompletionCheckSent = false
	cd.lastPromptWasCheck = false
	completionDebugLog("🎯 Objective tracking reset")
}

// ResetContext resets only the context tracking flags (for new user inputs)
func (cd *CompletionDetector) ResetContext() {
	cd.lastCompletionCheckSent = false
	cd.lastPromptWasCheck = false
	completionDebugLog("🔄 Context tracking reset")
}

// FormatObjectiveWarning creates a warning message for objective changes
func (cd *CompletionDetector) FormatObjectiveWarning(result *ObjectiveValidationResult) string {
	if result.IsValid {
		return ""
	}

	warning := fmt.Sprintf(`🚨 OBJECTIVE CHANGE DETECTED

❌ You changed your objective mid-stream:
   Original: "%s"
   New:      "%s"

🎯 STAY FOCUSED: Complete your original objective first!

✅ Correct approach:
   1. Keep working on: "%s"
   2. Signal completion with: OBJECTIVE_COMPLETE: [your analysis]
   3. ONLY THEN set new objectives if needed

💡 You can explore within your objective scope, but don't change the objective itself.`,
		result.OriginalObjective,
		result.NewObjective,
		result.OriginalObjective)

	return warning
}

// HasInfiniteLoopPattern detects if recent responses show repetitive behavior
func (cd *CompletionDetector) HasInfiniteLoopPattern(responses []string) bool {
	if len(responses) < 3 {
		return false
	}

	// Check for exact repetition
	last := strings.TrimSpace(responses[len(responses)-1])
	secondLast := strings.TrimSpace(responses[len(responses)-2])

	if len(last) > 10 && len(secondLast) > 10 && last == secondLast {
		return true
	}

	// Check for semantic repetition (similar tasks being repeated)
	if cd.hasSemanticRepetition(responses) {
		return true
	}

	// Check for oscillating between completion states
	if cd.hasCompletionOscillation(responses) {
		return true
	}

	return false
}

// hasSemanticRepetition checks for repeated similar actions
func (cd *CompletionDetector) hasSemanticRepetition(responses []string) bool {
	if len(responses) < 4 {
		return false
	}

	// Look for repeated task patterns
	taskPattern := regexp.MustCompile(`(?i)(?:READ|SEARCH|LIST|RUN)\s+(\S+)`)

	recentTasks := make(map[string]int)
	for i := len(responses) - 4; i < len(responses); i++ {
		matches := taskPattern.FindAllStringSubmatch(responses[i], -1)
		for _, match := range matches {
			if len(match) > 1 {
				task := strings.ToLower(match[0])
				recentTasks[task]++
			}
		}
	}

	// If any task appears more than twice in recent responses, it's likely repetitive
	for _, count := range recentTasks {
		if count > 2 {
			return true
		}
	}

	return false
}

// hasCompletionOscillation checks for alternating between complete/incomplete states
func (cd *CompletionDetector) hasCompletionOscillation(responses []string) bool {
	if len(responses) < 4 {
		return false
	}

	// Check completion status of recent responses
	completionStates := make([]bool, 0, 4)
	for i := len(responses) - 4; i < len(responses); i++ {
		completionStates = append(completionStates, cd.IsComplete(responses[i]))
	}

	// Look for alternating pattern: complete -> incomplete -> complete -> incomplete
	alternations := 0
	for i := 1; i < len(completionStates); i++ {
		if completionStates[i] != completionStates[i-1] {
			alternations++
		}
	}

	// If alternations >= 2 in 4 responses, it's oscillating
	return alternations >= 2
}

// ShouldUseContinuationPrompt determines if a continuation prompt should be used
// instead of a completion check. This helps prevent immediate completion checks
// right after an objective is set.
func (cd *CompletionDetector) ShouldUseContinuationPrompt(response string) bool {
	// Check if this response sets an objective for the first time
	objective := cd.ExtractObjective(response)
	isSettingObjective := objective != "" && !cd.objectiveSet

	// If this is setting an initial objective, use continuation prompt
	if isSettingObjective {
		return true
	}

	// If the response contains task commands or mentions next steps, use continuation prompt
	for _, pattern := range cd.incompletePatterns {
		if pattern.MatchString(response) {
			return true
		}
	}

	// Check for patterns indicating LLM is planning work but hasn't started
	planningPatterns := []string{
		"i'll start", "let me", "first", "begin by", "to accomplish this", "i will",
		"let's start", "we need to", "we'll", "step 1", "first step", "initially",
	}

	lowerResponse := strings.ToLower(response)
	for _, pattern := range planningPatterns {
		if strings.Contains(lowerResponse, pattern) {
			return true
		}
	}

	return false
}

// GetContinuationPrompt returns a simple prompt to continue work
// rather than asking if the task is complete
func (cd *CompletionDetector) GetContinuationPrompt() string {
	return "Please continue working on this task."
}

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
	// YES/NO completion check tracking
	yesNoCheckSent       bool
	lastResponseWasYesNo bool
	yesNoCheckCount      int
	maxYesNoChecks       int
	// Mixed message tracking
	mixedMessageWarningSent  bool
	mixedMessageWarningCount int
	maxMixedMessageWarnings  int
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
		completionPatterns:      completionPatterns,
		incompletePatterns:      incompletePatterns,
		maxYesNoChecks:          3, // Limit YES/NO checks to prevent loops
		maxMixedMessageWarnings: 2, // Limit mixed message warnings to prevent loops
	}
}

// IsComplete checks if the LLM response indicates the work is finished
func (cd *CompletionDetector) IsComplete(response string) bool {
	// First check for obvious completion indicators
	if cd.isCompletionCheckResponse(response) {
		completionDebugLog("Completion detected from explicit check response")
		return true
	}

	// Now check if this is a text-only response with no commands
	if cd.isTextOnlyResponse(response) {
		completionDebugLog("Completion detected: text-only response with no commands")
		return true
	}

	// Check for explicit completion phrases
	lowerResponse := strings.ToLower(response)
	completionSignals := []string{
		"all tasks completed",
		"task is now complete",
		"all requested work has been finished",
		"implementation is complete",
		"changes have been applied successfully",
		"the code has been updated",
		"the feature has been implemented",
		"your request has been completed",
	}

	for _, signal := range completionSignals {
		if strings.Contains(lowerResponse, signal) {
			completionDebugLog(fmt.Sprintf("Completion detected from signal: '%s'", signal))
			return true
		}
	}

	// If incomplete indicators are present, definitely not complete
	if cd.isIncompleteCheckResponse(response) {
		completionDebugLog("Incompletion detected from check response")
		return false
	}

	// Check heuristics for likely completion
	if cd.hasCompletionHeuristics(response) {
		completionDebugLog("Completion detected from heuristics")
		return true
	}

	// Check if there are any future tense phrases indicating more work
	if cd.hasFutureTense(response) {
		completionDebugLog("Incompletion detected due to future tense")
		return false
	}

	// Default to incomplete if none of the above conditions are met
	return false
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
	return "Continue. If you have completed all necessary tasks, please provide a final text-only summary of what you've done and what you've found. Otherwise, please continue with the next task."
}

// GenerateYesNoCompletionCheckPrompt creates a direct YES/NO question for completion checking
func (cd *CompletionDetector) GenerateYesNoCompletionCheckPrompt() string {
	cd.yesNoCheckSent = true
	cd.yesNoCheckCount++
	return "YES or NO, has the objective at hand been completed?"
}

// IsYesNoCompletionResponse checks if this response is answering our YES/NO completion question
func (cd *CompletionDetector) IsYesNoCompletionResponse(response string) bool {
	if !cd.yesNoCheckSent {
		return false
	}

	// Check if response starts with YES or NO
	trimmed := strings.TrimSpace(response)
	upper := strings.ToUpper(trimmed)

	return strings.HasPrefix(upper, "YES") || strings.HasPrefix(upper, "NO")
}

// ParseYesNoResponse extracts YES or NO from the response and returns appropriate action
func (cd *CompletionDetector) ParseYesNoResponse(response string) (isComplete bool, shouldContinue bool) {
	if !cd.IsYesNoCompletionResponse(response) {
		return false, false
	}

	trimmed := strings.TrimSpace(response)
	upper := strings.ToUpper(trimmed)

	cd.lastResponseWasYesNo = true
	cd.yesNoCheckSent = false // Reset flag after getting response

	if strings.HasPrefix(upper, "YES") {
		completionDebugLog("YES response received - objective complete")
		return true, false // Complete, don't continue
	} else if strings.HasPrefix(upper, "NO") {
		completionDebugLog("NO response received - objective not complete")
		return false, true // Not complete, should continue
	}

	return false, false
}

// ShouldSendYesNoCheck determines if we should send a YES/NO completion check
func (cd *CompletionDetector) ShouldSendYesNoCheck() bool {
	// Don't send if we've already sent one recently
	if cd.yesNoCheckSent {
		return false
	}

	// Don't send if we've reached max checks
	if cd.yesNoCheckCount >= cd.maxYesNoChecks {
		completionDebugLog(fmt.Sprintf("Maximum YES/NO checks reached (%d), not sending more", cd.maxYesNoChecks))
		return false
	}

	// Don't send if last response was already a YES/NO
	if cd.lastResponseWasYesNo {
		cd.lastResponseWasYesNo = false // Reset for next time
		return false
	}

	return true
}

// GenerateContinuationPrompt creates a prompt to encourage the LLM to continue working
func (cd *CompletionDetector) GenerateContinuationPrompt() string {
	return "You may continue with the OBJECTIVE at hand."
}

// ResetYesNoState resets the YES/NO completion check state (for new conversations)
func (cd *CompletionDetector) ResetYesNoState() {
	cd.yesNoCheckSent = false
	cd.lastResponseWasYesNo = false
	cd.yesNoCheckCount = 0
}

// DetectMixedMessage checks if a text message contains task-like content that wasn't parsed
func (cd *CompletionDetector) DetectMixedMessage(response string) bool {
	lowerResponse := strings.ToLower(response)

	// Action words that suggest the LLM is trying to perform tasks
	actionWords := []string{
		"reading file", "📖", "🔧", "creating", "editing", "modifying",
		"create", "edit", "file", "license", "i'll", "i will", "let me",
		"executing", "running", "applying", "writing to", "updating",
		"implementing", "adding", "removing", "deleting", "changing",
		"let's", "we should", "first i'll", "next i'll", "now i'll",
		"i need to", "i'm going to", "i will now", "let me now",
	}

	// Task-related phrases that suggest commands
	taskPhrases := []string{
		"read the", "edit the", "create a", "update the", "modify the",
		"list the", "search for", "run the", "execute the", "check the",
		"look at", "examine the", "analyze the", "review the",
	}

	foundActions := []string{}

	// Check for action words
	for _, word := range actionWords {
		if strings.Contains(lowerResponse, word) {
			foundActions = append(foundActions, word)
		}
	}

	// Check for task phrases
	for _, phrase := range taskPhrases {
		if strings.Contains(lowerResponse, phrase) {
			foundActions = append(foundActions, phrase)
		}
	}

	// If we found action indicators, this might be a mixed message
	if len(foundActions) > 0 {
		completionDebugLog(fmt.Sprintf("Mixed message detected - found action indicators: %v", foundActions))
		return true
	}

	return false
}

// GenerateMixedMessageWarning creates a warning for mixed text/task messages
func (cd *CompletionDetector) GenerateMixedMessageWarning() string {
	return `🚨 MIXED MESSAGE DETECTED

You are not allowed to mix text and task messages. Please follow these rules:

1. EITHER send a text-only message (no tasks)
2. OR send task commands using proper format:
   - 🔧 READ file.txt
   - 🔧 EDIT file.txt
   - 🔧 RUN command
   - etc.

Your message appears to contain both text and task-like content mixed together.

Please respond with either:
- A pure text message, OR  
- Properly formatted task commands

Do you want to continue with your objective? (YES/NO)`
}

// ShouldSendMixedMessageWarning determines if we should send a mixed message warning
func (cd *CompletionDetector) ShouldSendMixedMessageWarning() bool {
	// Don't send if we've already sent one recently
	if cd.mixedMessageWarningSent {
		return false
	}

	// Don't send if we've reached max warnings
	if cd.mixedMessageWarningCount >= cd.maxMixedMessageWarnings {
		completionDebugLog(fmt.Sprintf("Maximum mixed message warnings reached (%d), not sending more", cd.maxMixedMessageWarnings))
		return false
	}

	return true
}

// SendMixedMessageWarning marks that a mixed message warning has been sent
func (cd *CompletionDetector) SendMixedMessageWarning() {
	cd.mixedMessageWarningSent = true
	cd.mixedMessageWarningCount++
}

// IsMixedMessageResponse checks if this response is answering our mixed message warning
func (cd *CompletionDetector) IsMixedMessageResponse(response string) bool {
	if !cd.mixedMessageWarningSent {
		return false
	}

	// Check if response starts with YES or NO (same as completion check)
	trimmed := strings.TrimSpace(response)
	upper := strings.ToUpper(trimmed)

	return strings.HasPrefix(upper, "YES") || strings.HasPrefix(upper, "NO")
}

// ParseMixedMessageResponse extracts YES or NO from mixed message response
func (cd *CompletionDetector) ParseMixedMessageResponse(response string) (shouldContinue bool, isValidResponse bool) {
	if !cd.IsMixedMessageResponse(response) {
		return false, false
	}

	trimmed := strings.TrimSpace(response)
	upper := strings.ToUpper(trimmed)

	cd.mixedMessageWarningSent = false // Reset flag after getting response

	if strings.HasPrefix(upper, "YES") {
		completionDebugLog("YES response to mixed message warning - should continue")
		return true, true
	} else if strings.HasPrefix(upper, "NO") {
		completionDebugLog("NO response to mixed message warning - should not continue")
		return false, true
	}

	return false, false
}

// ResetMixedMessageState resets mixed message warning state
func (cd *CompletionDetector) ResetMixedMessageState() {
	cd.mixedMessageWarningSent = false
	cd.mixedMessageWarningCount = 0
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

// normalizeObjective normalizes an objective for comparison

// calculateObjectiveSimilarity calculates similarity between two objectives (0.0 to 1.0)

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
	cd.ResetYesNoState()
	cd.ResetMixedMessageState()
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
	// If the response already signals that the objective (or task) is complete,
	// we should NOT ask it to continue.
	if cd.IsComplete(response) {
		return false
	}

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
	return "Continue with the next step or task. If you've completed all tasks, please provide a final text-only summary."
}

// isTextOnlyResponse checks if the response contains no commands/tasks
// and appears to be a final explanatory message
func (cd *CompletionDetector) isTextOnlyResponse(response string) bool {
	// Check for common task patterns with emojis
	taskPatterns := []string{
		"🔧 READ", "📖 READ",
		"🔧 LIST", "📂 LIST",
		"🔧 SEARCH", "🔍 SEARCH",
		"🔧 RUN",
		"🔧 MEMORY", "💾 MEMORY",
		"🔧 TODO", "📝 TODO",
		">>LOOM_EDIT", "✏️ Edit",
	}

	for _, pattern := range taskPatterns {
		if strings.Contains(response, pattern) {
			return false
		}
	}

	// Check for natural language task patterns
	naturalLangPatterns := []string{
		"READ ",
		"LIST ",
		"SEARCH ",
		"RUN ",
		"MEMORY ",
	}

	for _, pattern := range naturalLangPatterns {
		if regexp.MustCompile(`(?m)^` + pattern).MatchString(response) {
			return false
		}
	}

	// Look for LOOM_EDIT blocks
	if regexp.MustCompile(`(?s)>>LOOM_EDIT.*?<<LOOM_EDIT`).MatchString(response) {
		return false
	}

	// Also check if it looks like a proper explanation (has some substance)
	// to avoid false positives on empty or transition responses
	if len(response) > 80 || strings.Contains(response, "\n") {
		return true
	}

	return false
}

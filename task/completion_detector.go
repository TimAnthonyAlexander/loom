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
}

// NewCompletionDetector creates a new completion detector with comprehensive patterns
func NewCompletionDetector() *CompletionDetector {
	// Patterns that indicate completion
	completionPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bOBJECTIVE_COMPLETE\b`),
		regexp.MustCompile(`(?i)\bTASK_COMPLETE\b`),
		regexp.MustCompile(`(?i)\bEXPLORATION_COMPLETE\b`),
		regexp.MustCompile(`(?i)\bANALYSIS_COMPLETE\b`),
		regexp.MustCompile(`(?i)\bthe\s+(?:task|objective|work|implementation|analysis)\s+is\s+(?:now\s+)?complete`),
		regexp.MustCompile(`(?i)\bobjective\s+achieved\s+successfully`),
		regexp.MustCompile(`(?i)\ball\s+requested\s+work\s+has\s+been\s+(?:finished|completed)`),
		regexp.MustCompile(`(?i)\bimplementation\s+is\s+complete\s+and\s+tested`),
		regexp.MustCompile(`(?i)\bwork\s+is\s+(?:now\s+)?(?:finished|complete|done)`),
		regexp.MustCompile(`(?i)\btask\s+(?:has\s+been\s+)?(?:finished|completed|accomplished)`),
		regexp.MustCompile(`(?i)\bsuccessfully\s+(?:implemented|completed|finished)`),
		regexp.MustCompile(`(?i)\bready\s+for\s+(?:user\s+)?(?:review|testing|deployment)`),
		regexp.MustCompile(`(?i)\bno\s+(?:further|additional|more)\s+(?:work|changes|modifications)\s+(?:needed|required)`),
		regexp.MustCompile(`(?i)\b(?:yes,?\s+)?(?:the\s+)?(?:task|objective|work)\s+is\s+complete`),
		regexp.MustCompile(`(?i)\b(?:yes,?\s+)?(?:everything|all)\s+is\s+(?:done|finished|complete)`),
	}

	// Patterns that indicate work is not complete
	incompletePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\badditional\s+work\s+needed`),
		regexp.MustCompile(`(?i)\bobjective\s+partially\s+complete`),
		regexp.MustCompile(`(?i)\bstill\s+need\s+to`),
		regexp.MustCompile(`(?i)\bnext\s+(?:step|phase)\s+(?:is|would\s+be)`),
		regexp.MustCompile(`(?i)\bmore\s+work\s+(?:is\s+)?(?:needed|required)`),
		regexp.MustCompile(`(?i)\bnot\s+yet\s+(?:complete|finished|done)`),
		regexp.MustCompile(`(?i)\bno,?\s+(?:not\s+)?(?:yet|complete)`),
		regexp.MustCompile(`(?i)\bneed\s+to\s+(?:continue|proceed|implement|add|fix)`),
		regexp.MustCompile(`(?i)\b(?:there's|there\s+is)\s+(?:still\s+)?more\s+work`),
		regexp.MustCompile(`(?i)\bremaining\s+(?:tasks|work|steps)`),
		regexp.MustCompile(`(?i)\bwould\s+like\s+to\s+(?:continue|proceed|add)`),
		regexp.MustCompile(`(?i)\bshould\s+(?:also|next|now)\s+(?:implement|add|create|fix)`),
	}

	return &CompletionDetector{
		completionPatterns: completionPatterns,
		incompletePatterns: incompletePatterns,
	}
}

// IsComplete checks if the LLM response indicates the work is finished
func (cd *CompletionDetector) IsComplete(response string) bool {
	if response == "" {
		return false
	}

	cleanResponse := strings.TrimSpace(response)

	// First check for explicit incomplete signals
	for _, pattern := range cd.incompletePatterns {
		if pattern.MatchString(cleanResponse) {
			return false
		}
	}

	// Then check for completion signals
	for _, pattern := range cd.completionPatterns {
		if pattern.MatchString(cleanResponse) {
			return true
		}
	}

	// Additional heuristics for completion detection
	return cd.hasCompletionHeuristics(cleanResponse)
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

// GenerateCompletionCheckPrompt creates a comprehensive prompt to check if work is complete
func (cd *CompletionDetector) GenerateCompletionCheckPrompt(userObjective string) string {
	if userObjective != "" {
		return fmt.Sprintf(`COMPLETION_CHECK: Has your stated objective been fully achieved?

Original objective: %s

Please answer clearly:
- YES if the objective is complete and no further work is needed
- NO if more work is required, and explain what specific steps remain

Be specific about whether you've accomplished what was requested.`, userObjective)
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
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
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
	taskPattern := regexp.MustCompile(`(?i)(?:READ|SEARCH|LIST|EDIT|RUN)\s+(\S+)`)

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

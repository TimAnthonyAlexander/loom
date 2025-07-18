package task

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ChangeSummary represents a summary of a code change
type ChangeSummary struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"` // "edit", "create", "delete", "refactor"
	FilePath       string    `json:"file_path"`
	Summary        string    `json:"summary"`         // Brief one-line summary
	Rationale      string    `json:"rationale"`       // Detailed explanation
	Impact         string    `json:"impact"`          // What systems/files are affected
	TestSuggestion string    `json:"test_suggestion"` // Suggested tests
	Timestamp      time.Time `json:"timestamp"`
	Task           *Task     `json:"task,omitempty"` // Associated task
}

// RationaleExtractor extracts rationales and explanations from LLM responses
type RationaleExtractor struct {
	summaryPatterns   []*regexp.Regexp
	rationalePatterns []*regexp.Regexp
	impactPatterns    []*regexp.Regexp
	testPatterns      []*regexp.Regexp
}

// NewRationaleExtractor creates a new rationale extractor
func NewRationaleExtractor() *RationaleExtractor {
	return &RationaleExtractor{
		summaryPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^(?:summary|change|modification):\s*(.+)$`),
			regexp.MustCompile(`(?i)(?:briefly|in summary),?\s*(.+)`),
			regexp.MustCompile(`(?i)this\s+(?:change|modification|edit)\s+(.+)`),
		},
		rationalePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^(?:rationale|reasoning|why|explanation):\s*(.+)$`),
			regexp.MustCompile(`(?i)(?:the reason|because|rationale is)\s+(.+)`),
			regexp.MustCompile(`(?i)this\s+(?:approach|solution|pattern)\s+(.+)`),
			regexp.MustCompile(`(?i)(?:I chose|I decided|I'm using)\s+(.+)`),
		},
		impactPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^(?:impact|affects|changes):\s*(.+)$`),
			regexp.MustCompile(`(?i)(?:this affects|this changes|impacted files?)\s+(.+)`),
			regexp.MustCompile(`(?i)(?:other files?|dependencies?|systems?)\s+(?:affected|changed)\s+(.+)`),
		},
		testPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^(?:test|testing)(?:\s+suggestion)?:\s*(.+)$`),
			regexp.MustCompile(`(?i)(?:should test|need to test|testing)\s+(.+)`),
			regexp.MustCompile(`(?i)(?:test cases?|unit tests?)\s+(.+)`),
		},
	}
}

// ExtractChangeSummary extracts a change summary from LLM response
func (re *RationaleExtractor) ExtractChangeSummary(llmResponse string, task *Task) *ChangeSummary {
	summary := &ChangeSummary{
		ID:        generateChangeSummaryID(),
		Timestamp: time.Now(),
		Task:      task,
	}

	// Determine change type from task
	if task != nil {
		summary.Type = re.getChangeType(task)
		summary.FilePath = task.Path
	}

	// Extract components from LLM response
	summary.Summary = re.extractSummary(llmResponse)
	summary.Rationale = re.extractRationale(llmResponse)
	summary.Impact = re.extractImpact(llmResponse)
	summary.TestSuggestion = re.extractTestSuggestion(llmResponse)

	// If no explicit summary found, generate one from task
	if summary.Summary == "" && task != nil {
		summary.Summary = re.generateDefaultSummary(task)
	}

	return summary
}

// extractSummary extracts the summary from LLM response
func (re *RationaleExtractor) extractSummary(text string) string {
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check against summary patterns
		for _, pattern := range re.summaryPatterns {
			if matches := pattern.FindStringSubmatch(line); matches != nil {
				return strings.TrimSpace(matches[1])
			}
		}

		// Look for lines that seem like summaries (not too long, descriptive)
		if len(line) > 20 && len(line) < 150 &&
			strings.Contains(line, "change") || strings.Contains(line, "add") ||
			strings.Contains(line, "update") || strings.Contains(line, "fix") ||
			strings.Contains(line, "implement") || strings.Contains(line, "refactor") {
			return line
		}
	}

	return ""
}

// extractRationale extracts the rationale/reasoning from LLM response
func (re *RationaleExtractor) extractRationale(text string) string {
	lines := strings.Split(text, "\n")
	var rationale []string
	inRationale := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if inRationale {
				rationale = append(rationale, "")
			}
			continue
		}

		// Check for rationale patterns
		for _, pattern := range re.rationalePatterns {
			if matches := pattern.FindStringSubmatch(line); matches != nil {
				rationale = append(rationale, strings.TrimSpace(matches[1]))
				inRationale = true
				continue
			}
		}

		// Look for explanatory content
		if inRationale || re.isExplanatoryLine(line) {
			rationale = append(rationale, line)
			inRationale = true
		}

		// Stop if we hit a new section
		if strings.Contains(line, "```") || strings.HasPrefix(line, "#") {
			inRationale = false
		}
	}

	result := strings.Join(rationale, "\n")
	return strings.TrimSpace(result)
}

// extractImpact extracts impact assessment from LLM response
func (re *RationaleExtractor) extractImpact(text string) string {
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check against impact patterns
		for _, pattern := range re.impactPatterns {
			if matches := pattern.FindStringSubmatch(line); matches != nil {
				return strings.TrimSpace(matches[1])
			}
		}
	}

	return ""
}

// extractTestSuggestion extracts test suggestions from LLM response
func (re *RationaleExtractor) extractTestSuggestion(text string) string {
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check against test patterns
		for _, pattern := range re.testPatterns {
			if matches := pattern.FindStringSubmatch(line); matches != nil {
				return strings.TrimSpace(matches[1])
			}
		}
	}

	return ""
}

// isExplanatoryLine checks if a line contains explanatory content
func (re *RationaleExtractor) isExplanatoryLine(line string) bool {
	explanatoryWords := []string{
		"because", "since", "this allows", "this enables", "this ensures",
		"the benefit", "advantage", "reason", "purpose", "goal",
		"this approach", "this pattern", "this design", "this implementation",
	}

	lowerLine := strings.ToLower(line)
	for _, word := range explanatoryWords {
		if strings.Contains(lowerLine, word) {
			return true
		}
	}

	return false
}

// getChangeType determines the type of change from a task
func (re *RationaleExtractor) getChangeType(task *Task) string {
	switch task.Type {
	case TaskTypeEditFile:
		if task.Content != "" {
			return "create"
		}
		return "edit"
	case TaskTypeRunShell:
		return "build"
	default:
		return "other"
	}
}

// generateDefaultSummary generates a default summary from task information
func (re *RationaleExtractor) generateDefaultSummary(task *Task) string {
	switch task.Type {
	case TaskTypeEditFile:
		if task.Content != "" {
			return fmt.Sprintf("Create new file %s", task.Path)
		}
		return fmt.Sprintf("Edit file %s", task.Path)
	case TaskTypeRunShell:
		return fmt.Sprintf("Execute command: %s", task.Command)
	case TaskTypeReadFile:
		return fmt.Sprintf("Read file %s", task.Path)
	case TaskTypeListDir:
		return fmt.Sprintf("List directory %s", task.Path)
	default:
		return "Perform task"
	}
}

// generateChangeSummaryID generates a unique ID for a change summary
func generateChangeSummaryID() string {
	return fmt.Sprintf("change_%d", time.Now().UnixNano())
}

// ChangeSummaryManager manages change summaries and rationales
type ChangeSummaryManager struct {
	extractor    *RationaleExtractor
	summaries    []*ChangeSummary
	maxSummaries int
}

// NewChangeSummaryManager creates a new change summary manager
func NewChangeSummaryManager() *ChangeSummaryManager {
	return &ChangeSummaryManager{
		extractor:    NewRationaleExtractor(),
		summaries:    make([]*ChangeSummary, 0),
		maxSummaries: 50, // Keep last 50 summaries
	}
}

// AddSummary adds a new change summary
func (csm *ChangeSummaryManager) AddSummary(llmResponse string, task *Task) *ChangeSummary {
	summary := csm.extractor.ExtractChangeSummary(llmResponse, task)
	csm.summaries = append(csm.summaries, summary)

	// Trim if we exceed max summaries
	if len(csm.summaries) > csm.maxSummaries {
		csm.summaries = csm.summaries[len(csm.summaries)-csm.maxSummaries:]
	}

	return summary
}

// GetSummaries returns all change summaries
func (csm *ChangeSummaryManager) GetSummaries() []*ChangeSummary {
	return csm.summaries
}

// GetRecentSummaries returns the N most recent summaries
func (csm *ChangeSummaryManager) GetRecentSummaries(n int) []*ChangeSummary {
	if n <= 0 || n >= len(csm.summaries) {
		return csm.summaries
	}

	return csm.summaries[len(csm.summaries)-n:]
}

// GetSummariesByFile returns summaries for a specific file
func (csm *ChangeSummaryManager) GetSummariesByFile(filePath string) []*ChangeSummary {
	var result []*ChangeSummary
	for _, summary := range csm.summaries {
		if summary.FilePath == filePath {
			result = append(result, summary)
		}
	}
	return result
}

// FormatSummariesForDisplay formats summaries for TUI display
func (csm *ChangeSummaryManager) FormatSummariesForDisplay() string {
	if len(csm.summaries) == 0 {
		return "No change summaries available."
	}

	var result strings.Builder
	result.WriteString("📋 Recent Change Summaries\n")
	result.WriteString(strings.Repeat("=", 50) + "\n\n")

	// Show recent summaries in reverse order (newest first)
	recent := csm.GetRecentSummaries(10)
	for i := len(recent) - 1; i >= 0; i-- {
		summary := recent[i]
		result.WriteString(fmt.Sprintf("🔸 **%s** (%s)\n", summary.Summary, summary.Type))
		if summary.FilePath != "" {
			result.WriteString(fmt.Sprintf("   📁 %s\n", summary.FilePath))
		}
		if summary.Rationale != "" {
			rationale := summary.Rationale
			if len(rationale) > 100 {
				rationale = rationale[:100] + "..."
			}
			result.WriteString(fmt.Sprintf("   💭 %s\n", rationale))
		}
		result.WriteString(fmt.Sprintf("   ⏰ %s\n\n", summary.Timestamp.Format("15:04:05")))
	}

	return result.String()
}

// ExportSummaries exports summaries as JSON
func (csm *ChangeSummaryManager) ExportSummaries() (string, error) {
	data, err := json.MarshalIndent(csm.summaries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to export summaries: %w", err)
	}
	return string(data), nil
}

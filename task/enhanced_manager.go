package task

import (
	"fmt"
	"loom/chat"
	"loom/indexer"
	"loom/llm"
	"strings"
	"time"
)

// EnhancedManager extends the basic task manager with M6 features
type EnhancedManager struct {
	*Manager
	testDiscovery      *TestDiscovery
	changeSummaryMgr   *ChangeSummaryManager
	rationaleExtractor *RationaleExtractor
	enableTestFirst    bool
	autoRunTests       bool
}

// NewEnhancedManager creates a new enhanced manager with M6 features
func NewEnhancedManager(executor *Executor, llmAdapter llm.LLMAdapter, chatSession *chat.Session, index *indexer.Index) *EnhancedManager {
	baseManager := NewManager(executor, llmAdapter, chatSession)

	return &EnhancedManager{
		Manager:            baseManager,
		testDiscovery:      NewTestDiscovery(index),
		changeSummaryMgr:   NewChangeSummaryManager(),
		rationaleExtractor: NewRationaleExtractor(),
		enableTestFirst:    false,
		autoRunTests:       true,
	}
}

// HandleLLMResponseEnhanced processes LLM responses with enhanced features
func (em *EnhancedManager) HandleLLMResponseEnhanced(llmResponse string, eventChan chan<- TaskExecutionEvent) (*TaskExecution, error) {
	// Extract change summary and rationale from LLM response
	changeSummary := em.rationaleExtractor.ExtractChangeSummary(llmResponse, nil)
	if changeSummary.Summary != "" {
		em.changeSummaryMgr.AddSummary(llmResponse, nil)
	}

	// Handle task execution with the base manager (includes objective validation)
	execution, err := em.Manager.HandleLLMResponse(llmResponse, eventChan)
	if err != nil {
		return nil, err
	}

	// If there was an execution (tasks were found)
	if execution != nil {
		// Extract change summaries for each task
		for i, task := range execution.Tasks {
			if i < len(execution.Responses) {
				summary := em.rationaleExtractor.ExtractChangeSummary(llmResponse, &task)
				if summary.Summary != "" {
					em.changeSummaryMgr.AddSummary(llmResponse, &task)
				}
			}
		}

		// Check if we should run tests after edits
		if em.autoRunTests && em.hasFileEdits(execution) {
			return em.handlePostEditTesting(execution, eventChan)
		}
	}

	return execution, nil
}

// hasFileEdits checks if the execution contains file edit tasks
func (em *EnhancedManager) hasFileEdits(execution *TaskExecution) bool {
	for _, task := range execution.Tasks {
		if task.Type == TaskTypeEditFile {
			return true
		}
	}
	return false
}

// handlePostEditTesting handles test discovery and execution after file edits
func (em *EnhancedManager) handlePostEditTesting(execution *TaskExecution, eventChan chan<- TaskExecutionEvent) (*TaskExecution, error) {
	// Discover tests first
	if err := em.testDiscovery.DiscoverTests(); err != nil {
		eventChan <- TaskExecutionEvent{
			Type:    "test_discovery_failed",
			Message: fmt.Sprintf("Test discovery failed: %v", err),
		}
		return execution, nil // Don't fail the main execution
	}

	testCount := em.testDiscovery.GetTestCount()
	if testCount == 0 {
		eventChan <- TaskExecutionEvent{
			Type:    "test_discovery_completed",
			Message: "No tests found in workspace",
		}
		return execution, nil
	}

	// Notify about test discovery
	eventChan <- TaskExecutionEvent{
		Type:    "test_discovery_completed",
		Message: fmt.Sprintf("Found %d tests across %d files", testCount, len(em.testDiscovery.GetDiscoveredTests())),
	}

	// For now, auto-run Go tests if they exist (can be extended)
	goTests := em.testDiscovery.GetTestsByLanguage("Go")
	if len(goTests) > 0 {
		return em.runTestsForLanguage("Go", execution, eventChan)
	}

	return execution, nil
}

// runTestsForLanguage runs tests for a specific language
func (em *EnhancedManager) runTestsForLanguage(language string, execution *TaskExecution, eventChan chan<- TaskExecutionEvent) (*TaskExecution, error) {
	eventChan <- TaskExecutionEvent{
		Type:    "test_execution_started",
		Message: fmt.Sprintf("Running %s tests...", language),
	}

	// Run tests with timeout
	timeout := 30 * time.Second
	testResult, err := em.testDiscovery.RunTests(language, em.executor.workspacePath, timeout)
	if err != nil {
		eventChan <- TaskExecutionEvent{
			Type:    "test_execution_failed",
			Message: fmt.Sprintf("Test execution failed: %v", err),
		}
		return execution, nil
	}

	// Format test results
	testSummary := em.formatTestResults(testResult)

	eventChan <- TaskExecutionEvent{
		Type:    "test_execution_completed",
		Message: testSummary,
	}

	// Add test results to chat for AI feedback
	testMessage := llm.Message{
		Role:      "system",
		Content:   fmt.Sprintf("TEST_RESULTS: %s\n\n%s", language, testSummary),
		Timestamp: time.Now(),
	}

	if err := em.chatSession.AddMessage(testMessage); err != nil {
		fmt.Printf("Warning: failed to add test results to chat: %v\n", err)
	}

	// If tests failed, trigger AI analysis
	if !testResult.Success && testResult.TestsFailed > 0 {
		return em.handleTestFailures(testResult, execution, eventChan)
	}

	return execution, nil
}

// formatTestResults creates a human-readable summary of test results
func (em *EnhancedManager) formatTestResults(result *TestResult) string {
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("🧪 Test Results for %s\n", result.Command))
	summary.WriteString(fmt.Sprintf("Duration: %v\n", result.Duration.Round(time.Millisecond)))

	if result.Success {
		summary.WriteString("✅ All tests passed!\n")
	} else {
		summary.WriteString("❌ Some tests failed\n")
	}

	summary.WriteString(fmt.Sprintf("Passed: %d", result.TestsPassed))
	if result.TestsFailed > 0 {
		summary.WriteString(fmt.Sprintf(", Failed: %d", result.TestsFailed))
	}
	if result.TestsSkipped > 0 {
		summary.WriteString(fmt.Sprintf(", Skipped: %d", result.TestsSkipped))
	}
	summary.WriteString("\n")

	// Show failed test names if any
	if len(result.FailedTests) > 0 {
		summary.WriteString("\nFailed tests:\n")
		for _, testName := range result.FailedTests {
			summary.WriteString(fmt.Sprintf("  • %s\n", testName))
		}
	}

	// Include limited output for debugging (first 500 chars)
	if result.Output != "" && !result.Success {
		output := result.Output
		if len(output) > 500 {
			output = output[:500] + "...[truncated]"
		}
		summary.WriteString(fmt.Sprintf("\nTest Output:\n%s", output))
	}

	return summary.String()
}

// handleTestFailures triggers AI analysis of test failures
func (em *EnhancedManager) handleTestFailures(testResult *TestResult, execution *TaskExecution, eventChan chan<- TaskExecutionEvent) (*TaskExecution, error) {
	eventChan <- TaskExecutionEvent{
		Type:    "test_analysis_started",
		Message: "Analyzing test failures with AI...",
	}

	// Create a prompt for the AI to analyze test failures
	analysisPrompt := fmt.Sprintf(`The tests have failed after the recent code changes. Please analyze the test results and suggest fixes.

Test Results:
%s

Recent Changes Made:
%s

Please provide:
1. Analysis of why the tests might be failing
2. Suggested fixes for the failing tests
3. Whether the test failures are related to the recent changes

`, em.formatTestResults(testResult), em.getRecentChangesSummary())

	// Add analysis request to chat
	analysisMessage := llm.Message{
		Role:      "user",
		Content:   analysisPrompt,
		Timestamp: time.Now(),
	}

	if err := em.chatSession.AddMessage(analysisMessage); err != nil {
		return execution, fmt.Errorf("failed to add analysis request to chat: %w", err)
	}

	eventChan <- TaskExecutionEvent{
		Type:    "test_analysis_requested",
		Message: "AI analysis of test failures requested - check chat for detailed analysis",
	}

	return execution, nil
}

// getRecentChangesSummary returns a summary of recent changes
func (em *EnhancedManager) getRecentChangesSummary() string {
	recent := em.changeSummaryMgr.GetRecentSummaries(5)
	if len(recent) == 0 {
		return "No recent changes recorded"
	}

	var summary strings.Builder
	for _, change := range recent {
		summary.WriteString(fmt.Sprintf("- %s (%s)\n", change.Summary, change.FilePath))
		if change.Rationale != "" && len(change.Rationale) < 100 {
			summary.WriteString(fmt.Sprintf("  Reason: %s\n", change.Rationale))
		}
	}

	return summary.String()
}

// GetTestDiscovery returns the test discovery instance
func (em *EnhancedManager) GetTestDiscovery() *TestDiscovery {
	return em.testDiscovery
}

// GetChangeSummaryManager returns the change summary manager
func (em *EnhancedManager) GetChangeSummaryManager() *ChangeSummaryManager {
	return em.changeSummaryMgr
}

// GetTestSummary returns a summary of discovered tests
func (em *EnhancedManager) GetTestSummary() string {
	if err := em.testDiscovery.DiscoverTests(); err != nil {
		return fmt.Sprintf("Test discovery failed: %v", err)
	}

	return em.testDiscovery.FormatSummary()
}

// RunTestsManually manually runs tests for a specific language
func (em *EnhancedManager) RunTestsManually(language string) (*TestResult, error) {
	timeout := 30 * time.Second
	return em.testDiscovery.RunTests(language, em.executor.workspacePath, timeout)
}

// SetTestConfiguration sets test-related configuration
func (em *EnhancedManager) SetTestConfiguration(enableTestFirst, autoRunTests bool) {
	em.enableTestFirst = enableTestFirst
	em.autoRunTests = autoRunTests
}

// GetChangeSummaries returns recent change summaries for display
func (em *EnhancedManager) GetChangeSummaries() string {
	return em.changeSummaryMgr.FormatSummariesForDisplay()
}
